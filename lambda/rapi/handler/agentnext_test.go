// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/metering"
	"go.amzn.com/lambda/rapi/model"
	"go.amzn.com/lambda/rapi/rendering"
	"go.amzn.com/lambda/telemetry"
)

func TestRenderAgentInternalError(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	handler := NewAgentNextHandler(registrationService, rendering.NewRenderingService())
	request := httptest.NewRequest("GET", "/", nil)
	// request is missing agent's UUID context. This should not happen since the middleware validation should have failed
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
}

func TestRenderAgentInvokeUnknownAgent(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, uuid.New()))
	responseRecorder := httptest.NewRecorder()

	handler := NewAgentNextHandler(registrationService, rendering.NewRenderingService())
	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

	var errorResponse model.ErrorResponse
	respBody, _ := io.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &errorResponse)
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assert.Equal(t, errAgentIdentifierUnknown, errorResponse.ErrorType)
}

func TestRenderAgentInvokeInvalidAgentState(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	agent, err := registrationService.CreateExternalAgent("dummyName")
	assert.NoError(t, err)
	handler := NewAgentNextHandler(registrationService, rendering.NewRenderingService())
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

	var errorResponse model.ErrorResponse
	respBody, _ := io.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &errorResponse)
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assert.Equal(t, errAgentInvalidState, errorResponse.ErrorType)
}

func TestRenderAgentInvokeNextHappy(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	agent, err := registrationService.CreateExternalAgent("dummyName")
	assert.NoError(t, err)

	agent.SetState(agent.RegisteredState)
	agent.Release() // sets operator condition to true so that the thread doesn't suspend waiting for invoke request

	deadlineNs := metering.Monotime() + int64(100*time.Millisecond)
	requestID, functionArn := "ID", "InvokedFunctionArn"
	traceID := "Root=RootID;Parent=LambdaFrontend;Sampled=1"
	invoke := &interop.Invoke{
		TraceID:               traceID,
		ID:                    requestID,
		InvokedFunctionArn:    functionArn,
		CognitoIdentityID:     "CognitoIdentityId1",
		CognitoIdentityPoolID: "CognitoIdentityPoolId1",
		ClientContext:         "ClientContext",
		DeadlineNs:            fmt.Sprintf("%d", deadlineNs),
		ContentType:           "image/png",
		Payload:               strings.NewReader("Payload"),
	}

	renderingService := rendering.NewRenderingService()
	var buf bytes.Buffer
	renderingService.SetRenderer(rendering.NewInvokeRenderer(context.Background(), invoke, &buf, telemetry.NewNoOpTracer().BuildTracingHeader()))

	handler := NewAgentNextHandler(registrationService, renderingService)
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	var response model.AgentInvokeEvent
	respBody, _ := io.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &response)

	assert.Equal(t, agent.RunningState, agent.GetState())
	assert.Equal(t, "INVOKE", response.AgentEvent.EventType)
	assert.InDelta(t, time.Now().Add(100*time.Millisecond).UnixNano()/int64(time.Millisecond), response.AgentEvent.DeadlineMs, 5)
	assert.Equal(t, requestID, response.RequestID)
	assert.Equal(t, functionArn, response.InvokedFunctionArn)
	assert.Equal(t, model.XRayTracingType, response.Tracing.Type)
	assert.Equal(t, traceID, response.Tracing.Value)
}

func TestRenderAgentInternalInvokeNextHappy(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	agent, err := registrationService.CreateInternalAgent("dummyName")
	assert.NoError(t, err)

	agent.SetState(agent.RegisteredState)
	agent.Release() // sets operator condition to true so that the thread doesn't suspend waiting for invoke request

	deadlineNs := metering.Monotime() + int64(100*time.Millisecond)
	requestID, functionArn := "ID", "InvokedFunctionArn"
	traceID := "Root=RootID;Parent=LambdaFrontend;Sampled=1"
	invoke := &interop.Invoke{
		TraceID:               traceID,
		ID:                    requestID,
		InvokedFunctionArn:    functionArn,
		CognitoIdentityID:     "CognitoIdentityId1",
		CognitoIdentityPoolID: "CognitoIdentityPoolId1",
		ClientContext:         "ClientContext",
		DeadlineNs:            fmt.Sprintf("%d", deadlineNs),
		ContentType:           "image/png",
		Payload:               strings.NewReader("Payload"),
	}

	renderingService := rendering.NewRenderingService()
	var buf bytes.Buffer
	renderingService.SetRenderer(rendering.NewInvokeRenderer(context.Background(), invoke, &buf, telemetry.NewNoOpTracer().BuildTracingHeader()))

	handler := NewAgentNextHandler(registrationService, renderingService)
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	var response model.AgentInvokeEvent
	respBody, _ := io.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &response)

	assert.Equal(t, agent.RunningState, agent.GetState())
	assert.Equal(t, "INVOKE", response.AgentEvent.EventType)
	assert.InDelta(t, time.Now().Add(100*time.Millisecond).UnixNano()/int64(time.Millisecond), response.AgentEvent.DeadlineMs, 5)
	assert.Equal(t, requestID, response.RequestID)
	assert.Equal(t, functionArn, response.InvokedFunctionArn)
	assert.Equal(t, model.XRayTracingType, response.Tracing.Type)
	assert.Equal(t, traceID, response.Tracing.Value)
}

func TestRenderAgentInternalShutdownEvent(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	agent, err := registrationService.CreateInternalAgent("dummyName")
	assert.NoError(t, err)

	agent.SetState(agent.RegisteredState)
	agent.Release()

	renderingService := rendering.NewRenderingService()
	deadlineMs := time.Now().UnixNano() / (1000 * 1000)
	shutdownReason := "spindown"
	renderingService.SetRenderer(
		&rendering.ShutdownRenderer{
			AgentEvent: model.AgentShutdownEvent{
				AgentEvent: &model.AgentEvent{
					EventType:  "SHUTDOWN",
					DeadlineMs: int64(deadlineMs),
				},
				ShutdownReason: shutdownReason,
			},
		})

	handler := NewAgentNextHandler(registrationService, renderingService)
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	var response model.AgentShutdownEvent
	respBody, _ := io.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &response)

	assert.Equal(t, agent.RunningState, agent.GetState())
	assert.Equal(t, "SHUTDOWN", response.AgentEvent.EventType)
	assert.Equal(t, int64(deadlineMs), response.AgentEvent.DeadlineMs)
	assert.Equal(t, shutdownReason, response.ShutdownReason)
}

func TestRenderAgentExternalShutdownEvent(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	agent, err := registrationService.CreateExternalAgent("dummyName")
	assert.NoError(t, err)

	agent.SetState(agent.RegisteredState)
	agent.Release()

	renderingService := rendering.NewRenderingService()
	deadlineMs := time.Now().UnixNano() / (1000 * 1000)
	shutdownReason := "spindown"
	renderingService.SetRenderer(
		&rendering.ShutdownRenderer{
			AgentEvent: model.AgentShutdownEvent{
				AgentEvent: &model.AgentEvent{
					EventType:  "SHUTDOWN",
					DeadlineMs: int64(deadlineMs),
				},
				ShutdownReason: shutdownReason,
			},
		})

	handler := NewAgentNextHandler(registrationService, renderingService)
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	var response model.AgentShutdownEvent
	respBody, _ := io.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &response)

	assert.Equal(t, agent.RunningState, agent.GetState())
	assert.Equal(t, "SHUTDOWN", response.AgentEvent.EventType)
	assert.Equal(t, int64(deadlineMs), response.AgentEvent.DeadlineMs)
	assert.Equal(t, shutdownReason, response.ShutdownReason)
}

func TestRenderAgentInvokeNextHappyEmptyTraceID(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	agent, err := registrationService.CreateExternalAgent("dummyName")
	assert.NoError(t, err)

	agent.SetState(agent.RegisteredState)
	agent.Release() // sets operator condition to true so that the thread doesn't suspend waiting for invoke request

	deadlineNs := metering.Monotime() + int64(100*time.Millisecond)
	requestID, functionArn := "ID", "InvokedFunctionArn"
	traceID := ""
	invoke := &interop.Invoke{
		TraceID:            traceID,
		ID:                 requestID,
		InvokedFunctionArn: functionArn,
		DeadlineNs:         fmt.Sprintf("%d", deadlineNs),
		ContentType:        "image/png",
		Payload:            strings.NewReader("Payload"),
	}

	renderingService := rendering.NewRenderingService()
	var buf bytes.Buffer
	renderingService.SetRenderer(rendering.NewInvokeRenderer(context.Background(), invoke, &buf, telemetry.NewNoOpTracer().BuildTracingHeader()))

	handler := NewAgentNextHandler(registrationService, renderingService)
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	var response model.AgentInvokeEvent
	respBody, _ := io.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &response)

	assert.Nil(t, response.Tracing)
}
