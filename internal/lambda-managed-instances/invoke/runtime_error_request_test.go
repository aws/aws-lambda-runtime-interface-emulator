// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func TestNewRuntimeError(t *testing.T) {
	tests := []struct {
		name                 string
		errorTypeHeader      string
		xrayErrorCauseHeader string
		contentTypeHeader    string
		expectedErrorType    model.ErrorType
		expectedContentType  string
		expectedXrayJSON     string
	}{
		{
			name:                 "validRuntimeErrorWithValidXrayCause",
			errorTypeHeader:      "Runtime.CustomError",
			xrayErrorCauseHeader: `{"message":"test error"}`,
			contentTypeHeader:    "application/json",
			expectedErrorType:    "Runtime.CustomError",
			expectedContentType:  "application/json",
			expectedXrayJSON:     `{"exceptions":null,"message":"test error","paths":null,"working_directory":""}`,
		},
		{
			name:                 "invalidErrorTypeDefaultsToRuntimeUnknown",
			errorTypeHeader:      "InvalidType",
			xrayErrorCauseHeader: `{"paths":["index.js"]}`,
			contentTypeHeader:    "text/plain",
			expectedErrorType:    "Runtime.Unknown",
			expectedContentType:  "text/plain",
			expectedXrayJSON:     `{"exceptions":null,"paths":["index.js"],"working_directory":""}`,
		},
		{
			name:                 "emptyErrorTypeHeader",
			errorTypeHeader:      "",
			xrayErrorCauseHeader: `{"working_directory":"/app"}`,
			contentTypeHeader:    "",
			expectedErrorType:    "Runtime.Unknown",
			expectedContentType:  "",
			expectedXrayJSON:     `{"exceptions":null,"paths":null,"working_directory":"/app"}`,
		},
		{
			name:                 "invalidXrayErrorCauseReturnsNil",
			errorTypeHeader:      "Function.CustomError",
			xrayErrorCauseHeader: `{invalid json}`,
			contentTypeHeader:    "application/json",
			expectedErrorType:    "Function.CustomError",
			expectedContentType:  "application/json",
			expectedXrayJSON:     "",
		},
		{
			name:                 "emptyXrayErrorCauseHeader",
			errorTypeHeader:      "Runtime.ImportModuleError",
			xrayErrorCauseHeader: "",
			contentTypeHeader:    "application/octet-stream",
			expectedErrorType:    "Runtime.ImportModuleError",
			expectedContentType:  "application/octet-stream",
			expectedXrayJSON:     "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			req, err := http.NewRequest("POST", "/runtime/invocation/test-id/error", strings.NewReader("error details"))
			require.NoError(t, err)

			req.Header.Set(RuntimeErrorTypeHeader, tc.errorTypeHeader)
			req.Header.Set(LambdaXRayErrorCauseHeader, tc.xrayErrorCauseHeader)
			req.Header.Set(RuntimeContentTypeHeader, tc.contentTypeHeader)

			ctx := context.Background()
			invokeID := interop.InvokeID("test-invoke-id")

			runtimeErr := NewRuntimeError(ctx, req, invokeID, "error details")

			assert.Equal(t, tc.expectedErrorType, runtimeErr.ErrorType())
			assert.Equal(t, tc.expectedContentType, runtimeErr.ContentType())
			assert.Equal(t, model.ErrorCategory(RuntimeErrorCategory), runtimeErr.ErrorCategory())
			assert.Equal(t, "error details", runtimeErr.ErrorDetails())
			assert.Equal(t, invokeID, runtimeErr.InvokeID())

			xrayErrorCause := runtimeErr.GetXrayErrorCause()
			if tc.expectedXrayJSON == "" {
				assert.Nil(t, xrayErrorCause)
			} else {
				assert.NotNil(t, xrayErrorCause)
				assert.JSONEq(t, tc.expectedXrayJSON, string(xrayErrorCause))
			}
		})
	}
}

func TestGetValidatedErrorCause(t *testing.T) {
	tests := []struct {
		name             string
		errorCauseHeader string
		expectedJSON     string
	}{
		{
			name:             "emptyHeaderReturnsNil",
			errorCauseHeader: "",
			expectedJSON:     "",
		},
		{
			name:             "validErrorCauseWithMessage",
			errorCauseHeader: `{"message":"test error"}`,
			expectedJSON:     `{"exceptions":null,"message":"test error","paths":null,"working_directory":""}`,
		},
		{
			name:             "invalidJSONReturnsNil",
			errorCauseHeader: `{invalid json}`,
			expectedJSON:     "",
		},
		{
			name:             "invalidErrorCauseFormatReturnsNil",
			errorCauseHeader: `{}`,
			expectedJSON:     "",
		},
		{
			name:             "validErrorCauseWithPaths",
			errorCauseHeader: `{"paths":["test.js"]}`,
			expectedJSON:     `{"exceptions":null,"paths":["test.js"],"working_directory":""}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			result := getValidatedErrorCause(ctx, tc.errorCauseHeader)

			if tc.expectedJSON == "" {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.JSONEq(t, tc.expectedJSON, string(result))
			}
		})
	}
}
