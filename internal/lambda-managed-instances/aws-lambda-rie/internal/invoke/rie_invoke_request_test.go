// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRieInvokeRequest(t *testing.T) {
	tests := []struct {
		name    string
		request func() *http.Request
		writer  http.ResponseWriter
		want    *rieInvokeRequest
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
		},
		{
			name: "all_headers_present_in_request",
			request: func() *http.Request {
				r, err := http.NewRequest("GET", "http://localhost/", nil)
				r.Header.Set("Content-Type", "text/plain")
				r.Header.Set("X-Amzn-Trace-Id", "Root=1-5e1b4151-5ac6c58f3375aa3c7c6b73c9")
				r.Header.Set("X-Amz-Client-Context", "eyJjdXN0b20iOnsidGVzdCI6InZhbHVlIn19")
				r.Header.Set("X-Amzn-RequestId", "test-invoke-id")
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
				cognitoIdentityId:          "",
				cognitoIdentityPoolId:      "",
				clientContext:              "eyJjdXN0b20iOnsidGVzdCI6InZhbHVlIn19",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.request()
			got := NewRieInvokeRequest(r, tt.writer)

			tt.want.request = r
			tt.want.writer = tt.writer
			if tt.want.invokeID == "" {
				tt.want.invokeID = got.invokeID
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
