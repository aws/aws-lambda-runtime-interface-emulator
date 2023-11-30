// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi/model"
	"go.amzn.com/lambda/testdata"

	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"
)

// TestInvocationErrorHandler tests that API handler for
// invocation-time errors receives and passes information
// through to the Slicer unmodified.
func TestInvocationErrorHandler(t *testing.T) {
	t.Run("GA", func(t *testing.T) { runTestInvocationErrorHandler(t) })
}

func addInvocationID(r *http.Request, invokeID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("awsrequestid", invokeID)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func runTestInvocationErrorHandler(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	flowTest.Runtime.Ready()
	handler := NewInvocationErrorHandler(flowTest.RegistrationService)
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
		ContentType:           "image/png",
		Payload:               strings.NewReader("Payload1"),
	}

	flowTest.ConfigureForInvoke(context.Background(), invoke)

	// Error request, as submitted by custom runtime.
	errorBody := []byte("My byte array is yours")
	errorType := "ErrorType"
	errorContentType := "application/MyBinaryType"

	request := appctx.RequestWithAppCtx(httptest.NewRequest("POST", "/", bytes.NewReader(errorBody)), appCtx)
	request = addInvocationID(request, invoke.ID)

	request.Header.Set("Content-Type", errorContentType)
	request.Header.Set("Lambda-runtime-functioN-erroR-typE", errorType) // Headers are case-insensitive anyway !

	// Submit !
	handler.ServeHTTP(responseRecorder, request)

	// Assertions

	// Validate response sent to the runtime.
	assert.Equal(t, http.StatusAccepted, responseRecorder.Code, "Handler returned wrong status code: got %v expected %v",
		responseRecorder.Code, http.StatusAccepted)
	assert.JSONEq(t, fmt.Sprintf("{\"status\":\"%s\"}\n", "OK"), responseRecorder.Body.String())
	assert.Equal(t, "application/json", responseRecorder.Header().Get("Content-Type"))

	errorResponse := flowTest.InteropServer.ErrorResponse
	assert.NotNil(t, errorResponse)
	assert.Nil(t, flowTest.InteropServer.Response)

	// Slicer falls back to using ErrorMessage when error
	// payload is not provided. This fallback is not part
	// of the RAPID API spec and is not available to
	// customers.
	assert.Equal(t, "", errorResponse.FunctionError.Message)

	// Slicer falls back to using ErrorType when error
	// payload is not provided. Customers can set error
	// type header to use this fallback.
	assert.Equal(t, fatalerror.RuntimeUnknown, errorResponse.FunctionError.Type)

	// Payload is arbitrary data that customers submit - it's error response body.
	assert.Equal(t, errorBody, errorResponse.Payload)
}

func TestInvocationErrorHandlerRemovesErrorCauseFromResponse(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	flowTest.Runtime.Ready()
	handler := NewInvocationErrorHandler(flowTest.RegistrationService)
	responseRecorder := httptest.NewRecorder()
	appCtx := flowTest.AppCtx

	invoke := &interop.Invoke{ID: "InvocationID1"}

	flowTest.ConfigureForInvoke(context.Background(), invoke)

	// Error request, as submitted by custom runtime.
	errMsg, errType := "foo", "foo"

	errorCause := json.RawMessage(`{"paths":[],"working_directory":[],"exceptions":[]}`)
	errorWithCause := errorWithCauseRequest{
		ErrorMessage: errMsg,
		ErrorType:    errType,
		ErrorCause:   errorCause,
	}

	requestBody, err := json.Marshal(errorWithCause)
	assert.NoError(t, err, "error while creating test request")

	errorContentType := errorWithCauseContentType

	request := appctx.RequestWithAppCtx(httptest.NewRequest("POST", "/", bytes.NewReader(requestBody)), appCtx)
	request = addInvocationID(request, invoke.ID)
	request.Header.Set("Content-Type", errorContentType)

	handler.ServeHTTP(responseRecorder, request)

	expectedResponsePayload := []byte(fmt.Sprintf(`{"errorMessage":"%s","errorType":"%s"}`, errMsg, errType))

	errorResponse := flowTest.InteropServer.ErrorResponse
	assert.NotNil(t, errorResponse)
	assert.Nil(t, flowTest.InteropServer.Response)

	// Payload is arbitrary data that customers submit - it's error response body.
	assert.JSONEq(t, string(expectedResponsePayload), string(errorResponse.Payload))
}

//////////////////////////////////////////////
///// Tests for error.cause Content-Type /////
//////////////////////////////////////////////

func TestInvocationErrorHandlerSendsErrorCauseToXRayForContentTypeErrorCause(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	flowTest.Runtime.Ready()
	handler := NewInvocationErrorHandler(flowTest.RegistrationService)
	responseRecorder := httptest.NewRecorder()
	appCtx := flowTest.AppCtx

	errorCause := json.RawMessage(`{"paths":[],"working_directory":"/foo/bar/baz","exceptions":[]}`)
	errorWithCause := errorWithCauseRequest{
		ErrorMessage: "foo",
		ErrorType:    "bar",
		ErrorCause:   errorCause,
	}

	requestBody, err := json.Marshal(errorWithCause)
	assert.NoError(t, err, "error while creating test request")

	invoke := &interop.Invoke{TraceID: "Root=TraceID;Parent=ParentID;Sampled=1", ID: "InvokeID"}

	request := httptest.NewRequest("POST", "/", bytes.NewReader(requestBody))
	request = addInvocationID(request, invoke.ID)
	request.Header.Set("Content-Type", errorWithCauseContentType)

	// Corresponding invoke must be placed into appCtx.
	flowTest.ConfigureForInvoke(context.Background(), invoke)

	// Run
	handler.ServeHTTP(responseRecorder, appctx.RequestWithAppCtx(request, appCtx))

	// Assert error response contains error cause
	invokeErrorTraceData := appctx.LoadInvokeErrorTraceData(appCtx)
	assert.NotNil(t, invokeErrorTraceData)
	assert.Nil(t, flowTest.InteropServer.Response)
	assert.JSONEq(t, string(errorCause), string(invokeErrorTraceData.ErrorCause))
}

func TestInvocationErrorHandlerSendsNullErrorCauseWhenErrorCauseFormatIsInvalidOrEmptyForContentTypeErrorCause(t *testing.T) {
	causes := []json.RawMessage{
		json.RawMessage(`{"foobar":"baz"}`),
		json.RawMessage(`""`),
	}
	for _, cause := range causes {
		flowTest := testdata.NewFlowTest()
		flowTest.ConfigureForInit()
		flowTest.Runtime.Ready()
		appCtx := flowTest.AppCtx

		errorWithCause := errorWithCauseRequest{
			ErrorMessage: "foo",
			ErrorType:    "bar",
			ErrorCause:   json.RawMessage(cause),
		}

		requestBody, err := json.Marshal(errorWithCause)
		assert.NoError(t, err, "error while creating test request")

		invoke := &interop.Invoke{TraceID: "Root=TraceID;Parent=ParentID;Sampled=1", ID: "InvokeID"}
		request := httptest.NewRequest("POST", "/", bytes.NewReader(requestBody))
		request = addInvocationID(request, invoke.ID)
		request.Header.Set("Content-Type", errorWithCauseContentType)

		// Corresponding invoke must be placed into appCtx.
		flowTest.ConfigureForInvoke(context.Background(), invoke)

		// Run
		NewInvocationErrorHandler(flowTest.RegistrationService).ServeHTTP(httptest.NewRecorder(), appctx.RequestWithAppCtx(request, appCtx))

		invokeErrorTraceData := appctx.LoadInvokeErrorTraceData(appCtx)
		assert.NotNil(t, invokeErrorTraceData)
		assert.Nil(t, flowTest.InteropServer.Response)
		assert.Equal(t, json.RawMessage(nil), invokeErrorTraceData.ErrorCause)
	}
}

func TestInvocationErrorHandlerSendsCompactedErrorCauseWhenErrorCauseIsTooLargeForContentTypeErrorCause(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	flowTest.Runtime.Ready()
	appCtx := flowTest.AppCtx

	cause := json.RawMessage(`{"working_directory": "` + strings.Repeat(`a`, model.MaxErrorCauseSizeBytes+1) + `"}`)

	errorWithCause := errorWithCauseRequest{
		ErrorMessage: "foo",
		ErrorType:    "bar",
		ErrorCause:   json.RawMessage(cause),
	}

	requestBody, err := json.Marshal(errorWithCause)
	assert.NoError(t, err, "error while creating test request")

	invoke := &interop.Invoke{TraceID: "Root=TraceID;Parent=ParentID;Sampled=1", ID: "InvokeID"}
	request := httptest.NewRequest("POST", "/", bytes.NewReader(requestBody))
	request = addInvocationID(request, invoke.ID)
	request.Header.Set("Content-Type", errorWithCauseContentType)

	// Corresponding invoke must be placed into appCtx.
	flowTest.ConfigureForInvoke(context.Background(), invoke)

	// Run
	NewInvocationErrorHandler(flowTest.RegistrationService).ServeHTTP(httptest.NewRecorder(), appctx.RequestWithAppCtx(request, appCtx))

	invokeErrorTraceData := appctx.LoadInvokeErrorTraceData(appCtx)
	assert.NotNil(t, invokeErrorTraceData)
	assert.Nil(t, flowTest.InteropServer.Response)

	errorCauseJSON, err := model.ValidatedErrorCauseJSON(invokeErrorTraceData.ErrorCause)
	assert.NoError(t, err, "expected cause sent x-ray to be valid")
	assert.True(t, len(errorCauseJSON) < model.MaxErrorCauseSizeBytes, "expected cause to be compacted to size")
}

func TestInvocationResponsePayloadIsDefaultErrorMessageWhenRequestParsingFailsForContentTypeErrorCause(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	flowTest.Runtime.Ready()
	appCtx := flowTest.AppCtx

	invalidRequestBody := json.RawMessage(`{"foo":bar}`)

	invoke := &interop.Invoke{TraceID: "Root=TraceID;Parent=ParentID;Sampled=1", ID: "InvokeID"}
	request := httptest.NewRequest("POST", "/", bytes.NewReader(invalidRequestBody))
	request = addInvocationID(request, invoke.ID)
	request.Header.Set(contentTypeHeader, errorWithCauseContentType)
	request.Header.Set(functionResponseModeHeader, "function-response-mode")

	// Corresponding invoke must be placed into appCtx.
	flowTest.ConfigureForInvoke(context.Background(), invoke)

	// Run
	NewInvocationErrorHandler(flowTest.RegistrationService).ServeHTTP(httptest.NewRecorder(), appctx.RequestWithAppCtx(request, appCtx))

	invokeErrorTraceData := appctx.LoadInvokeErrorTraceData(appCtx)
	assert.NotNil(t, invokeErrorTraceData)
	assert.Nil(t, flowTest.InteropServer.Response)
	assert.Equal(t, "application/octet-stream", flowTest.InteropServer.ResponseContentType)
	assert.Equal(t, "function-response-mode", flowTest.InteropServer.FunctionResponseMode)

	errorResponse := flowTest.InteropServer.ErrorResponse
	invokeResponsePayload := errorResponse.Payload

	expectedResponse, _ := json.Marshal(invalidErrorBodyMessage)
	assert.Equal(t, invokeResponsePayload, expectedResponse)
}

//////////////////////////////////////////////
///// Tests for X-Ray Error-Cause header /////
//////////////////////////////////////////////

func TestInvocationErrorHandlerSendsErrorCauseToXRayWhenXRayErrorCauseHeaderIsSet(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	flowTest.Runtime.Ready()
	appCtx := flowTest.AppCtx

	invoke := &interop.Invoke{TraceID: "Root=TraceID;Parent=ParentID;Sampled=1", ID: "InvokeID"}
	request := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(`foo doesn't matter`)))
	request = addInvocationID(request, invoke.ID)
	errorCause := json.RawMessage(`{"paths":[],"working_directory":"/foo/bar/baz","exceptions":[]}`)
	request.Header.Set(xrayErrorCauseHeaderName, string(errorCause))

	// Corresponding invoke must be placed into appCtx.
	flowTest.ConfigureForInvoke(context.Background(), invoke)

	// Run
	NewInvocationErrorHandler(flowTest.RegistrationService).ServeHTTP(httptest.NewRecorder(), appctx.RequestWithAppCtx(request, appCtx))

	invokeErrorTraceData := appctx.LoadInvokeErrorTraceData(appCtx)
	assert.NotNil(t, invokeErrorTraceData)
	assert.Nil(t, flowTest.InteropServer.Response)
	assert.JSONEq(t, string(errorCause), string(invokeErrorTraceData.ErrorCause))
}

func TestInvocationErrorHandlerSendsNilCauseToXRayWhenXRayErrorCauseHeaderContainsInvalidCause(t *testing.T) {
	invalidCauses := []json.RawMessage{
		json.RawMessage(`{invalid:json}`),
		json.RawMessage(``),
	}

	for _, errorCause := range invalidCauses {
		flowTest := testdata.NewFlowTest()
		flowTest.ConfigureForInit()
		flowTest.Runtime.Ready()
		appCtx := flowTest.AppCtx

		invoke := &interop.Invoke{TraceID: "Root=TraceID;Parent=ParentID;Sampled=1", ID: "InvokeID"}
		request := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(`foo doesn't matter`)))
		request = addInvocationID(request, invoke.ID)
		request.Header.Set(xrayErrorCauseHeaderName, string(errorCause))

		// Corresponding invoke must be placed into appCtx.
		flowTest.ConfigureForInvoke(context.Background(), invoke)

		// Run
		NewInvocationErrorHandler(flowTest.RegistrationService).ServeHTTP(httptest.NewRecorder(), appctx.RequestWithAppCtx(request, appCtx))

		invokeErrorTraceData := appctx.LoadInvokeErrorTraceData(appCtx)
		assert.NotNil(t, invokeErrorTraceData)
		assert.Nil(t, flowTest.InteropServer.Response)
		assert.Equal(t, json.RawMessage(nil), invokeErrorTraceData.ErrorCause)
	}
}

func TestInvocationErrorHandlerSendsCompactedErrorCauseToXRayWhenXRayErrorCauseInHeaderIsTooLarge(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	flowTest.Runtime.Ready()
	appCtx := flowTest.AppCtx

	invoke := &interop.Invoke{TraceID: "Root=TraceID;Parent=ParentID;Sampled=1", ID: "InvokeID"}
	request := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(`foo doesn't matter`)))
	request = addInvocationID(request, invoke.ID)

	errorCause := json.RawMessage(`{"working_directory": "` + strings.Repeat(`a`, model.MaxErrorCauseSizeBytes+1) + `"}`)
	request.Header.Set(xrayErrorCauseHeaderName, string(errorCause))

	// Corresponding invoke must be placed into appCtx.
	flowTest.ConfigureForInvoke(context.Background(), invoke)

	// Run
	NewInvocationErrorHandler(flowTest.RegistrationService).ServeHTTP(httptest.NewRecorder(), appctx.RequestWithAppCtx(request, appCtx))

	invokeErrorTraceData := appctx.LoadInvokeErrorTraceData(appCtx)
	assert.NotNil(t, invokeErrorTraceData)
	assert.Nil(t, flowTest.InteropServer.Response)

	errorCauseJSON, err := model.ValidatedErrorCauseJSON(invokeErrorTraceData.ErrorCause)
	assert.NoError(t, err, "expected cause sent x-ray to be valid")
	assert.True(t, len(errorCauseJSON) < model.MaxErrorCauseSizeBytes, "expected cause to be compacted to size")
}

func TestInvocationErrorHandlerSendsNilToXRayWhenXRayErrorCauseHeaderIsNotSet(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	flowTest.Runtime.Ready()
	appCtx := flowTest.AppCtx

	invoke := &interop.Invoke{TraceID: "Root=TraceID;Parent=ParentID;Sampled=1", ID: "InvokeID"}
	request := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(`foo doesn't matter`)))
	request = addInvocationID(request, invoke.ID)

	// Corresponding invoke must be placed into appCtx.
	flowTest.ConfigureForInvoke(context.Background(), invoke)

	// Run
	NewInvocationErrorHandler(flowTest.RegistrationService).ServeHTTP(httptest.NewRecorder(), appctx.RequestWithAppCtx(request, appCtx))

	invokeErrorTraceData := appctx.LoadInvokeErrorTraceData(appCtx)
	assert.NotNil(t, invokeErrorTraceData)
	assert.Nil(t, flowTest.InteropServer.Response)
	assert.Nil(t, invokeErrorTraceData.ErrorCause)
}

func TestInvocationErrorHandlerSendsErrorCauseToXRayWhenXRayErrorCauseContainsUTF8Characters(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	flowTest.Runtime.Ready()
	appCtx := flowTest.AppCtx

	invoke := &interop.Invoke{TraceID: "Root=TraceID;Parent=ParentID;Sampled=1", ID: "InvokeID"}
	request := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(`foo doesn't matter`)))
	request = addInvocationID(request, invoke.ID)

	errorCause := json.RawMessage(`{"exceptions":[],"working_directory":"κόσμε","paths":[]}`)
	request.Header.Set(xrayErrorCauseHeaderName, string(errorCause))

	// Corresponding invoke must be placed into appCtx.
	flowTest.ConfigureForInvoke(context.Background(), invoke)

	// Run
	NewInvocationErrorHandler(flowTest.RegistrationService).ServeHTTP(httptest.NewRecorder(), appctx.RequestWithAppCtx(request, appCtx))

	invokeErrorTraceData := appctx.LoadInvokeErrorTraceData(appCtx)
	assert.NotNil(t, invokeErrorTraceData)
	assert.Nil(t, flowTest.InteropServer.Response)
	assert.JSONEq(t, string(errorCause), string(invokeErrorTraceData.ErrorCause))
}
