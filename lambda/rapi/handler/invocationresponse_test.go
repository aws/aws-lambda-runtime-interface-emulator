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

	"github.com/aws/aws-lambda-go/events/test"
	"github.com/stretchr/testify/assert"
	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/testdata"
)

func TestResponseTooLarge(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	flowTest.Runtime.Ready()
	handler := NewInvocationResponseHandler(flowTest.RegistrationService)
	responseRecorder := httptest.NewRecorder()
	appCtx := flowTest.AppCtx

	// Invoke that we are sending response for must be placed into appCtx.
	invoke := &interop.Invoke{
		ID:                    "InvocationID1",
		InvokedFunctionArn:    "arn::dummy1",
		CognitoIdentityID:     "CognitoidentityID1",
		CognitoIdentityPoolID: "CognitoidentityPollID1",
		DeadlineNs:            "deadlinens1",
		ClientContext:         "clientcontext1",
		ContentType:           "application/json",
		Payload:               strings.NewReader(`{"message": "hello"}`),
	}

	flowTest.ConfigureForInvoke(context.Background(), invoke)

	// Invocation response submitted by runtime.
	var responseBody = make([]byte, interop.MaxPayloadSize+1)
	request := httptest.NewRequest("", "/", bytes.NewReader(responseBody))
	request = addInvocationID(request, invoke.ID)
	handler.ServeHTTP(responseRecorder, appctx.RequestWithAppCtx(request, appCtx))

	// Assertions

	assert.Equal(t, http.StatusRequestEntityTooLarge, responseRecorder.Code, "Handler returned wrong status code: got %v expected %v",
		responseRecorder.Code, http.StatusRequestEntityTooLarge)

	expectedAPIResponse := fmt.Sprintf("{\"errorMessage\":\"Exceeded maximum allowed payload size (%d bytes).\",\"errorType\":\"RequestEntityTooLarge\"}\n", interop.MaxPayloadSize)
	body, err := io.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)
	test.AssertJsonsEqual(t, []byte(expectedAPIResponse), body)

	errorResponse := flowTest.InteropServer.ErrorResponse
	assert.NotNil(t, errorResponse)
	assert.Nil(t, flowTest.InteropServer.Response)
	assert.Equal(t, fatalerror.FunctionOversizedResponse, errorResponse.FunctionError.Type)
	assert.Equal(t, "Response payload size (6291557 bytes) exceeded maximum allowed payload size (6291556 bytes).", errorResponse.FunctionError.Message)

	var errorPayload map[string]interface{}
	assert.NoError(t, json.Unmarshal(errorResponse.Payload, &errorPayload))
	assert.Equal(t, string(errorResponse.FunctionError.Type), errorPayload["errorType"])
	assert.Equal(t, errorResponse.FunctionError.Message, errorPayload["errorMessage"])
}

func TestResponseAccepted(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	flowTest.Runtime.Ready()
	handler := NewInvocationResponseHandler(flowTest.RegistrationService)
	responseRecorder := httptest.NewRecorder()
	appCtx := flowTest.AppCtx

	// Invoke that we are sending response for must be placed into appCtx.
	invoke := &interop.Invoke{
		ID:                    "InvocationID1",
		InvokedFunctionArn:    "arn::dummy1",
		CognitoIdentityID:     "CognitoidentityID1",
		CognitoIdentityPoolID: "CognitoidentityPollID1",
		DeadlineNs:            "deadlinens1",
		ClientContext:         "clientcontext1",
		ContentType:           "application/json",
		Payload:               strings.NewReader(`{"message": "hello"}`),
	}

	flowTest.ConfigureForInvoke(context.Background(), invoke)

	// Invocation response submitted by runtime.
	var responseBody = []byte("{'foo': 'bar'}")

	request := httptest.NewRequest("", "/", bytes.NewReader(responseBody))
	request = addInvocationID(request, invoke.ID)
	request.Header.Set(contentTypeHeader, "application/json")
	request.Header.Set(functionResponseModeHeader, "streaming")
	handler.ServeHTTP(responseRecorder, appctx.RequestWithAppCtx(request, appCtx))

	// Assertions
	assert.Equal(t, http.StatusAccepted, responseRecorder.Code, "Handler returned wrong status code: got %v expected %v",
		responseRecorder.Code, http.StatusAccepted)

	expectedAPIResponse := "{\"status\":\"OK\"}\n"
	body, err := io.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)
	test.AssertJsonsEqual(t, []byte(expectedAPIResponse), body)

	response := flowTest.InteropServer.Response
	assert.NotNil(t, response)
	assert.Nil(t, flowTest.InteropServer.ErrorResponse)
	assert.Equal(t, "application/json", flowTest.InteropServer.ResponseContentType)
	assert.Equal(t, "streaming", flowTest.InteropServer.FunctionResponseMode)
	assert.Equal(t, responseBody, response,
		"Persisted response data in app context must match the submitted.")
}

func TestResponseWithDifferentFunctionResponseModes(t *testing.T) {
	type testCase struct {
		providedFunctionResponseMode string
		expectedFunctionResponseMode string
		expectedAPIResponse          string
		expectedStatusCode           int
		expectedErrorResponse        bool
	}
	testCases := []testCase{
		{
			providedFunctionResponseMode: "",
			expectedFunctionResponseMode: "",
			expectedAPIResponse:          "{\"status\":\"OK\"}\n",
			expectedStatusCode:           http.StatusAccepted,
			expectedErrorResponse:        false,
		},
		{
			providedFunctionResponseMode: "streaming",
			expectedFunctionResponseMode: "streaming",
			expectedAPIResponse:          "{\"status\":\"OK\"}\n",
			expectedStatusCode:           http.StatusAccepted,
			expectedErrorResponse:        false,
		},
		{
			providedFunctionResponseMode: "invalid-mode",
			expectedFunctionResponseMode: "",
			expectedAPIResponse:          "{\"errorMessage\":\"Invalid function response mode\", \"errorType\":\"InvalidFunctionResponseMode\"}\n",
			expectedStatusCode:           http.StatusBadRequest,
			expectedErrorResponse:        true,
		},
	}

	for _, testCase := range testCases {
		flowTest := testdata.NewFlowTest()
		flowTest.ConfigureForInit()
		flowTest.Runtime.Ready()
		handler := NewInvocationResponseHandler(flowTest.RegistrationService)
		responseRecorder := httptest.NewRecorder()
		appCtx := flowTest.AppCtx

		// Invoke that we are sending response for must be placed into appCtx.
		invoke := &interop.Invoke{
			ID:                    "InvocationID1",
			InvokedFunctionArn:    "arn::dummy1",
			CognitoIdentityID:     "CognitoidentityID1",
			CognitoIdentityPoolID: "CognitoidentityPollID1",
			DeadlineNs:            "deadlinens1",
			ClientContext:         "clientcontext1",
			ContentType:           "application/json",
			Payload:               strings.NewReader(`{"message": "hello"}`),
		}

		flowTest.ConfigureForInvoke(context.Background(), invoke)

		// Invocation response submitted by runtime.
		var responseBody = []byte("{'foo': 'bar'}")

		request := httptest.NewRequest("", "/", bytes.NewReader(responseBody))
		request = addInvocationID(request, invoke.ID)
		request.Header.Set(functionResponseModeHeader, testCase.providedFunctionResponseMode)
		handler.ServeHTTP(responseRecorder, appctx.RequestWithAppCtx(request, appCtx))

		// Assertions
		assert.Equal(t, testCase.expectedStatusCode, responseRecorder.Code, "Handler returned wrong status code: got %v expected %v",
			responseRecorder.Code, testCase.expectedStatusCode)

		body, err := io.ReadAll(responseRecorder.Body)
		assert.NoError(t, err)
		test.AssertJsonsEqual(t, []byte(testCase.expectedAPIResponse), body)

		if testCase.expectedErrorResponse {
			assert.NotNil(t, flowTest.InteropServer.ErrorResponse)
			assert.Nil(t, flowTest.InteropServer.Response)
			assert.Equal(t, fatalerror.RuntimeInvalidResponseModeHeader, flowTest.InteropServer.ErrorResponse.FunctionError.Type)
		} else {
			assert.NotNil(t, flowTest.InteropServer.Response)
			assert.Nil(t, flowTest.InteropServer.ErrorResponse)
			assert.Equal(t, responseBody, flowTest.InteropServer.Response,
				"Persisted response data in app context must match the submitted.")
		}

		assert.Equal(t, testCase.expectedFunctionResponseMode, flowTest.InteropServer.FunctionResponseMode)
	}
}
