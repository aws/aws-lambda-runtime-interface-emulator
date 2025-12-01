// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/handler"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/testdata"
)

func FuzzAgentRegisterHandler(f *testing.F) {
	registerReq := handler.RegisterRequest{
		Events: []core.Event{core.ShutdownEvent},
	}
	regReqBytes, err := json.Marshal(&registerReq)
	if err != nil {
		f.Errorf("failed to marshal register request: %v", err)
	}
	f.Add("agent", "accountId", true, regReqBytes)
	f.Add("agent", "accountId", false, regReqBytes)

	f.Fuzz(func(t *testing.T,
		agentName string,
		featuresHeader string,
		external bool,
		payload []byte,
	) {
		flowTest := testdata.NewFlowTest()

		if external {
			_, err := flowTest.RegistrationService.CreateExternalAgent(agentName)
			require.NoError(t, err)
		}

		functionMetadata := createDummyFunctionMetadata()
		flowTest.RegistrationService.SetFunctionMetadata(functionMetadata)

		rapiServer := makeRapiServer(flowTest)

		target := makeTargetURL("/extension/register", version20200101)
		request := httptest.NewRequest("POST", target, bytes.NewReader(payload))
		request.Header.Add(handler.LambdaAgentName, agentName)
		request.Header.Add("Lambda-Extension-Accept-Feature", featuresHeader)

		responseRecorder := serveTestRequest(rapiServer, request)

		if agentName == "" {
			assertForbiddenErrorType(t, responseRecorder, "Extension.InvalidExtensionName")
			return
		}

		regReqStruct := struct {
			handler.RegisterRequest
			ConfigurationKeys []string `json:"configurationKeys"`
		}{}
		if err := json.Unmarshal(payload, &regReqStruct); err != nil {
			assertForbiddenErrorType(t, responseRecorder, "InvalidRequestFormat")
			return
		}

		if containsInvalidEvent(external, regReqStruct.Events) {
			assertForbiddenErrorType(t, responseRecorder, "Extension.InvalidEventType")
			return
		}

		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		respBody, err := io.ReadAll(responseRecorder.Body)
		assert.NoError(t, err)

		expectedResponse := map[string]interface{}{
			"functionName":    functionMetadata.FunctionName,
			"functionVersion": functionMetadata.FunctionVersion,
			"handler":         functionMetadata.Handler,
		}
		if featuresHeader == "accountId" && functionMetadata.AccountID != "" {
			expectedResponse["accountId"] = functionMetadata.AccountID
		}

		expectedRespBytes, err := json.Marshal(expectedResponse)
		assert.NoError(t, err)
		assert.JSONEq(t, string(expectedRespBytes), string(respBody))

		if external {
			agent, found := flowTest.RegistrationService.FindExternalAgentByName(agentName)
			assert.True(t, found)
			assert.Equal(t, agent.RegisteredState, agent.GetState())
		} else {
			agent, found := flowTest.RegistrationService.FindInternalAgentByName(agentName)
			assert.True(t, found)
			assert.Equal(t, agent.RegisteredState, agent.GetState())
		}
	})
}

func FuzzAgentNextHandler(f *testing.F) {
	regService := core.NewRegistrationService(core.NewInitFlowSynchronization())
	testAgent := makeExternalAgent(regService)
	f.Add(testAgent.ID().String(), true)
	f.Add(testAgent.ID().String(), false)

	f.Fuzz(func(t *testing.T,
		agentIdentifierHeader string,
		registered bool,
	) {
		flowTest := testdata.NewFlowTest()
		agent := makeExternalAgent(flowTest.RegistrationService)

		if registered {
			agent.SetState(agent.RegisteredState)
			agent.Release()
		}

		configureRendererForEvent(flowTest)

		rapiServer := makeRapiServer(flowTest)

		target := makeTargetURL("/extension/event/next", version20200101)
		request := httptest.NewRequest("GET", target, nil)
		request.Header.Set(model.LambdaAgentIdentifier, agentIdentifierHeader)

		responseRecorder := serveTestRequest(rapiServer, request)

		if agentIdentifierHeader == "" {
			assertForbiddenErrorType(t, responseRecorder, model.ErrAgentIdentifierMissing)
			return
		}
		if _, err := uuid.Parse(agentIdentifierHeader); err != nil {
			assertForbiddenErrorType(t, responseRecorder, model.ErrAgentIdentifierInvalid)
			return
		}
		if agentIdentifierHeader != agent.ID().String() {
			assertForbiddenErrorType(t, responseRecorder, "Extension.UnknownExtensionIdentifier")
			return
		}
		if !registered {
			assertForbiddenErrorType(t, responseRecorder, "Extension.InvalidExtensionState")
			return
		}

		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		assertResponseEventType(t, responseRecorder)

		assert.Equal(t, agent.RunningState, agent.GetState())
	})
}

func FuzzAgentInitErrorHandler(f *testing.F) {
	fuzzErrorHandler(f, "/extension/init/error", rapidmodel.ErrorAgentInit)
}

func FuzzAgentExitErrorHandler(f *testing.F) {
	fuzzErrorHandler(f, "/extension/exit/error", rapidmodel.ErrorAgentExit)
}

func fuzzErrorHandler(f *testing.F, handlerPath string, fatalErrorType rapidmodel.ErrorType) {
	regService := core.NewRegistrationService(core.NewInitFlowSynchronization())
	testAgent := makeExternalAgent(regService)
	f.Add(true, testAgent.ID().String(), "Extension.SomeError")
	f.Add(false, testAgent.ID().String(), "Extension.SomeError")

	f.Fuzz(func(t *testing.T,
		agentRegistered bool,
		agentIdentifierHeader string,
		errorType string,
	) {
		flowTest := testdata.NewFlowTest()

		agent := makeExternalAgent(flowTest.RegistrationService)

		if agentRegistered {
			agent.SetState(agent.RegisteredState)
		}

		rapiServer := makeRapiServer(flowTest)

		target := makeTargetURL(handlerPath, version20200101)

		request := httptest.NewRequest("POST", target, nil)
		request = appctx.RequestWithAppCtx(request, flowTest.AppCtx)
		request.Header.Set(model.LambdaAgentIdentifier, agentIdentifierHeader)
		request.Header.Set(handler.LambdaAgentFunctionErrorType, errorType)

		responseRecorder := serveTestRequest(rapiServer, request)

		if agentIdentifierHeader == "" {
			assertForbiddenErrorType(t, responseRecorder, model.ErrAgentIdentifierMissing)
			return
		}

		if _, e := uuid.Parse(agentIdentifierHeader); e != nil {
			assertForbiddenErrorType(t, responseRecorder, model.ErrAgentIdentifierInvalid)
			return
		}

		if errorType == "" {
			assertForbiddenErrorType(t, responseRecorder, "Extension.MissingHeader")
			return
		}
		if agentIdentifierHeader != agent.ID().String() {
			assertForbiddenErrorType(t, responseRecorder, "Extension.UnknownExtensionIdentifier")
			return
		}
		if !agentRegistered {
			assertForbiddenErrorType(t, responseRecorder, "Extension.InvalidExtensionState")
		} else {
			assertErrorAgentRegistered(t, responseRecorder, flowTest, fatalErrorType)
		}
	})
}

func assertErrorAgentRegistered(t *testing.T, responseRecorder *httptest.ResponseRecorder, flowTest *testdata.FlowTest, expectedErrType rapidmodel.ErrorType) {
	var response model.StatusResponse

	respBody, _ := io.ReadAll(responseRecorder.Body)
	err := json.Unmarshal(respBody, &response)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
	assert.Equal(t, "OK", response.Status)

	v, found := appctx.LoadFirstFatalError(flowTest.AppCtx)
	assert.True(t, found)
	assert.Equal(t, expectedErrType, v)
}

func assertForbiddenErrorType(t *testing.T, responseRecorder *httptest.ResponseRecorder, errType string) {
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)

	var errorResponse model.ErrorResponse

	respBody, _ := io.ReadAll(responseRecorder.Body)
	err := json.Unmarshal(respBody, &errorResponse)
	assert.NoError(t, err)

	assert.Equal(t, errType, errorResponse.ErrorType)
}

func createDummyFunctionMetadata() rapidmodel.FunctionMetadata {
	return rapidmodel.FunctionMetadata{
		AccountID:       "accID",
		FunctionName:    "myFunc",
		FunctionVersion: "1.0",
		Handler:         "myHandler",
	}
}

func makeExternalAgent(registrationService core.RegistrationService) *core.ExternalAgent {
	agent, err := registrationService.CreateExternalAgent("agent")
	if err != nil {
		slog.Error("failed to create external agent", "error", err)
		panic(err)
	}

	return agent
}

func configureRendererForEvent(flowTest *testdata.FlowTest) {
	flowTest.RenderingService.SetRenderer(
		&rendering.ShutdownRenderer{
			AgentEvent: model.AgentShutdownEvent{
				AgentEvent: &model.AgentEvent{
					EventType:  "SHUTDOWN",
					DeadlineMs: int64(10000),
				},
				ShutdownReason: "spindown",
			},
		})
}

func assertResponseEventType(t *testing.T, responseRecorder *httptest.ResponseRecorder) {
	var response model.AgentShutdownEvent

	respBody, _ := io.ReadAll(responseRecorder.Body)
	err := json.Unmarshal(respBody, &response)
	assert.NoError(t, err)

	assert.Equal(t, "SHUTDOWN", response.EventType)
}

func containsInvalidEvent(external bool, events []core.Event) bool {
	for _, e := range events {
		if external {
			if err := core.ValidateExternalAgentEvent(e); err != nil {
				return true
			}
		} else if len(events) > 0 {
			return true
		}
	}

	return false
}
