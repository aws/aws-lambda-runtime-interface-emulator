// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi/model"

	"go.amzn.com/lambda/testdata"
)

func createInvoke(id string) *interop.Invoke {
	return &interop.Invoke{
		ID:                 id,
		InvokedFunctionArn: "arn::dummy:Function",
		Payload:            strings.NewReader("{\"invoke\":\"" + id + "\"}"),
		DeadlineNs:         "123456",
	}
}

// Make a test request
func makeTestRequest(t *testing.T, router http.Handler, request *http.Request) *httptest.ResponseRecorder {
	responseRecorder := httptest.NewRecorder()
	router.ServeHTTP(responseRecorder, request)
	t.Logf("test(%v) = %v", request.URL, responseRecorder.Code)
	return responseRecorder
}

// Make a test request in a benchmark
func makeBenchRequest(b *testing.B, router http.Handler, request *http.Request) *httptest.ResponseRecorder {
	responseRecorder := httptest.NewRecorder()
	b.StartTimer()
	router.ServeHTTP(responseRecorder, request)
	b.StopTimer()
	return responseRecorder
}

// Verify response error type
func assertResponseErrorType(t *testing.T, expectedErrorType string, response *httptest.ResponseRecorder) {
	errResp := model.ErrorResponse{}
	err := json.Unmarshal(response.Body.Bytes(), &errResp)
	assert.Nil(t, err)
	assert.Equal(t, expectedErrorType, errResp.ErrorType)
}

// TestAcceptXML tests that server response is always
// rendered as JSON, regardless of the value provided
// in "Accept" header.
//
// When using render.Render(...), chi rendering library
// would attempt to render response using content type
// specified in the "Accept" header.
//
// The purpose of this test is to confirm that RAPID
// renders all server responses as application/json.
func TestAcceptXML(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/runtime/invocation/x-y-z/error", bytes.NewReader([]byte("")))
	// Tell server that client side accepts "application/xml".
	request.Header.Add("Accept", "application/xml")
	// Serve request.
	router.ServeHTTP(responseRecorder, request)
	v := &model.ErrorResponse{}
	// Expected response is 403 transition is not allowed, rendered as JSON.
	err := json.Unmarshal(responseRecorder.Body.Bytes(), v)
	if err != nil {
		t.Error("Expected JSON document, received: ", responseRecorder.Body.String())
	}
	assert.Equal(t, "InvalidRequestID", v.ErrorType)
	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
}

// Verify that unsupported methods return 404
func Test404PageNotFound(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/unsupported", bytes.NewReader([]byte(""))))
	assert.Equal(t, http.StatusNotFound, responseRecorder.Code)
	assert.Equal(t, "404 page not found\n", responseRecorder.Body.String())
}

func Test405MethodNotAllowed(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("DELETE", "/runtime/invocation/ABC/error", bytes.NewReader([]byte(""))))
	assert.Equal(t, http.StatusMethodNotAllowed, responseRecorder.Code)
}

func TestInitErrorAccepted(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/init/error", bytes.NewReader([]byte("{}"))))
	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
}

func TestInitErrorForbidden(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	flowTest.ConfigureForInvoke(context.Background(), createInvoke("InvokeA"))
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/init/error", bytes.NewReader([]byte("{}"))))
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
}

func TestInvokeResponseAccepted(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	flowTest.ConfigureForInvoke(context.Background(), createInvoke("InvokeA"))
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/InvokeA/response", bytes.NewReader([]byte("{}"))))
	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
}

func TestInvokeErrorResponseAccepted(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	flowTest.ConfigureForInvoke(context.Background(), createInvoke("InvokeA"))
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/InvokeA/error", bytes.NewReader([]byte("{}"))))
	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
}

func TestInvokeNextTwice(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	flowTest.ConfigureForInvoke(context.Background(), createInvoke("InvokeA"))
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
}

func TestInvokeResponseInvalidRequestID(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	flowTest.ConfigureForInvoke(context.Background(), createInvoke("ABC"))
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/XYZ/response", bytes.NewReader([]byte("{}"))))
	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	assertResponseErrorType(t, "InvalidRequestID", responseRecorder)
}

func TestInvokeErrorResponseInvalidRequestID(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	flowTest.ConfigureForInvoke(context.Background(), createInvoke("ABC"))
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/XYZ/error", bytes.NewReader([]byte("{}"))))
	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	assertResponseErrorType(t, "InvalidRequestID", responseRecorder)
}

func TestInvokeResponseTwice(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	flowTest.ConfigureForInvoke(context.Background(), createInvoke("ABC"))
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/ABC/response", bytes.NewReader([]byte("{}"))))
	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/ABC/response", bytes.NewReader([]byte("{}"))))
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assertResponseErrorType(t, "InvalidStateTransition", responseRecorder)
}

func TestInvokeErrorResponseTwice(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	flowTest.ConfigureForInvoke(context.Background(), createInvoke("ABC"))
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/ABC/error", bytes.NewReader([]byte("{}"))))
	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/ABC/error", bytes.NewReader([]byte("{}"))))
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assertResponseErrorType(t, "InvalidStateTransition", responseRecorder)
}

func TestInvokeResponseAfterErrorResponse(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	flowTest.ConfigureForInvoke(context.Background(), createInvoke("ABC"))
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/ABC/error", bytes.NewReader([]byte("{}"))))
	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/ABC/response", bytes.NewReader([]byte("{}"))))
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assertResponseErrorType(t, "InvalidStateTransition", responseRecorder)
}

func TestInvokeErrorResponseAfterResponse(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	flowTest.ConfigureForInvoke(context.Background(), createInvoke("ABC"))
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/ABC/response", bytes.NewReader([]byte("{}"))))
	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/ABC/error", bytes.NewReader([]byte("{}"))))
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assertResponseErrorType(t, "InvalidStateTransition", responseRecorder)
}

func TestMoreThanOneInvoke(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	var responseRecorder *httptest.ResponseRecorder
	for _, id := range []string{"A", "B", "C"} {
		flowTest.ConfigureForInvoke(context.Background(), createInvoke(id))
		responseRecorder = makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", fmt.Sprintf("/runtime/invocation/%s/response", id), bytes.NewReader([]byte("{}"))))
		assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
	}
}

func benchmarkInvoke(b *testing.B, payload []byte) {
	b.StopTimer()
	b.ReportAllocs()
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)
	for i := 0; i < b.N; i++ {
		id := uuid.New().String()
		flowTest.ConfigureForInvoke(context.Background(), createInvoke(id))
		makeBenchRequest(b, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))
		makeBenchRequest(b, router, httptest.NewRequest("POST", fmt.Sprintf("/runtime/invocation/%s/response", id), bytes.NewReader(payload)))
	}
}

func BenchmarkInvokeWithEmptyPayload(b *testing.B) {
	benchmarkInvoke(b, []byte(""))
}

func BenchmarkInvokeWith4KBPayload(b *testing.B) {
	benchmarkInvoke(b, bytes.Repeat([]byte("a"), 4*1024))
}

func BenchmarkInvokeWith512KBPayload(b *testing.B) {
	benchmarkInvoke(b, bytes.Repeat([]byte("a"), 512*1024))
}

func BenchmarkInvokeWith1MBPayload(b *testing.B) {
	benchmarkInvoke(b, bytes.Repeat([]byte("a"), 1*1024*1024))
}

func BenchmarkInvokeWith2MBPayload(b *testing.B) {
	benchmarkInvoke(b, bytes.Repeat([]byte("a"), 2*1024*1024))
}

func BenchmarkInvokeWith4MBPayload(b *testing.B) {
	benchmarkInvoke(b, bytes.Repeat([]byte("a"), 4*1024*1024))
}

func BenchmarkInvokeWith6MBPayload(b *testing.B) {
	benchmarkInvoke(b, bytes.Repeat([]byte("a"), 6*1024*1024))
}
