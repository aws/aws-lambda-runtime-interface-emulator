// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func TestNewRieInvokeRequest(t *testing.T) {
	tests := []struct {
		name             string
		request          func() *http.Request
		writer           http.ResponseWriter
		want             *rieInvokeRequest
		wantError        bool
		wantErrorContain string
	}{
		{
			name: "no_headers_in_request",
			request: func() *http.Request {
				r, err := http.NewRequest("GET", "http://localhost/", nil)
				require.NoError(t, err)
				return r
			},
			writer: httptest.NewRecorder(),
			want: &rieInvokeRequest{
				contentType:                "application/json",
				maxPayloadSize:             6*1024*1024 + 100,
				responseBandwidthRate:      2 * 1024 * 1024,
				responseBandwidthBurstSize: 6 * 1024 * 1024,
				traceId:                    "",
				cognitoIdentityId:          "",
				cognitoIdentityPoolId:      "",
				clientContext:              "",
			},
			wantError: false,
		},
		{
			name: "all_headers_present_in_request",
			request: func() *http.Request {
				r, err := http.NewRequest("GET", "http://localhost/", nil)
				r.Header.Set("Content-Type", "text/plain")
				r.Header.Set("X-Amzn-Trace-Id", "Root=1-5e1b4151-5ac6c58f3375aa3c7c6b73c9")
				r.Header.Set("X-Amz-Client-Context", "eyJjdXN0b20iOnsidGVzdCI6InZhbHVlIn19")
				r.Header.Set("X-Amzn-RequestId", "test-invoke-id")
				r.Header.Set("X-Amz-Cognito-Identity", `{"cognitoIdentityId":"us-east-1:12345678-1234-1234-1234-123456789012","cognitoIdentityPoolId":"us-east-1:87654321-4321-4321-4321-210987654321"}`)
				require.NoError(t, err)
				return r
			},
			writer: httptest.NewRecorder(),
			want: &rieInvokeRequest{
				invokeID:                   "test-invoke-id",
				contentType:                "text/plain",
				maxPayloadSize:             6*1024*1024 + 100,
				responseBandwidthRate:      2 * 1024 * 1024,
				responseBandwidthBurstSize: 6 * 1024 * 1024,
				traceId:                    "Root=1-5e1b4151-5ac6c58f3375aa3c7c6b73c9",
				cognitoIdentityId:          "us-east-1:12345678-1234-1234-1234-123456789012",
				cognitoIdentityPoolId:      "us-east-1:87654321-4321-4321-4321-210987654321",
				clientContext:              `{"custom":{"test":"value"}}`,
			},
			wantError: false,
		},
		{
			name: "malformed_cognito_identity_header",
			request: func() *http.Request {
				r, err := http.NewRequest("GET", "http://localhost/", nil)
				r.Header.Set("X-Amzn-RequestId", "test-invoke-id")
				r.Header.Set("X-Amz-Cognito-Identity", "not-valid-json{")
				require.NoError(t, err)
				return r
			},
			writer:           httptest.NewRecorder(),
			want:             nil,
			wantError:        true,
			wantErrorContain: "X-Amz-Cognito-Identity must be a valid JSON string",
		},
		{
			name: "malformed_client_context_header",
			request: func() *http.Request {
				r, err := http.NewRequest("GET", "http://localhost/", nil)
				r.Header.Set("X-Amzn-RequestId", "test-invoke-id")
				r.Header.Set("X-Amz-Client-Context", "not-valid-base64!!!")
				require.NoError(t, err)
				return r
			},
			writer:           httptest.NewRecorder(),
			want:             nil,
			wantError:        true,
			wantErrorContain: "X-Amz-Client-Context must be a valid base64 encoded string",
		},
		{
			name: "partial_cognito_identity_header",
			request: func() *http.Request {
				r, err := http.NewRequest("GET", "http://localhost/", nil)
				r.Header.Set("X-Amzn-RequestId", "test-invoke-id")
				r.Header.Set("X-Amz-Cognito-Identity", `{"cognitoIdentityId":"us-east-1:only-id"}`)
				require.NoError(t, err)
				return r
			},
			writer: httptest.NewRecorder(),
			want: &rieInvokeRequest{
				invokeID:                   "test-invoke-id",
				contentType:                "application/json",
				maxPayloadSize:             6*1024*1024 + 100,
				responseBandwidthRate:      2 * 1024 * 1024,
				responseBandwidthBurstSize: 6 * 1024 * 1024,
				traceId:                    "",
				cognitoIdentityId:          "us-east-1:only-id",
				cognitoIdentityPoolId:      "",
				clientContext:              "",
			},
			wantError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.request()
			got, err := NewRieInvokeRequest(r, tt.writer)

			if tt.wantError {
				assert.NotNil(t, err)
				assert.Nil(t, got)
				assert.Equal(t, model.ErrorMalformedRequest, err.ErrorType())
				assert.Equal(t, http.StatusBadRequest, err.ReturnCode())
				assert.Contains(t, err.Error(), tt.wantErrorContain)
				return
			}

			assert.Nil(t, err)
			require.NotNil(t, got)

			tt.want.request = r
			tt.want.writer = tt.writer
			if tt.want.invokeID == "" {
				tt.want.invokeID = got.invokeID
			}
			tt.want.internalInvocationID = got.internalInvocationID

			assert.Equal(t, tt.want, got)
		})
	}
}
