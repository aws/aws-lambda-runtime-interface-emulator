// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/extensions"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapi/handler"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapi/model"
)

type mockHandler struct{}

func (h *mockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

func TestRuntimeReleaseMiddleware(t *testing.T) {
	appCtx := appctx.NewApplicationContext()
	router := chi.NewRouter()
	handler := &mockHandler{}
	router.Use(RuntimeReleaseMiddleware())
	router.Get("/", handler.ServeHTTP)

	userAgent := "foobar"

	responseRecorder := httptest.NewRecorder()
	responseBody := make([]byte, 100)
	request := httptest.NewRequest("GET", "/", bytes.NewReader(responseBody))
	request.Header.Set("User-Agent", userAgent)
	router.ServeHTTP(responseRecorder, appctx.RequestWithAppCtx(request, appCtx))

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
	ctxRuntimeRelease, ok := appCtx.Load(appctx.AppCtxRuntimeReleaseKey)
	assert.True(t, ok)
	assert.Equal(t, userAgent, ctxRuntimeRelease)
}

func TestAgentUniqueIdentifierHeaderValidatorForbidden(t *testing.T) {
	router := chi.NewRouter()
	mockHandler := &mockHandler{}
	router.Get("/", AgentUniqueIdentifierHeaderValidator(mockHandler).ServeHTTP)
	responseBody := make([]byte, 100)
	var errorResponse model.ErrorResponse

	request := httptest.NewRequest("GET", "/", bytes.NewReader(responseBody))

	responseRecorder := httptest.NewRecorder()
	router.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	respBody, _ := io.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &errorResponse)
	assert.Equal(t, handler.ErrAgentIdentifierMissing, errorResponse.ErrorType)

	responseRecorder = httptest.NewRecorder()
	request.Header.Set(handler.LambdaAgentIdentifier, "invalid-unique-identifier")
	router.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	respBody, _ = io.ReadAll(responseRecorder.Body)
	json.Unmarshal(respBody, &errorResponse)
	assert.Equal(t, handler.ErrAgentIdentifierInvalid, errorResponse.ErrorType)
}

func TestAgentUniqueIdentifierHeaderValidatorSuccess(t *testing.T) {
	router := chi.NewRouter()
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		val, ok := r.Context().Value(handler.AgentIDCtxKey).(uuid.UUID)
		if !ok {
			assert.FailNow(t, "expected key not in request context")
		}
		assert.Equal(t, "85083764-ff1e-476f-ada1-d51f26e4f6be", val.String())
	})
	router.Get("/", AgentUniqueIdentifierHeaderValidator(mockHandler).ServeHTTP)
	responseBody := make([]byte, 100)
	request := httptest.NewRequest("GET", "/", bytes.NewReader(responseBody))
	ctx := context.Background()
	request = request.WithContext(ctx)

	responseRecorder := httptest.NewRecorder()
	responseRecorder.Code = http.StatusOK
	request.Header.Set(handler.LambdaAgentIdentifier, "85083764-ff1e-476f-ada1-d51f26e4f6be")
	router.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
}

func TestAllowIfExtensionsEnabledPositive(t *testing.T) {
	router := chi.NewRouter()
	handler := &mockHandler{}
	router.Use(AllowIfExtensionsEnabled)
	router.Get("/", handler.ServeHTTP)

	responseRecorder := httptest.NewRecorder()
	responseBody := make([]byte, 100)

	extensions.Enable()
	defer extensions.Disable()

	router.ServeHTTP(responseRecorder, httptest.NewRequest("GET", "/", bytes.NewReader(responseBody)))

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
}

func TestAllowIfExtensionsEnabledNegative(t *testing.T) {
	router := chi.NewRouter()
	handler := &mockHandler{}
	router.Use(AllowIfExtensionsEnabled)
	router.Get("/", handler.ServeHTTP)

	responseRecorder := httptest.NewRecorder()
	responseBody := make([]byte, 100)
	router.ServeHTTP(responseRecorder, httptest.NewRequest("GET", "/", bytes.NewReader(responseBody)))

	assert.Equal(t, http.StatusNotFound, responseRecorder.Code)
}
