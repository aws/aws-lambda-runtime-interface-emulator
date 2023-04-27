// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rendering

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/metering"
	"go.amzn.com/lambda/rapi/model"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	// ErrorTypeInternalServerError error type for internal server error
	ErrorTypeInternalServerError = "InternalServerError"
	// ErrorTypeInvalidStateTransition error type for invalid state transition
	ErrorTypeInvalidStateTransition = "InvalidStateTransition"
	// ErrorTypeInvalidRequestID error type for invalid request ID error
	ErrorTypeInvalidRequestID = "InvalidRequestID"
	// ErrorTypeRequestEntityTooLarge error type for payload too large
	ErrorTypeRequestEntityTooLarge = "RequestEntityTooLarge"
	// ErrorTypeTruncatedHTTPRequest error type for truncated HTTP request
	ErrorTypeTruncatedHTTPRequest = "TruncatedHTTPRequest"
)

// ErrRenderingServiceStateNotSet returned when state not set
var ErrRenderingServiceStateNotSet = errors.New("EventRenderingService state not set")

// RendererState is renderer object state.
type RendererState interface {
	RenderAgentEvent(w http.ResponseWriter, r *http.Request) error
	RenderRuntimeEvent(w http.ResponseWriter, r *http.Request) error
}

// EventRenderingService is a state machine for rendering runtime and agent API responses.
type EventRenderingService struct {
	mutex        *sync.RWMutex
	currentState RendererState
}

// SetRenderer set current state
func (s *EventRenderingService) SetRenderer(state RendererState) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.currentState = state
}

// RenderAgentEvent delegates to state implementation.
func (s *EventRenderingService) RenderAgentEvent(w http.ResponseWriter, r *http.Request) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.currentState == nil {
		return ErrRenderingServiceStateNotSet
	}
	return s.currentState.RenderAgentEvent(w, r)
}

// RenderRuntimeEvent delegates to state implementation.
func (s *EventRenderingService) RenderRuntimeEvent(w http.ResponseWriter, r *http.Request) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.currentState == nil {
		return ErrRenderingServiceStateNotSet
	}
	return s.currentState.RenderRuntimeEvent(w, r)
}

// NewRenderingService returns new EventRenderingService.
func NewRenderingService() *EventRenderingService {
	return &EventRenderingService{
		mutex: &sync.RWMutex{},
	}
}

// InvokeRendererMetrics contains metrics of invoke request
type InvokeRendererMetrics struct {
	ReadTime  time.Duration
	SizeBytes int
}

// InvokeRenderer knows how to render invoke event.
type InvokeRenderer struct {
	ctx                 context.Context
	invoke              *interop.Invoke
	tracingHeaderParser func(context.Context, *interop.Invoke) string
	requestBuffer       []byte
	requestMutex        sync.Mutex
	metrics             InvokeRendererMetrics
}

type RestoreRenderer struct {
}

// NewAgentInvokeEvent forms a new AgentInvokeEvent from INVOKE request
func NewAgentInvokeEvent(req *interop.Invoke) (*model.AgentInvokeEvent, error) {
	deadlineMono, err := strconv.ParseInt(req.DeadlineNs, 10, 64)
	if err != nil {
		return nil, err
	}
	deadline := metering.MonoToEpoch(deadlineMono) / int64(time.Millisecond)
	return &model.AgentInvokeEvent{
		AgentEvent: &model.AgentEvent{
			EventType:  "INVOKE",
			DeadlineMs: deadline,
		},
		RequestID:          req.ID,
		InvokedFunctionArn: req.InvokedFunctionArn,
		Tracing:            model.NewXRayTracing(req.TraceID),
	}, nil
}

// RenderAgentEvent renders invoke event json for agent.
func (s *InvokeRenderer) RenderAgentEvent(writer http.ResponseWriter, request *http.Request) error {
	event, err := NewAgentInvokeEvent(s.invoke)
	if err != nil {
		return err
	}

	bytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	renderAgentInvokeHeaders(writer, uuid.New()) // TODO: check this thing

	if _, err := writer.Write(bytes); err != nil {
		return err
	}
	return nil
}

func (s *InvokeRenderer) bufferInvokeRequest() error {
	s.requestMutex.Lock()
	defer s.requestMutex.Unlock()
	var err error = nil
	if nil == s.requestBuffer {
		reader := io.LimitReader(s.invoke.Payload, interop.MaxPayloadSize)
		start := time.Now()
		s.requestBuffer, err = io.ReadAll(reader)
		s.metrics = InvokeRendererMetrics{
			ReadTime:  time.Since(start),
			SizeBytes: len(s.requestBuffer),
		}
	}
	return err
}

// RenderRuntimeEvent renders invoke payload for runtime.
func (s *InvokeRenderer) RenderRuntimeEvent(writer http.ResponseWriter, request *http.Request) error {
	invoke := s.invoke
	customerTraceID := s.tracingHeaderParser(s.ctx, s.invoke)

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
	if t, err := strconv.ParseInt(invoke.DeadlineNs, 10, 64); err == nil {
		deadlineHeader = strconv.FormatInt(metering.MonoToEpoch(t)/int64(time.Millisecond), 10)
	} else {
		log.WithError(err).Warn("Failed to compute deadline header")
	}

	renderInvokeHeaders(writer, invoke.ID, customerTraceID, invoke.ClientContext,
		cognitoIdentityJSON, invoke.InvokedFunctionArn, deadlineHeader, invoke.ContentType)

	if invoke.Payload != nil {
		if err := s.bufferInvokeRequest(); err != nil {
			return err
		}
		_, err := writer.Write(s.requestBuffer)
		return err
	}

	return nil
}

func (s *RestoreRenderer) RenderRuntimeEvent(writer http.ResponseWriter, request *http.Request) error {
	writer.WriteHeader(http.StatusOK)
	return nil
}

func (s *RestoreRenderer) RenderAgentEvent(writer http.ResponseWriter, request *http.Request) error {
	return nil
}

// NewInvokeRenderer returns new invoke event renderer
func NewInvokeRenderer(ctx context.Context, invoke *interop.Invoke, traceParser func(context.Context, *interop.Invoke) string) *InvokeRenderer {
	return &InvokeRenderer{
		invoke:              invoke,
		ctx:                 ctx,
		tracingHeaderParser: traceParser,
		requestBuffer:       nil,
		requestMutex:        sync.Mutex{},
	}
}

func NewRestoreRenderer() *RestoreRenderer {
	return &RestoreRenderer{}
}

func (s *InvokeRenderer) GetMetrics() InvokeRendererMetrics {
	s.requestMutex.Lock()
	defer s.requestMutex.Unlock()
	return s.metrics
}

// ShutdownRenderer renderer for shutdown event.
type ShutdownRenderer struct {
	AgentEvent model.AgentShutdownEvent
}

// RenderAgentEvent renders shutdown event for agent.
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

// RenderRuntimeEvent renders shutdown event for runtime.
func (s *ShutdownRenderer) RenderRuntimeEvent(w http.ResponseWriter, r *http.Request) error {
	panic("We should SIGTERM runtime")
}

func setHeaderIfNotEmpty(headers http.Header, key string, value string) {
	if len(value) != 0 {
		headers.Set(key, value)
	}
}

func setHeaderOrDefault(headers http.Header, key, val, defaultVal string) {
	if val == "" {
		headers.Set(key, defaultVal)
		return
	}
	headers.Set(key, val)
}

func renderInvokeHeaders(writer http.ResponseWriter, invokeID string, customerTraceID string, clientContext string,
	cognitoIdentity string, invokedFunctionArn string, deadlineMs string, contentType string) {
	headers := writer.Header()
	setHeaderIfNotEmpty(headers, "Lambda-Runtime-Aws-Request-Id", invokeID)
	setHeaderIfNotEmpty(headers, "Lambda-Runtime-Trace-Id", customerTraceID)
	setHeaderIfNotEmpty(headers, "Lambda-Runtime-Client-Context", clientContext)
	setHeaderIfNotEmpty(headers, "Lambda-Runtime-Cognito-Identity", cognitoIdentity)
	setHeaderIfNotEmpty(headers, "Lambda-Runtime-Invoked-Function-Arn", invokedFunctionArn)
	setHeaderIfNotEmpty(headers, "Lambda-Runtime-Deadline-Ms", deadlineMs)
	setHeaderOrDefault(headers, "Content-Type", contentType, "application/json")
	writer.WriteHeader(http.StatusOK)
}

// RenderRuntimeLogsResponse renders response from Telemetry API
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

func renderAgentInvokeHeaders(writer http.ResponseWriter, eventID uuid.UUID) {
	headers := writer.Header()
	headers.Set("Lambda-Extension-Event-Identifier", eventID.String())
	headers.Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
}

// RenderForbiddenWithTypeMsg method for rendering error response
func RenderForbiddenWithTypeMsg(w http.ResponseWriter, r *http.Request, errorType string, format string, args ...interface{}) {
	if err := RenderJSON(http.StatusForbidden, w, r, &model.ErrorResponse{
		ErrorType:    errorType,
		ErrorMessage: fmt.Sprintf(format, args...),
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderInternalServerError method for rendering error response
func RenderInternalServerError(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusInternalServerError, w, r, &model.ErrorResponse{
		ErrorMessage: "Internal Server Error",
		ErrorType:    ErrorTypeInternalServerError,
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderRequestEntityTooLarge method for rendering error response
func RenderRequestEntityTooLarge(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusRequestEntityTooLarge, w, r, &model.ErrorResponse{
		ErrorMessage: fmt.Sprintf("Exceeded maximum allowed payload size (%d bytes).", interop.MaxPayloadSize),
		ErrorType:    ErrorTypeRequestEntityTooLarge,
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderTruncatedHTTPRequestError method for rendering error response
func RenderTruncatedHTTPRequestError(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusBadRequest, w, r, &model.ErrorResponse{
		ErrorMessage: "HTTP request detected as truncated",
		ErrorType:    ErrorTypeTruncatedHTTPRequest,
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderInvalidRequestID renders invalid request ID error response
func RenderInvalidRequestID(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusBadRequest, w, r, &model.ErrorResponse{
		ErrorMessage: "Invalid request ID",
		ErrorType:    "InvalidRequestID",
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderInvalidFunctionResponseMode renders invalid function response mode response
func RenderInvalidFunctionResponseMode(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusBadRequest, w, r, &model.ErrorResponse{
		ErrorMessage: "Invalid function response mode",
		ErrorType:    "InvalidFunctionResponseMode",
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderAccepted method for rendering accepted status response
func RenderAccepted(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusAccepted, w, r, &model.StatusResponse{
		Status: "OK",
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderInteropError is a convenience method for interpreting interop errors
func RenderInteropError(writer http.ResponseWriter, request *http.Request, err error) {
	if err == interop.ErrInvalidInvokeID || err == interop.ErrResponseSent {
		RenderInvalidRequestID(writer, request)
	} else {
		log.Panic(err)
	}
}
