// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/testdata"
)

// TestInitErrorHandler tests that API handler for
// initialization-time errors receives and passes
// information through to the Slicer unmodified.
func TestInitErrorHandler(t *testing.T) {
	t.Run("GA", func(t *testing.T) { runTestInitErrorHandler(t) })
}

func runTestInitErrorHandler(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()

	handler := NewInitErrorHandler(flowTest.RegistrationService)
	responseRecorder := httptest.NewRecorder()
	appCtx := flowTest.AppCtx

	// Error request, as submitted by custom runtime.
	errorBody := []byte("My byte array is yours")
	errorType := "ErrorType"
	errorContentType := "application/MyBinaryType"

	request := appctx.RequestWithAppCtx(httptest.NewRequest("POST", "/", bytes.NewReader(errorBody)), appCtx)
	request.Header.Set("Content-Type", errorContentType)
	request.Header.Set("Lambda-runtime-functioN-erroR-typE", errorType) // Headers are case-insensitive anyway !

	// Submit !
	handler.ServeHTTP(responseRecorder, request)

	// Assertions

	// Validate response sent to the runtime.
	require.Equal(t, http.StatusAccepted, responseRecorder.Code, "Handler returned wrong status code: got %v expected %v",
		responseRecorder.Code, http.StatusAccepted)
	require.JSONEq(t, fmt.Sprintf("{\"status\":\"%s\"}\n", "OK"), responseRecorder.Body.String())
	require.Equal(t, "application/json", responseRecorder.Header().Get("Content-Type"))

	// Validate init error persisted in the application context.
	errorResponse := flowTest.InteropServer.ErrorResponse
	require.NotNil(t, errorResponse)
	require.Nil(t, flowTest.InteropServer.Response)

	// Slicer falls back to using ErrorMessage when error
	// payload is not provided. This fallback is not part
	// of the RAPID API spec and is not available to
	// customers.
	require.Equal(t, "", errorResponse.FunctionError.Message)

	// Slicer falls back to using ErrorType when error
	// payload is not provided. Customers can set error
	// type via header to use this fallback.
	require.Equal(t, fatalerror.RuntimeUnknown, errorResponse.FunctionError.Type)

	// Payload is arbitrary data that customers submit - it's error response body.
	require.Equal(t, errorBody, errorResponse.Payload)
}
