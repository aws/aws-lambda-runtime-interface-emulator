// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	intmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func TestApp_ServeHTTP(t *testing.T) {
	tests := []struct {
		name              string
		initResponse      rapidmodel.AppError
		invokeErr         rapidmodel.AppError
		responseSent      bool
		expectedStatus    int
		expectedError     bool
		expectedErrorType string
		expectedJSON      string
	}{
		{
			name:           "successful invocation",
			initResponse:   nil,
			invokeErr:      nil,
			responseSent:   false,
			expectedStatus: 200,
			expectedError:  false,
		},
		{
			name:              "init failure",
			initResponse:      rapidmodel.NewCustomerError(rapidmodel.ErrorRuntimeUnknown),
			expectedStatus:    200,
			expectedError:     true,
			expectedErrorType: "Runtime.Unknown",
			expectedJSON:      `{"errorType":"Runtime.Unknown"}`,
		},
		{
			name:              "platform error during init",
			initResponse:      rapidmodel.NewPlatformError(errors.New("platform error"), rapidmodel.ErrorReasonUnknownError),
			expectedStatus:    500,
			expectedError:     true,
			expectedErrorType: "UnknownError",
			expectedJSON:      `{"errorType":"UnknownError"}`,
		},
		{
			name:              "invoke error with response not sent",
			initResponse:      nil,
			invokeErr:         rapidmodel.NewCustomerError(rapidmodel.ErrorRuntimeUnknown),
			responseSent:      false,
			expectedStatus:    200,
			expectedError:     true,
			expectedErrorType: "Runtime.Unknown",
			expectedJSON:      `{"errorType":"Runtime.Unknown"}`,
		},
		{
			name:           "invoke error with response already sent",
			initResponse:   nil,
			invokeErr:      rapidmodel.NewCustomerError(rapidmodel.ErrorRuntimeUnknown),
			responseSent:   true,
			expectedStatus: 200,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApp := newMockRaptorApp(t)
			defer mockApp.AssertExpectations(t)
			mockApp.On("Init", mock.Anything, mock.Anything, mock.Anything).Return(tt.initResponse)
			if tt.initResponse == nil {
				mockApp.On("Invoke", mock.Anything, mock.Anything, mock.Anything).Return(tt.invokeErr, tt.responseSent)
			}

			initMsg := intmodel.InitRequestMessage{
				InitTimeout: intmodel.DurationMS(10 * time.Second),
				Handler:     "test.handler",
			}
			app := NewHTTPHandler(mockApp, initMsg)

			req := httptest.NewRequest("POST", "/2015-03-31/functions/function/invocations", strings.NewReader("{}"))
			w := httptest.NewRecorder()

			app.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
				assert.Equal(t, tt.expectedErrorType, w.Header().Get("Error-Type"))
				assert.JSONEq(t, tt.expectedJSON, w.Body.String())
			} else if tt.responseSent {
				assert.Empty(t, w.Header().Get("Content-Type"))
				assert.Empty(t, w.Header().Get("Error-Type"))
				assert.Empty(t, w.Body.String())
			}
		})
	}
}

func TestApp_ServeHTTP_Concurrent(t *testing.T) {
	mockApp := &mockRaptorApp{}
	defer mockApp.AssertExpectations(t)
	mockApp.On("Init", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			time.Sleep(100 * time.Millisecond)
		}).
		Return(rapidmodel.NewCustomerError(rapidmodel.ErrorRuntimeUnknown)).
		Once()

	initMsg := intmodel.InitRequestMessage{
		InitTimeout: intmodel.DurationMS(10 * time.Second),
		Handler:     "test.handler",
	}
	app := NewHTTPHandler(mockApp, initMsg)

	var wg sync.WaitGroup
	const invokes = 10
	wg.Add(invokes)
	for i := 0; i < invokes; i++ {
		go func() {
			req := httptest.NewRequest("POST", "/2015-03-31/functions/function/invocations", strings.NewReader("{}"))
			w := httptest.NewRecorder()
			app.ServeHTTP(w, req)
			assert.Equal(t, 200, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			assert.Equal(t, "Runtime.Unknown", w.Header().Get("Error-Type"))
			assert.JSONEq(t, `{"errorType":"Runtime.Unknown"}`, w.Body.String())
			wg.Done()
		}()
	}
	wg.Wait()
}
