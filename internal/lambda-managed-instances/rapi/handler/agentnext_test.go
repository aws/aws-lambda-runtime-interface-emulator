// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
)

func TestRenderAgentInternalError(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization())
	handler := NewAgentNextHandler(registrationService, rendering.NewRenderingService())
	request := httptest.NewRequest("GET", "/", nil)

	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
}

func TestRenderAgentInvokeUnknownAgent(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization())
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), model.AgentIDCtxKey, uuid.New()))
	responseRecorder := httptest.NewRecorder()

	handler := NewAgentNextHandler(registrationService, rendering.NewRenderingService())
	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

	var errorResponse model.ErrorResponse
	respBody, _ := io.ReadAll(responseRecorder.Body)
	require.NoError(t, json.Unmarshal(respBody, &errorResponse))
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assert.Equal(t, errAgentIdentifierUnknown, errorResponse.ErrorType)
}

func TestRenderAgentInvokeInvalidAgentState(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization())

	agent, err := registrationService.CreateExternalAgent("dummyName")
	assert.NoError(t, err)
	handler := NewAgentNextHandler(registrationService, rendering.NewRenderingService())
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), model.AgentIDCtxKey, agent.ID()))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

	var errorResponse model.ErrorResponse
	respBody, _ := io.ReadAll(responseRecorder.Body)
	require.NoError(t, json.Unmarshal(respBody, &errorResponse))
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assert.Equal(t, errAgentInvalidState, errorResponse.ErrorType)
}

func TestRenderAgentInvokeNextHappy(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization())
	agent, err := registrationService.CreateExternalAgent("dummyName")
	assert.NoError(t, err)

	agent.SetState(agent.RegisteredState)
	agent.Release()

	deadlineNs := time.Now().Add(100 * time.Millisecond)
	invokeID, functionArn := "ID", "InvokedFunctionArn"
	traceID := "Root=RootID;Parent=LambdaFrontend;Sampled=1"
	invoke := &interop.Invoke{
		TraceID:               traceID,
		ID:                    invokeID,
		InvokedFunctionArn:    functionArn,
		CognitoIdentityID:     "CognitoIdentityId1",
		CognitoIdentityPoolID: "CognitoIdentityPoolId1",
		ClientContext:         "ClientContext",
		Deadline:              deadlineNs,
		ContentType:           "image/png",
		Payload:               strings.NewReader("Payload"),
	}

	renderingService := rendering.NewRenderingService()
	var buf bytes.Buffer
	renderingService.SetRenderer(rendering.NewInvokeRenderer(context.Background(), invoke, &buf, func(context.Context) string { return "" }))

	handler := NewAgentNextHandler(registrationService, renderingService)
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), model.AgentIDCtxKey, agent.ID()))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	var response model.AgentInvokeEvent
	respBody, _ := io.ReadAll(responseRecorder.Body)
	require.NoError(t, json.Unmarshal(respBody, &response))

	assert.Equal(t, agent.RunningState, agent.GetState())
	assert.Equal(t, "INVOKE", response.EventType)
	assert.InDelta(t, time.Now().Add(100*time.Millisecond).UnixNano()/int64(time.Millisecond), response.DeadlineMs, 5)
	assert.Equal(t, invokeID, response.RequestID)
	assert.Equal(t, functionArn, response.InvokedFunctionArn)
	assert.Equal(t, model.XRayTracingType, response.Tracing.Type)
	assert.Equal(t, traceID, response.Tracing.Value)
}

func TestRenderAgentInternalInvokeNextHappy(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization())
	agent, err := registrationService.CreateInternalAgent("dummyName")
	assert.NoError(t, err)

	agent.SetState(agent.RegisteredState)
	agent.Release()

	deadlineNs := time.Now().Add(100 * time.Millisecond)
	invokeID, functionArn := "ID", "InvokedFunctionArn"
	traceID := "Root=RootID;Parent=LambdaFrontend;Sampled=1"
	invoke := &interop.Invoke{
		TraceID:               traceID,
		ID:                    invokeID,
		InvokedFunctionArn:    functionArn,
		CognitoIdentityID:     "CognitoIdentityId1",
		CognitoIdentityPoolID: "CognitoIdentityPoolId1",
		ClientContext:         "ClientContext",
		Deadline:              deadlineNs,
		ContentType:           "image/png",
		Payload:               strings.NewReader("Payload"),
	}

	renderingService := rendering.NewRenderingService()
	var buf bytes.Buffer
	renderingService.SetRenderer(rendering.NewInvokeRenderer(context.Background(), invoke, &buf, func(context.Context) string { return "" }))

	handler := NewAgentNextHandler(registrationService, renderingService)
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), model.AgentIDCtxKey, agent.ID()))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	var response model.AgentInvokeEvent
	respBody, _ := io.ReadAll(responseRecorder.Body)
	require.NoError(t, json.Unmarshal(respBody, &response))

	assert.Equal(t, agent.RunningState, agent.GetState())
	assert.Equal(t, "INVOKE", response.EventType)
	assert.InDelta(t, time.Now().Add(100*time.Millisecond).UnixNano()/int64(time.Millisecond), response.DeadlineMs, 5)
	assert.Equal(t, invokeID, response.RequestID)
	assert.Equal(t, functionArn, response.InvokedFunctionArn)
	assert.Equal(t, model.XRayTracingType, response.Tracing.Type)
	assert.Equal(t, traceID, response.Tracing.Value)
}

func TestRenderAgentInternalShutdownEvent(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization())
	agent, err := registrationService.CreateInternalAgent("dummyName")
	assert.NoError(t, err)

	agent.SetState(agent.RegisteredState)
	agent.Release()

	renderingService := rendering.NewRenderingService()
	deadlineMs := time.Now().UnixNano() / (1000 * 1000)
	renderingService.SetRenderer(
		&rendering.ShutdownRenderer{
			AgentEvent: model.AgentShutdownEvent{
				AgentEvent: &model.AgentEvent{
					EventType:  "SHUTDOWN",
					DeadlineMs: int64(deadlineMs),
				},
				ShutdownReason: model.Spindown,
			},
		})

	handler := NewAgentNextHandler(registrationService, renderingService)
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), model.AgentIDCtxKey, agent.ID()))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	var response model.AgentShutdownEvent
	respBody, _ := io.ReadAll(responseRecorder.Body)
	require.NoError(t, json.Unmarshal(respBody, &response))

	assert.Equal(t, agent.RunningState, agent.GetState())
	assert.Equal(t, "SHUTDOWN", response.EventType)
	assert.Equal(t, int64(deadlineMs), response.DeadlineMs)
	assert.Equal(t, model.Spindown, response.ShutdownReason)
}

func TestRenderAgentExternalShutdownEvent(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization())
	agent, err := registrationService.CreateExternalAgent("dummyName")
	assert.NoError(t, err)

	agent.SetState(agent.RegisteredState)
	agent.Release()

	renderingService := rendering.NewRenderingService()
	deadlineMs := time.Now().UnixNano() / (1000 * 1000)
	renderingService.SetRenderer(
		&rendering.ShutdownRenderer{
			AgentEvent: model.AgentShutdownEvent{
				AgentEvent: &model.AgentEvent{
					EventType:  "SHUTDOWN",
					DeadlineMs: int64(deadlineMs),
				},
				ShutdownReason: model.Spindown,
			},
		})

	handler := NewAgentNextHandler(registrationService, renderingService)
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), model.AgentIDCtxKey, agent.ID()))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	var response model.AgentShutdownEvent
	respBody, _ := io.ReadAll(responseRecorder.Body)
	require.NoError(t, json.Unmarshal(respBody, &response))

	assert.Equal(t, agent.RunningState, agent.GetState())
	assert.Equal(t, "SHUTDOWN", response.EventType)
	assert.Equal(t, int64(deadlineMs), response.DeadlineMs)
	assert.Equal(t, model.Spindown, response.ShutdownReason)
}

func TestRenderAgentInvokeNextHappyEmptyTraceID(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization())
	agent, err := registrationService.CreateExternalAgent("dummyName")
	assert.NoError(t, err)

	agent.SetState(agent.RegisteredState)
	agent.Release()

	deadlineNs := time.Now().Add(100 * time.Millisecond)
	invokeID, functionArn := "ID", "InvokedFunctionArn"
	traceID := ""
	invoke := &interop.Invoke{
		TraceID:            traceID,
		ID:                 invokeID,
		InvokedFunctionArn: functionArn,
		Deadline:           deadlineNs,
		ContentType:        "image/png",
		Payload:            strings.NewReader("Payload"),
	}

	renderingService := rendering.NewRenderingService()
	var buf bytes.Buffer
	renderingService.SetRenderer(rendering.NewInvokeRenderer(context.Background(), invoke, &buf, func(context.Context) string { return "" }))

	handler := NewAgentNextHandler(registrationService, renderingService)
	request := httptest.NewRequest("GET", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), model.AgentIDCtxKey, agent.ID()))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	var response model.AgentInvokeEvent
	respBody, _ := io.ReadAll(responseRecorder.Body)
	require.NoError(t, json.Unmarshal(respBody, &response))

	assert.Nil(t, response.Tracing)
}
