// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rendering

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
)

const (
	ErrorTypeInternalServerError = "InternalServerError"

	ErrorTypeInvalidStateTransition = "InvalidStateTransition"

	ErrorTypeInvalidRequestID = "InvalidRequestID"

	ErrorTypeRequestEntityTooLarge = "RequestEntityTooLarge"

	ErrorTypeTruncatedHTTPRequest = "TruncatedHTTPRequest"
)

var ErrRenderingServiceStateNotSet = errors.New("EventRenderingService state not set")

type RendererState interface {
	RenderAgentEvent(w http.ResponseWriter, r *http.Request) error
	RenderRuntimeEvent(w http.ResponseWriter, r *http.Request) error
}

type EventRenderingService struct {
	mutex        *sync.RWMutex
	currentState RendererState
}

func NewRenderingService() *EventRenderingService {
	return &EventRenderingService{
		mutex: &sync.RWMutex{},
	}
}

func (s *EventRenderingService) SetRenderer(state RendererState) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.currentState = state
}

func (s *EventRenderingService) RenderAgentEvent(w http.ResponseWriter, r *http.Request) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.currentState == nil {
		return ErrRenderingServiceStateNotSet
	}
	return s.currentState.RenderAgentEvent(w, r)
}

func (s *EventRenderingService) RenderRuntimeEvent(w http.ResponseWriter, r *http.Request) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.currentState == nil {
		return ErrRenderingServiceStateNotSet
	}
	return s.currentState.RenderRuntimeEvent(w, r)
}

type InvokeRendererMetrics struct {
	ReadTime  time.Duration
	SizeBytes int
}

type InvokeRenderer struct {
	ctx                 context.Context
	invoke              *interop.Invoke
	tracingHeaderParser func(context.Context) string
	requestBuffer       *bytes.Buffer
	requestMutex        sync.Mutex
	metrics             InvokeRendererMetrics
}

func NewInvokeRenderer(ctx context.Context, invoke *interop.Invoke, requestBuffer *bytes.Buffer, traceParser func(context.Context) string) *InvokeRenderer {
	requestBuffer.Reset()
	return &InvokeRenderer{
		invoke:              invoke,
		ctx:                 ctx,
		tracingHeaderParser: traceParser,
		requestBuffer:       requestBuffer,
		requestMutex:        sync.Mutex{},
	}
}

func newAgentInvokeEvent(ctx context.Context, req *interop.Invoke) (*model.AgentInvokeEvent, error) {
	return &model.AgentInvokeEvent{
		AgentEvent: &model.AgentEvent{
			EventType:  "INVOKE",
			DeadlineMs: req.GetDeadlineMs(ctx),
		},
		RequestID:          req.ID,
		InvokedFunctionArn: req.InvokedFunctionArn,
		Tracing:            model.NewXRayTracing(req.TraceID),
	}, nil
}

func (s *InvokeRenderer) RenderAgentEvent(writer http.ResponseWriter, request *http.Request) error {
	event, err := newAgentInvokeEvent(s.ctx, s.invoke)
	if err != nil {
		return err
	}

	bytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	eventID := uuid.New()
	headers := writer.Header()
	headers.Set("Lambda-Extension-Event-Identifier", eventID.String())
	headers.Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)

	if _, err := writer.Write(bytes); err != nil {
		return err
	}
	return nil
}

func (s *InvokeRenderer) bufferInvokeRequest() error {
	s.requestMutex.Lock()
	defer s.requestMutex.Unlock()
	var err error = nil
	if s.requestBuffer.Len() == 0 {
		reader := io.LimitReader(s.invoke.Payload, interop.MaxPayloadSize)
		start := time.Now()
		_, err = s.requestBuffer.ReadFrom(reader)
		s.metrics = InvokeRendererMetrics{
			ReadTime:  time.Since(start),
			SizeBytes: s.requestBuffer.Len(),
		}
	}
	return err
}

func (s *InvokeRenderer) RenderRuntimeEvent(writer http.ResponseWriter, request *http.Request) error {
	invoke := s.invoke
	customerTraceID := s.tracingHeaderParser(s.ctx)

	cognitoIdentityJSON := ""
	if len(invoke.CognitoIdentityID) != 0 || len(invoke.CognitoIdentityPoolID) != 0 {
		cognitoJSON, err := json.Marshal(model.CognitoIdentity{
			CognitoIdentityID:     invoke.CognitoIdentityID,
			CognitoIdentityPoolID: invoke.CognitoIdentityPoolID,
		})
		if err != nil {
			return err
		}

		cognitoIdentityJSON = string(cognitoJSON)
	}

	var deadlineHeader string
	if deadlineMs := invoke.GetDeadlineMs(s.ctx); deadlineMs > 0 {
		deadlineHeader = strconv.FormatInt(deadlineMs, 10)
	}

	renderInvokeHeaders(writer, invoke.ID, customerTraceID, invoke.ClientContext,
		cognitoIdentityJSON, invoke.InvokedFunctionArn, deadlineHeader, invoke.ContentType)

	if invoke.Payload != nil {
		if err := s.bufferInvokeRequest(); err != nil {
			return err
		}
		_, err := writer.Write(s.requestBuffer.Bytes())
		return err
	}

	return nil
}

func (s *InvokeRenderer) GetMetrics() InvokeRendererMetrics {
	s.requestMutex.Lock()
	defer s.requestMutex.Unlock()
	return s.metrics
}

type ShutdownRenderer struct {
	AgentEvent model.AgentShutdownEvent
}

func (s *ShutdownRenderer) RenderAgentEvent(w http.ResponseWriter, r *http.Request) error {
	bytes, err := json.Marshal(s.AgentEvent)
	if err != nil {
		return err
	}
	if _, err := w.Write(bytes); err != nil {
		return err
	}
	return nil
}

func (s *ShutdownRenderer) RenderRuntimeEvent(w http.ResponseWriter, r *http.Request) error {
	panic("We should SIGTERM runtime")
}

func renderInvokeHeaders(writer http.ResponseWriter, invokeID string, customerTraceID string, clientContext string,
	cognitoIdentity string, invokedFunctionArn string, deadlineMs string, contentType string,
) {
	setHeaderIfNotEmpty := func(headers http.Header, key string, value string) {
		if value != "" {
			headers.Set(key, value)
		}
	}

	headers := writer.Header()
	setHeaderIfNotEmpty(headers, "Lambda-Runtime-Aws-Request-Id", invokeID)
	setHeaderIfNotEmpty(headers, "Lambda-Runtime-Trace-Id", customerTraceID)
	setHeaderIfNotEmpty(headers, "Lambda-Runtime-Client-Context", clientContext)
	setHeaderIfNotEmpty(headers, "Lambda-Runtime-Cognito-Identity", cognitoIdentity)
	setHeaderIfNotEmpty(headers, "Lambda-Runtime-Invoked-Function-Arn", invokedFunctionArn)
	setHeaderIfNotEmpty(headers, "Lambda-Runtime-Deadline-Ms", deadlineMs)
	if contentType == "" {
		contentType = "application/json"
	}
	headers.Set("Content-Type", contentType)
	writer.WriteHeader(http.StatusOK)
}

func RenderRuntimeLogsResponse(w http.ResponseWriter, respBody []byte, status int, headers map[string][]string) error {
	respHeaders := w.Header()
	for k, vals := range headers {
		for _, v := range vals {
			respHeaders.Add(k, v)
		}
	}

	w.WriteHeader(status)

	_, err := w.Write(respBody)
	return err
}

func RenderAccepted(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusAccepted, w, r, &model.StatusResponse{
		Status: "OK",
	}); err != nil {
		slog.Warn("Error while rendering response", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
