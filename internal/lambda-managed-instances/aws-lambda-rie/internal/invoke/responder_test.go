// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/invoke"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func TestResponder_CompleteSuccessFlow(t *testing.T) {

	recorder := httptest.NewRecorder()
	mockInvokeReq := interop.NewMockInvokeRequest(t)
	mockRuntimeReq := invoke.NewMockRuntimeResponseRequest(t)
	mockInitData := interop.NewMockInitStaticDataProvider(t)

	mockInvokeReq.On("ResponseWriter").Return(recorder)
	mockInvokeReq.On("MaxPayloadSize").Return(int64(1024))

	responseBody := "test response body"
	mockRuntimeReq.On("BodyReader").Return(strings.NewReader(responseBody))
	mockRuntimeReq.On("ContentType").Return("application/json")
	mockRuntimeReq.On("ResponseMode").Return("buffered")
	mockRuntimeReq.On("TrailerError").Return(nil)

	responder := NewResponder(mockInvokeReq)
	responder.SendRuntimeResponseHeaders(mockInitData, "", "")
	result := responder.SendRuntimeResponseBody(context.Background(), mockRuntimeReq, 0)
	assert.NoError(t, result.Err)
	responder.SendRuntimeResponseTrailers(mockRuntimeReq)

	assert.Equal(t, "application/json", recorder.Header().Get(invoke.СontentTypeHeader))
	assert.Equal(t, "buffered", recorder.Header().Get(invoke.RuntimeResponseModeHeader))
	assert.Equal(t, responseBody, recorder.Body.String())
	assert.Equal(t, http.StatusOK, recorder.Code)

	mockInvokeReq.AssertExpectations(t)
	mockRuntimeReq.AssertExpectations(t)
}

func TestResponder_SendErrorFlow(t *testing.T) {

	recorder := httptest.NewRecorder()
	mockInvokeReq := interop.NewMockInvokeRequest(t)
	mockInitData := interop.NewMockInitStaticDataProvider(t)

	mockInvokeReq.On("ResponseWriter").Return(recorder)

	baseErr := io.ErrUnexpectedEOF
	appError := model.NewCustomerError("Function.TestError", model.WithCause(baseErr), model.WithErrorMessage("test error"))

	responder := NewResponder(mockInvokeReq)
	responder.SendError(appError, mockInitData)

	assert.Equal(t, "Function.TestError", recorder.Header().Get("Error-Type"))
	assert.Equal(t, `{"errorType":"Function.TestError","errorMessage":"test error"}`, recorder.Body.String())
	assert.Equal(t, http.StatusOK, recorder.Code)

	mockInvokeReq.AssertExpectations(t)
}

func TestResponder_RuntimeInvocationErrorFlow(t *testing.T) {

	recorder := httptest.NewRecorder()
	mockInvokeReq := interop.NewMockInvokeRequest(t)
	mockInitData := interop.NewMockInitStaticDataProvider(t)

	mockInvokeReq.On("ResponseWriter").Return(recorder)

	responder := NewResponder(mockInvokeReq)
	responder.SendRuntimeResponseHeaders(mockInitData, "", "")
	responder.SendErrorTrailers(model.NewCustomerError("Runtime.TestError", model.WithErrorMessage("trailer error")), "")

	assert.Equal(t, "Runtime.TestError", recorder.Header().Get("Error-Type"))
	assert.Equal(t, `{"errorType":"Runtime.TestError","errorMessage":"trailer error"}`, recorder.Body.String())
	assert.Equal(t, http.StatusOK, recorder.Code)

	mockInvokeReq.AssertExpectations(t)
}

func TestResponder_ErrorInTheMiddleOfResponse(t *testing.T) {

	recorder := httptest.NewRecorder()
	mockInvokeReq := interop.NewMockInvokeRequest(t)
	mockRuntimeReq := invoke.NewMockRuntimeResponseRequest(t)
	mockInitData := interop.NewMockInitStaticDataProvider(t)

	mockInvokeReq.On("ResponseWriter").Return(recorder)
	mockInvokeReq.On("MaxPayloadSize").Return(int64(1024))

	responseBody := "test response body"
	mockRuntimeReq.On("BodyReader").Return(strings.NewReader(responseBody))

	responder := NewResponder(mockInvokeReq)
	responder.SendRuntimeResponseHeaders(mockInitData, "", "")
	result := responder.SendRuntimeResponseBody(context.Background(), mockRuntimeReq, 0)
	assert.NoError(t, result.Err)
	responder.SendErrorTrailers(model.NewCustomerError("Sandbox.TestError", model.WithSeverity(model.ErrorSeverityFatal), model.WithErrorMessage("error after body")), "")

	assert.Equal(t, "Sandbox.TestError", recorder.Header().Get("Error-Type"))
	assert.Equal(t, `{"errorType":"Sandbox.TestError","errorMessage":"error after body"}`, recorder.Body.String())
	assert.Equal(t, http.StatusOK, recorder.Code)

	mockInvokeReq.AssertExpectations(t)
	mockRuntimeReq.AssertExpectations(t)
}

func TestResponder_RuntimeResponseTrailerError(t *testing.T) {

	recorder := httptest.NewRecorder()
	mockInvokeReq := interop.NewMockInvokeRequest(t)
	mockRuntimeReq := invoke.NewMockRuntimeResponseRequest(t)
	mockInitData := interop.NewMockInitStaticDataProvider(t)

	mockInvokeReq.On("ResponseWriter").Return(recorder)
	mockInvokeReq.On("MaxPayloadSize").Return(int64(1024))

	errorType := model.ErrorType("Function.TrailerError")
	errorBody := []byte(`trailer error`)

	trailerError := invoke.NewMockErrorForInvoker(t)
	trailerError.On("ErrorType").Return(errorType)
	trailerError.On("ReturnCode").Return(http.StatusOK)
	trailerError.On("ErrorDetails").Return("trailer error")

	responseBody := "test response body"
	mockRuntimeReq.On("BodyReader").Return(strings.NewReader(responseBody))
	mockRuntimeReq.On("TrailerError").Return(trailerError)

	responder := NewResponder(mockInvokeReq)
	responder.SendRuntimeResponseHeaders(mockInitData, "", "")
	result := responder.SendRuntimeResponseBody(context.Background(), mockRuntimeReq, 0)
	assert.NoError(t, result.Err)
	responder.SendRuntimeResponseTrailers(mockRuntimeReq)

	assert.Equal(t, "Function.TrailerError", recorder.Header().Get("Error-Type"))
	assert.Equal(t, string(errorBody), recorder.Body.String())
	assert.Equal(t, http.StatusOK, recorder.Code)

	mockInvokeReq.AssertExpectations(t)
	mockRuntimeReq.AssertExpectations(t)
}

func TestResponder_SendRuntimeResponseBody(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*interop.MockInvokeRequest, *invoke.MockRuntimeResponseRequest)
		expectedError model.ErrorType
		expectError   bool
	}{
		{
			name: "OversizedResponse",
			setupMocks: func(mockInvokeReq *interop.MockInvokeRequest, mockRuntimeReq *invoke.MockRuntimeResponseRequest) {
				mockInvokeReq.On("MaxPayloadSize").Return(int64(10))
				largeResponse := strings.Repeat("x", 20)
				mockRuntimeReq.On("BodyReader").Return(strings.NewReader(largeResponse))
			},
			expectedError: model.ErrorFunctionOversizedResponse,
			expectError:   true,
		},
		{
			name: "ReadError",
			setupMocks: func(mockInvokeReq *interop.MockInvokeRequest, mockRuntimeReq *invoke.MockRuntimeResponseRequest) {
				mockInvokeReq.On("MaxPayloadSize").Return(int64(1024))
				errorReader := &errorReader{err: io.ErrUnexpectedEOF}
				mockRuntimeReq.On("BodyReader").Return(errorReader)
			},
			expectedError: model.ErrorRuntimeTruncatedResponse,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			recorder := httptest.NewRecorder()
			mockInvokeReq := interop.NewMockInvokeRequest(t)
			mockRuntimeReq := invoke.NewMockRuntimeResponseRequest(t)

			mockInvokeReq.On("ResponseWriter").Return(recorder)
			tt.setupMocks(mockInvokeReq, mockRuntimeReq)

			responder := NewResponder(mockInvokeReq)
			result := responder.SendRuntimeResponseBody(context.Background(), mockRuntimeReq, 0)

			if tt.expectError {
				assert.Error(t, result.Err)
				assert.Equal(t, tt.expectedError, result.Err.ErrorType())
			} else {
				assert.NoError(t, result.Err)
			}

			mockInvokeReq.AssertExpectations(t)
			mockRuntimeReq.AssertExpectations(t)
		})
	}
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}
