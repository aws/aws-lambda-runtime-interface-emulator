// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.amzn.com/lambda/testdata"
)

// Verify that state machine will accept response and error with valid invoke id
func TestInvokeValidId(t *testing.T) {

	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)

	flowTest.ConfigureForInvoke(context.Background(), createInvoke("InvokeA"))

	// Send /next to start Invoke A
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))

	assert.Equal(t, http.StatusOK, responseRecorder.Code)

	// Send invocation response with correct Invoke Id
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/InvokeA/response", bytes.NewReader([]byte("{}"))))

	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)

	flowTest.ConfigureForInvoke(context.Background(), createInvoke("InvokeB"))

	// Send /next to start Invoke B
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))

	assert.Equal(t, http.StatusOK, responseRecorder.Code)

	// Send invocation error with correct Invoke id
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/InvokeB/error", bytes.NewReader([]byte("{}"))))

	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
}

// All invoke responses must be validated to ensure they use the active Invoke request-id
// This is a Security requirement
func TestSecurityInvokeResponseBadRequestId(t *testing.T) {

	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)

	flowTest.ConfigureForInvoke(context.Background(), createInvoke("InvokeA"))

	// Try to use the invoke id before next
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/InvokeA/response", bytes.NewReader([]byte("{}"))))

	// NOTE: InvalidStateTransition 403 - forbidden by the state machine
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assertResponseErrorType(t, "InvalidStateTransition", responseRecorder)

	// Send /next to start Invoke A
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))

	assert.Equal(t, http.StatusOK, responseRecorder.Code)

	// Send invocation response with invalid invoke id
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/InvokeZ/response", bytes.NewReader([]byte("{}"))))

	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	assertResponseErrorType(t, "InvalidRequestID", responseRecorder)

	// Send invocation response with correct Invoke Id
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/InvokeA/response", bytes.NewReader([]byte("{}"))))

	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)

	flowTest.ConfigureForInvoke(context.Background(), createInvoke("InvokeB"))

	// Send /next to start new Invoke
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))

	assert.Equal(t, http.StatusOK, responseRecorder.Code)

	// Try to re-use the old invoke id
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/InvokeA/response", bytes.NewReader([]byte("{}"))))

	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	assertResponseErrorType(t, "InvalidRequestID", responseRecorder)
}

// All invoke errors must be validated to ensure they use the active Invoke request-id
// This is a Security requirement
func TestSecurityInvokeErrorBadRequestId(t *testing.T) {

	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	router := NewRouter(flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService)

	flowTest.ConfigureForInvoke(context.Background(), createInvoke("InvokeA"))

	// Try to use invoke id before next
	responseRecorder := makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/InvokeA/error", bytes.NewReader([]byte("{}"))))

	// NOTE: InvalidStateTransition 403 - forbidden by the state machine
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assertResponseErrorType(t, "InvalidStateTransition", responseRecorder)

	// Send /next to start Invoke A
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))

	assert.Equal(t, http.StatusOK, responseRecorder.Code)

	// Send invocation response with invalid invoke id
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/InvokeZ/error", bytes.NewReader([]byte("{}"))))

	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	assertResponseErrorType(t, "InvalidRequestID", responseRecorder)

	// Send invocation error with correct Invoke Id
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/InvokeA/error", bytes.NewReader([]byte("{}"))))

	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)

	flowTest.ConfigureForInvoke(context.Background(), createInvoke("InvokeB"))

	// Send /next to start Invoke B
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("GET", "/runtime/invocation/next", nil))

	assert.Equal(t, http.StatusOK, responseRecorder.Code)

	// Try to re-use the previous invoke id
	responseRecorder = makeTestRequest(t, router, httptest.NewRequest("POST", "/runtime/invocation/InvokeA/error", bytes.NewReader([]byte("{}"))))

	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	assertResponseErrorType(t, "InvalidRequestID", responseRecorder)
}
