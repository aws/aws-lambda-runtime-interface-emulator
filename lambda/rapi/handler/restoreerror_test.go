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
	"go.amzn.com/lambda/testdata"
)

func TestRestoreErrorHandler(t *testing.T) {
	t.Run("GA", func(t *testing.T) { runTestRestoreErrorHandler(t) })
}

func runTestRestoreErrorHandler(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForRestoring()

	handler := NewRestoreErrorHandler(flowTest.RegistrationService)
	responseRecorder := httptest.NewRecorder()
	appCtx := flowTest.AppCtx

	errorBody := []byte("My byte array is yours")
	errorType := "ErrorType"
	errorContentType := "application/MyBinaryType"

	request := appctx.RequestWithAppCtx(httptest.NewRequest("POST", "/", bytes.NewReader(errorBody)), appCtx)

	request.Header.Set("Content-Type", errorContentType)
	request.Header.Set("Lambda-Runtime-Function-Error-Type", errorType)

	handler.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusAccepted, responseRecorder.Code, "Handler returned wrong status code: got %v expected %v", responseRecorder.Code, http.StatusAccepted)
	require.JSONEq(t, fmt.Sprintf("{\"status\":\"%s\"}\n", "OK"), responseRecorder.Body.String())
	require.Equal(t, "application/json", responseRecorder.Header().Get("Content-Type"))
}
