// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/rapi/model"
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
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	handler := NewAgentRegisterHandler(registrationService)
	request := httptest.NewRequest("POST", "/extension/register", registerRequestReader(RegisterRequest{}))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	require.Equal(t, http.StatusForbidden, responseRecorder.Code)

	var errorResponse model.ErrorResponse
	respBody, _ := ioutil.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &errorResponse)
	require.Equal(t, http.StatusForbidden, responseRecorder.Code)
	require.Equal(t, errAgentNameInvalid, errorResponse.ErrorType)
}

func TestRenderAgentRegisterRegistrationClosed(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	registrationService.TurnOff()

	handler := NewAgentRegisterHandler(registrationService)
	request := httptest.NewRequest("POST", "/extension/register", registerRequestReader(RegisterRequest{}))
	request.Header.Add(LambdaAgentName, "dummyName")
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)
	require.Equal(t, http.StatusForbidden, responseRecorder.Code)

	var errorResponse model.ErrorResponse
	respBody, _ := ioutil.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &errorResponse)
	require.Equal(t, http.StatusForbidden, responseRecorder.Code)
	require.Equal(t, errAgentRegistrationClosed, errorResponse.ErrorType)
}

func TestRenderAgentRegisterInvalidAgentState(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

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
	respBody, _ := ioutil.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &errorResponse)
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
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	registrationService.CreateExternalAgent("externalInvokeAgent")
	registrationService.CreateExternalAgent("externalShutdownAgent")

	handler := NewAgentRegisterHandler(registrationService)

	registerAgent(t, "externalInvokeAgent", []core.Event{core.InvokeEvent}, handler)
	registerAgent(t, "externalShutdownAgent", []core.Event{core.ShutdownEvent}, handler)
	registerAgent(t, "internalInvokeAgent", []core.Event{core.InvokeEvent}, handler)

	subscribers := registrationService.GetSubscribedExternalAgents(core.InvokeEvent)
	require.Equal(t, 1, len(subscribers))
	require.Equal(t, "externalInvokeAgent", subscribers[0].Name)
}

func TestInternalAgentShutdownSubscription(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	request := httptest.NewRequest("POST", "/extension/register", registerRequestReader(RegisterRequest{Events: []core.Event{core.ShutdownEvent}}))
	agentName := "internalShutdownAgent"
	request.Header.Add(LambdaAgentName, agentName)

	responseRecorder := httptest.NewRecorder()
	NewAgentRegisterHandler(registrationService).ServeHTTP(responseRecorder, request)
	require.Equal(t, http.StatusForbidden, responseRecorder.Code)

	response := model.ErrorResponse{}
	json.Unmarshal(responseRecorder.Body.Bytes(), &response)
	require.Equal(t, errInvalidEventType, response.ErrorType)
	require.Contains(t, response.ErrorMessage, string(core.ShutdownEvent))

	_, found := registrationService.FindInternalAgentByName(agentName)
	require.False(t, found)

	subscribers := registrationService.GetSubscribedInternalAgents(core.ShutdownEvent)
	require.Equal(t, 0, len(subscribers))
}

func TestInternalAgentInvalidEventType(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	for i := 0; i < 2; i++ { // make the request twice to make sure invalid /register request doesn't change agent state
		request := httptest.NewRequest("POST", "/extension/register", registerRequestReader(RegisterRequest{Events: []core.Event{"abcdef"}}))
		agentName := "internalShutdownAgent"
		request.Header.Add(LambdaAgentName, agentName)

		responseRecorder := httptest.NewRecorder()
		NewAgentRegisterHandler(registrationService).ServeHTTP(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		response := model.ErrorResponse{}
		json.Unmarshal(responseRecorder.Body.Bytes(), &response)
		require.Equal(t, errInvalidEventType, response.ErrorType)
		require.Contains(t, response.ErrorMessage, "abcdef")

		_, found := registrationService.FindInternalAgentByName(agentName)
		require.False(t, found)

		subscribers := registrationService.GetSubscribedInternalAgents(core.ShutdownEvent)
		require.Equal(t, 0, len(subscribers))
	}
}

func TestExternalAgentInvalidEventType(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	agentName := "ABC"
	registrationService.CreateExternalAgent(agentName)

	for i := 0; i < 2; i++ { // make the request twice to make sure invalid /register request doesn't change agent state
		request := httptest.NewRequest("POST", "/extension/register", registerRequestReader(RegisterRequest{Events: []core.Event{"abcdef"}}))
		request.Header.Add(LambdaAgentName, agentName)

		responseRecorder := httptest.NewRecorder()
		NewAgentRegisterHandler(registrationService).ServeHTTP(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)

		response := model.ErrorResponse{}
		json.Unmarshal(responseRecorder.Body.Bytes(), &response)
		require.Equal(t, errInvalidEventType, response.ErrorType)
		require.Contains(t, response.ErrorMessage, "abcdef")

		_, found := registrationService.FindExternalAgentByName(agentName)
		require.True(t, found)

		subscribers := registrationService.GetSubscribedExternalAgents(core.ShutdownEvent)
		require.Equal(t, 0, len(subscribers))
	}
}

func TestGetSubscribedInternalAgents(t *testing.T) {
	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)
	registrationService.CreateExternalAgent("externalInvokeAgent")
	registrationService.CreateExternalAgent("externalShutdownAgent")

	handler := NewAgentRegisterHandler(registrationService)

	registerAgent(t, "externalInvokeAgent", []core.Event{core.InvokeEvent}, handler)
	registerAgent(t, "externalShutdownAgent", []core.Event{core.ShutdownEvent}, handler)
	registerAgent(t, "internalInvokeAgent", []core.Event{core.InvokeEvent}, handler)

	subscribers := registrationService.GetSubscribedInternalAgents(core.InvokeEvent)
	require.Equal(t, 1, len(subscribers))
	require.Equal(t, "internalInvokeAgent", subscribers[0].Name)
}

type ExtensionRegisterResponseWithConfig struct {
	model.ExtensionRegisterResponse
	Configuration map[string]string `json:"configuration"`
}

var happyPathTests = []struct {
	testName                     string
	agentName                    string
	external                     bool
	registrationRequest          RegisterRequest
	functionMetadata             *core.FunctionMetadata
	expectedRegistrationResponse ExtensionRegisterResponseWithConfig
}{
	{
		testName:            "no-config-internal",
		agentName:           "internal",
		external:            false,
		registrationRequest: RegisterRequest{},
		expectedRegistrationResponse: ExtensionRegisterResponseWithConfig{
			ExtensionRegisterResponse: model.ExtensionRegisterResponse{
				FunctionName:    "my-func",
				FunctionVersion: "$LATEST",
				Handler:         "lambda_handler",
			},
		},
	},
	{
		testName:            "no-config-external",
		agentName:           "external",
		external:            true,
		registrationRequest: RegisterRequest{},
		expectedRegistrationResponse: ExtensionRegisterResponseWithConfig{
			ExtensionRegisterResponse: model.ExtensionRegisterResponse{
				FunctionName:    "my-func",
				FunctionVersion: "$LATEST",
				Handler:         "lambda_handler",
			},
		},
	},
	{
		testName:            "function-md-override",
		agentName:           "external",
		external:            true,
		functionMetadata:    &core.FunctionMetadata{FunctionName: "function-name", FunctionVersion: "1", Handler: "myHandler"},
		registrationRequest: RegisterRequest{},
		expectedRegistrationResponse: ExtensionRegisterResponseWithConfig{
			ExtensionRegisterResponse: model.ExtensionRegisterResponse{
				FunctionName:    "function-name",
				FunctionVersion: "1",
				Handler:         "myHandler",
			},
		},
	},
}

func TestRenderAgentResponse(t *testing.T) {
	defaultFunctionMetadata := core.FunctionMetadata{
		FunctionVersion: "$LATEST",
		FunctionName:    "my-func",
		Handler:         "lambda_handler",
	}

	for _, tt := range happyPathTests {
		t.Run(tt.testName, func(t *testing.T) {
			registrationService := core.NewRegistrationService(
				core.NewInitFlowSynchronization(),
				core.NewInvokeFlowSynchronization(),
			)
			registrationService.CreateExternalAgent("external") // external agent has to be pre-registered
			if tt.functionMetadata != nil {
				registrationService.SetFunctionMetadata(*tt.functionMetadata)
			} else {
				registrationService.SetFunctionMetadata(defaultFunctionMetadata)
			}

			handler := NewAgentRegisterHandler(registrationService)

			request := httptest.NewRequest("POST", "/extension/register", registerRequestReader(tt.registrationRequest))
			request.Header.Add(LambdaAgentName, tt.agentName)
			responseRecorder := httptest.NewRecorder()

			handler.ServeHTTP(responseRecorder, request)
			require.Equal(t, http.StatusOK, responseRecorder.Code)

			registerResponse := ExtensionRegisterResponseWithConfig{}
			respBody, _ := ioutil.ReadAll(responseRecorder.Body)
			json.Unmarshal(respBody, &registerResponse)
			assert.Equal(t, tt.expectedRegistrationResponse.FunctionName, registerResponse.FunctionName)
			assert.Equal(t, tt.expectedRegistrationResponse.FunctionVersion, registerResponse.FunctionVersion)
			assert.Equal(t, tt.expectedRegistrationResponse.Handler, registerResponse.Handler)

			require.Len(t, registerResponse.Configuration, 0)

			if tt.external {
				agent, found := registrationService.FindExternalAgentByName(tt.agentName)
				require.True(t, found)
				require.Equal(t, agent.RegisteredState, agent.GetState())
			} else {
				agent, found := registrationService.FindInternalAgentByName(tt.agentName)
				require.True(t, found)
				require.Equal(t, agent.RegisteredState, agent.GetState())
			}
		})
	}
}
