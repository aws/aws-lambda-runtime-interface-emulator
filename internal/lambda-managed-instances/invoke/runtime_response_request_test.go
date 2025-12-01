// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func TestRuntimeResponse_TrailerError(t *testing.T) {
	tests := []struct {
		name              string
		errorTypeTrailer  string
		errorBodyTrailer  string
		expectedErrorType model.ErrorType
		expectedErrorBody string
	}{
		{
			name:              "empty error type returns empty values",
			errorTypeTrailer:  "",
			errorBodyTrailer:  "dGVzdA==",
			expectedErrorType: "",
			expectedErrorBody: "",
		},
		{
			name:              "valid runtime error with valid base64 body",
			errorTypeTrailer:  "Runtime.CustomError",
			errorBodyTrailer:  "ZXhpdCBjb2RlIDE=",
			expectedErrorType: "Runtime.CustomError",
			expectedErrorBody: "exit code 1",
		},
		{
			name:              "valid function error with empty base64 body",
			errorTypeTrailer:  "Function.CustomError",
			errorBodyTrailer:  "",
			expectedErrorType: "Function.CustomError",
			expectedErrorBody: "",
		},
		{
			name:              "invalid error type with valid base64 body",
			errorTypeTrailer:  "InvalidType",
			errorBodyTrailer:  "c29tZSBlcnJvcg==",
			expectedErrorType: "Runtime.Unknown",
			expectedErrorBody: "some error",
		},
		{
			name:              "valid error type with invalid base64 body",
			errorTypeTrailer:  "Function.CustomError",
			errorBodyTrailer:  "invalid-base64!@#",
			expectedErrorType: "Function.CustomError",
			expectedErrorBody: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			req, err := http.NewRequest("POST", "/test", nil)
			require.NoError(t, err)
			req.Trailer = make(http.Header)
			req.Trailer.Set(FunctionErrorTypeTrailer, tc.errorTypeTrailer)
			req.Trailer.Set(FunctionErrorBodyTrailer, tc.errorBodyTrailer)
			resp := NewRuntimeResponse(req.Context(), req, "test-invoke-id")

			actualTrailerError := resp.TrailerError()

			if tc.expectedErrorType == "" {
				assert.Nil(t, actualTrailerError)
			} else {
				assert.Equal(t, tc.expectedErrorType, actualTrailerError.ErrorType())
				assert.Equal(t, tc.expectedErrorBody, actualTrailerError.ErrorDetails())
			}
		})
	}
}
