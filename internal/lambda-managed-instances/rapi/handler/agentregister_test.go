// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func registerRequestReader(req RegisterRequest) io.Reader {
	body, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(body)
}

func TestRenderAgentRegisterInvalidAgentName(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization())

	handler := NewAgentRegisterHandler(registrationService)
	request := httptest.NewRequest("POST", "/extension/register", registerRequestReader(RegisterRequest{}))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	require.Equal(t, http.StatusForbidden, responseRecorder.Code)

	var errorResponse model.ErrorResponse
	respBody, _ := io.ReadAll(responseRecorder.Body)
	require.NoError(t, json.Unmarshal(respBody, &errorResponse))
	require.Equal(t, http.StatusForbidden, responseRecorder.Code)
	require.Equal(t, errAgentNameInvalid, errorResponse.ErrorType)
}

func TestRenderAgentRegisterRegistrationClosed(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization())
	registrationService.TurnOff()

	handler := NewAgentRegisterHandler(registrationService)
	request := httptest.NewRequest("POST", "/extension/register", registerRequestReader(RegisterRequest{}))
	request.Header.Add(LambdaAgentName, "dummyName")
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	require.Equal(t, http.StatusForbidden, responseRecorder.Code)

	var errorResponse model.ErrorResponse
	respBody, _ := io.ReadAll(responseRecorder.Body)
	require.NoError(t, json.Unmarshal(respBody, &errorResponse))
	require.Equal(t, http.StatusForbidden, responseRecorder.Code)
	require.Equal(t, errAgentRegistrationClosed, errorResponse.ErrorType)
}

func TestRenderAgentRegisterInvalidAgentState(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization())

	agent, err := registrationService.CreateExternalAgent("dummyName")
	require.NoError(t, err)
	agent.SetState(agent.RegisteredState)

	handler := NewAgentRegisterHandler(registrationService)
	request := httptest.NewRequest("POST", "/extension/register", registerRequestReader(RegisterRequest{}))
	request.Header.Add(LambdaAgentName, "dummyName")
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	require.Equal(t, http.StatusForbidden, responseRecorder.Code)

	var errorResponse model.ErrorResponse
	respBody, _ := io.ReadAll(responseRecorder.Body)
	require.NoError(t, json.Unmarshal(respBody, &errorResponse))
	require.Equal(t, http.StatusForbidden, responseRecorder.Code)
	require.Equal(t, errAgentInvalidState, errorResponse.ErrorType)
}

func registerAgent(t *testing.T, agentName string, events []core.Event, registerHandler http.Handler) {
	request := httptest.NewRequest("POST", "/extension/register", registerRequestReader(RegisterRequest{Events: events}))
	request.Header.Add(LambdaAgentName, agentName)
	responseRecorder := httptest.NewRecorder()
	registerHandler.ServeHTTP(responseRecorder, request)
	require.Equal(t, http.StatusOK, responseRecorder.Code)
}

func TestGetSubscribedExternalAgents(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization())
	_, err := registrationService.CreateExternalAgent("externalInvokeAgent")
	require.NoError(t, err)
	_, err = registrationService.CreateExternalAgent("externalShutdownAgent")
	require.NoError(t, err)

	handler := NewAgentRegisterHandler(registrationService)

	registerAgent(t, "externalShutdownAgent", []core.Event{core.ShutdownEvent}, handler)
	registerAgent(t, "internalInvokeAgent", []core.Event{}, handler)

	subscribers := registrationService.GetSubscribedExternalAgents(core.ShutdownEvent)
	require.Equal(t, 1, len(subscribers))
	require.Equal(t, "externalShutdownAgent", subscribers[0].Name())
}

func TestExternalAgentInvalidEventType(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization())
	agentName := "ABC"
	_, err := registrationService.CreateExternalAgent(agentName)
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		request := httptest.NewRequest("POST", "/extension/register", registerRequestReader(RegisterRequest{Events: []core.Event{"abcdef"}}))
		request.Header.Add(LambdaAgentName, agentName)

		responseRecorder := httptest.NewRecorder()
		NewAgentRegisterHandler(registrationService).ServeHTTP(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		response := model.ErrorResponse{}
		require.NoError(t, json.Unmarshal(responseRecorder.Body.Bytes(), &response))
		require.Equal(t, errInvalidEventType, response.ErrorType)
		require.Contains(t, response.ErrorMessage, "abcdef")

		_, found := registrationService.FindExternalAgentByName(agentName)
		require.True(t, found)

		shutdownSubscribers := registrationService.GetSubscribedExternalAgents(core.ShutdownEvent)
		require.Equal(t, 0, len(shutdownSubscribers))

		require.Equal(t, 1, registrationService.CountAgents())
	}
}

type ExtensionRegisterResponseWithConfig struct {
	model.ExtensionRegisterResponse
	Configuration map[string]string `json:"configuration"`
}

func TestRenderAgentResponse(t *testing.T) {
	defaultFunctionMetadata := rapidmodel.FunctionMetadata{
		FunctionVersion: "$LATEST",
		FunctionName:    "my-func",
		Handler:         "lambda_handler",
	}

	happyPathTests := map[string]struct {
		agentName           string
		external            bool
		registrationRequest RegisterRequest
		featuresHeader      string
		functionMetadata    rapidmodel.FunctionMetadata
		expectedResponse    string
	}{
		"no-config-internal": {
			agentName:           "internal",
			external:            false,
			functionMetadata:    defaultFunctionMetadata,
			registrationRequest: RegisterRequest{},
			expectedResponse: `{
				"functionName": "my-func",
				"functionVersion": "$LATEST",
				"handler": "lambda_handler"
			}`,
		},
		"no-config-external": {
			agentName:           "external",
			external:            true,
			functionMetadata:    defaultFunctionMetadata,
			registrationRequest: RegisterRequest{},
			expectedResponse: `{
				"functionName": "my-func",
				"functionVersion": "$LATEST",
				"handler": "lambda_handler"
			}`,
		},
		"function-md-override": {
			agentName:           "external",
			external:            true,
			functionMetadata:    rapidmodel.FunctionMetadata{FunctionName: "function-name", FunctionVersion: "1", Handler: "myHandler"},
			registrationRequest: RegisterRequest{},
			expectedResponse: `{
				"functionName": "function-name",
				"functionVersion": "1",
				"handler": "myHandler"
			}`,
		},
		"internal with account id feature": {
			agentName: "internal",
			external:  false,
			functionMetadata: rapidmodel.FunctionMetadata{
				FunctionName:    "function-name",
				FunctionVersion: "1",
				Handler:         "myHandler",
				AccountID:       "0123",
			},
			featuresHeader:      "accountId",
			registrationRequest: RegisterRequest{},
			expectedResponse: `{
				"functionName": "function-name",
				"functionVersion": "1",
				"handler": "myHandler",
				"accountId": "0123"
			}`,
		},
		"external with account id feature": {
			agentName: "external",
			external:  true,
			functionMetadata: rapidmodel.FunctionMetadata{
				FunctionName:    "function-name",
				FunctionVersion: "1",
				Handler:         "myHandler",
				AccountID:       "0123",
			},
			featuresHeader:      "accountId",
			registrationRequest: RegisterRequest{},
			expectedResponse: `{
				"functionName": "function-name",
				"functionVersion": "1",
				"handler": "myHandler",
				"accountId": "0123"
			}`,
		},
		"with non-existing accept feature": {
			agentName:           "external",
			external:            true,
			featuresHeader:      "some_non_existing_feature,",
			functionMetadata:    defaultFunctionMetadata,
			registrationRequest: RegisterRequest{},
			expectedResponse: `{
				"functionName": "my-func",
				"functionVersion": "$LATEST",
				"handler": "lambda_handler"
			}`,
		},
		"account id feature and some non-existing feature": {
			agentName:      "external",
			external:       true,
			featuresHeader: "some_non_existing_feature,accountId,",
			functionMetadata: rapidmodel.FunctionMetadata{
				FunctionName:    "function-name",
				FunctionVersion: "1",
				Handler:         "myHandler",
				AccountID:       "0123",
			},
			registrationRequest: RegisterRequest{},
			expectedResponse: `{
				"functionName": "function-name",
				"functionVersion": "1",
				"handler": "myHandler",
				"accountId": "0123"
			}`,
		},
		"with empty account id data": {
			agentName:           "external",
			external:            true,
			featuresHeader:      "accountId",
			functionMetadata:    defaultFunctionMetadata,
			registrationRequest: RegisterRequest{},
			expectedResponse: `{
				"functionName": "my-func",
				"functionVersion": "$LATEST",
				"handler": "lambda_handler"
			}`,
		},
	}

	for name, tt := range happyPathTests {
		t.Run(name, func(t *testing.T) {
			registrationService := core.NewRegistrationService(
				core.NewInitFlowSynchronization())
			_, _ = registrationService.CreateExternalAgent("external")
			registrationService.SetFunctionMetadata(tt.functionMetadata)

			handler := NewAgentRegisterHandler(registrationService)

			request := httptest.NewRequest("POST", "/extension/register", registerRequestReader(tt.registrationRequest))
			request.Header.Add(LambdaAgentName, tt.agentName)
			if tt.featuresHeader != "" {
				request.Header.Add(featuresHeader, tt.featuresHeader)
			}
			responseRecorder := httptest.NewRecorder()

			handler.ServeHTTP(responseRecorder, request)
			assert.Equal(t, http.StatusOK, responseRecorder.Code)

			respBody, err := io.ReadAll(responseRecorder.Body)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expectedResponse, string(respBody))

			if tt.external {
				agent, found := registrationService.FindExternalAgentByName(tt.agentName)
				assert.True(t, found)
				assert.Equal(t, agent.RegisteredState, agent.GetState())
			} else {
				agent, found := registrationService.FindInternalAgentByName(tt.agentName)
				assert.True(t, found)
				assert.Equal(t, agent.RegisteredState, agent.GetState())
			}
		})
	}
}
