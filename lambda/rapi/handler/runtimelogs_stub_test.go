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

func TestSuccessfulRuntimeLogsStub202Response(t *testing.T) {
	handler := NewRuntimeLogsStubHandler()
	requestBody := []byte(`foobar`)
	request := httptest.NewRequest("PUT", "/logs", bytes.NewBuffer(requestBody))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
	assert.JSONEq(t, `{"errorMessage":"Logs API is not supported","errorType":"Logs.NotSupported"}`, responseRecorder.Body.String())
}
