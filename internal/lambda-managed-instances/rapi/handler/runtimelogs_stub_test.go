// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuccessfulRuntimeTelemetryAPIStub202Response(t *testing.T) {
	handler := NewRuntimeTelemetryAPIStubHandler()
	requestBody := []byte(`foobar`)
	request := httptest.NewRequest("PUT", "/telemetry", bytes.NewBuffer(requestBody))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
	assert.JSONEq(t, `{"errorMessage":"Telemetry API is not supported","errorType":"Telemetry.NotSupported"}`, responseRecorder.Body.String())
}
