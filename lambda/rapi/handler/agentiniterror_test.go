// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/rapi/model"
)

func newRequest(appCtx appctx.ApplicationContext, agentID uuid.UUID) *http.Request {
	request := httptest.NewRequest("POST", "/", nil)
	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agentID))
	request = appctx.RequestWithAppCtx(request, appCtx)
	request.Header.Set(LambdaAgentFunctionErrorType, "Extension.TestError")
	return request
}

func TestAgentInitErrorInternalError(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	handler := NewAgentInitErrorHandler(registrationService)
	request := httptest.NewRequest("POST", "/", nil)
	// request is missing agent's UUID context. This should not happen since the middleware validation should have failed
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
}

func TestAgentInitErrorMissingErrorHeader(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	appCtx := appctx.NewApplicationContext()
	agent, err := registrationService.CreateExternalAgent("dummyName")
	agent.SetState(agent.RegisteredState)
	assert.NoError(t, err)
	handler := NewAgentInitErrorHandler(registrationService)
	responseRecorder := httptest.NewRecorder()

	req := newRequest(appCtx, uuid.New())
	req.Header.Del(LambdaAgentFunctionErrorType)
	handler.ServeHTTP(responseRecorder, req)
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

	var errorResponse model.ErrorResponse
	respBody, _ := io.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &errorResponse)
	assert.Equal(t, errAgentMissingHeader, errorResponse.ErrorType)
}

func TestAgentInitErrorUnknownAgent(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	handler := NewAgentInitErrorHandler(registrationService)
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, newRequest(appctx.NewApplicationContext(), uuid.New()))
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

	var errorResponse model.ErrorResponse
	respBody, _ := io.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &errorResponse)
	assert.Equal(t, errAgentIdentifierUnknown, errorResponse.ErrorType)
}

func TestAgentInitErrorAgentInvalidState(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	// agent is in started state, it is not allowed to transition to init error
	agent, err := registrationService.CreateExternalAgent("dummyName")
	assert.NoError(t, err)
	handler := NewAgentInitErrorHandler(registrationService)
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, newRequest(appctx.NewApplicationContext(), agent.ID))
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

	var errorResponse model.ErrorResponse
	respBody, _ := io.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &errorResponse)
	assert.Equal(t, errAgentInvalidState, errorResponse.ErrorType)
}

func TestAgentInitErrorRequestAccepted(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	appCtx := appctx.NewApplicationContext()
	agent, err := registrationService.CreateExternalAgent("dummyName")
	agent.SetState(agent.RegisteredState)
	assert.NoError(t, err)
	handler := NewAgentInitErrorHandler(registrationService)
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, newRequest(appCtx, agent.ID))
	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)

	var response model.StatusResponse
	respBody, _ := io.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &response)
	assert.Equal(t, "OK", response.Status)

	v, found := appctx.LoadFirstFatalError(appCtx)
	assert.True(t, found)
	assert.Equal(t, fatalerror.AgentInitError, v)
}
