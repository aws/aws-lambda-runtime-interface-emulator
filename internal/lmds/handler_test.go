// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package lmds

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testToken = "test-secret-token-12345"
	testAZID  = "use1-az1"
)

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func TestServeHTTP(t *testing.T) {
	tests := []struct {
		name            string
		initialMetadata []byte
		initialMaxAge   time.Duration
		updatedMetadata []byte
		updatedMaxAge   time.Duration
		method          string
		authToken       string
		expectedStatus  int
		expectedJSON    string
		updatedJSON     string
		expectedMetrics Metrics
	}{
		{
			name:            "200/ok/default-cache",
			initialMetadata: mustMarshal(&Metadata{AvailabilityZoneID: "use1-az1"}),
			initialMaxAge:   12 * time.Hour,
			updatedMetadata: mustMarshal(&Metadata{AvailabilityZoneID: "use1-az2"}),
			updatedMaxAge:   12 * time.Hour,
			method:          http.MethodGet,
			authToken:       "Bearer " + testToken,
			expectedStatus:  http.StatusOK,
			expectedJSON:    `{"AvailabilityZoneID":"use1-az1"}`,
			updatedJSON:     `{"AvailabilityZoneID":"use1-az2"}`,
			expectedMetrics: Metrics{
				ClientErrors:    0,
				SuccessfulCalls: 1,
			},
		},
		{
			name:            "200/ok/snapstart-cache",
			initialMetadata: mustMarshal(&Metadata{AvailabilityZoneID: "use1-az1"}),
			initialMaxAge:   1 * time.Second,
			updatedMetadata: mustMarshal(&Metadata{AvailabilityZoneID: "use1-az2"}),
			updatedMaxAge:   12 * time.Hour,
			method:          http.MethodGet,
			authToken:       "Bearer " + testToken,
			expectedStatus:  http.StatusOK,
			expectedJSON:    `{"AvailabilityZoneID":"use1-az1"}`,
			updatedJSON:     `{"AvailabilityZoneID":"use1-az2"}`,
			expectedMetrics: Metrics{
				ClientErrors:    0,
				SuccessfulCalls: 1,
			},
		},
		{
			name:            "401/unauthorized/wrongToken",
			initialMetadata: mustMarshal(&Metadata{AvailabilityZoneID: testAZID}),
			updatedMetadata: mustMarshal(&Metadata{AvailabilityZoneID: "use1-az3"}),
			method:          http.MethodGet,
			authToken:       "Bearer wrong-token",
			expectedStatus:  http.StatusUnauthorized,
			expectedMetrics: Metrics{
				ClientErrors:    1,
				SuccessfulCalls: 0,
			},
		},
		{
			name:            "401/unauthorized/noBearerPrefix",
			initialMetadata: mustMarshal(&Metadata{AvailabilityZoneID: testAZID}),
			updatedMetadata: mustMarshal(&Metadata{AvailabilityZoneID: "use1-az3"}),
			method:          http.MethodGet,
			authToken:       testToken,
			expectedStatus:  http.StatusUnauthorized,
			expectedMetrics: Metrics{
				ClientErrors:    1,
				SuccessfulCalls: 0,
			},
		},
		{
			name:            "401/unauthorized/emptyHeader",
			initialMetadata: mustMarshal(&Metadata{AvailabilityZoneID: testAZID}),
			updatedMetadata: mustMarshal(&Metadata{AvailabilityZoneID: "use1-az3"}),
			method:          http.MethodGet,
			authToken:       "",
			expectedStatus:  http.StatusUnauthorized,
			expectedMetrics: Metrics{
				ClientErrors:    1,
				SuccessfulCalls: 0,
			},
		},
		{
			name:            "401/unauthorized/bearerOnly",
			initialMetadata: mustMarshal(&Metadata{AvailabilityZoneID: testAZID}),
			updatedMetadata: mustMarshal(&Metadata{AvailabilityZoneID: "use1-az3"}),
			method:          http.MethodGet,
			authToken:       "Bearer",
			expectedStatus:  http.StatusUnauthorized,
			expectedMetrics: Metrics{
				ClientErrors:    1,
				SuccessfulCalls: 0,
			},
		},
		{
			name:            "401/unauthorized/bearerWithSpace",
			initialMetadata: mustMarshal(&Metadata{AvailabilityZoneID: testAZID}),
			updatedMetadata: mustMarshal(&Metadata{AvailabilityZoneID: "use1-az3"}),
			method:          http.MethodGet,
			authToken:       "Bearer ",
			expectedStatus:  http.StatusUnauthorized,
			expectedMetrics: Metrics{
				ClientErrors:    1,
				SuccessfulCalls: 0,
			},
		},
		{
			name:            "401/unauthorized/wrongScheme",
			initialMetadata: mustMarshal(&Metadata{AvailabilityZoneID: testAZID}),
			updatedMetadata: mustMarshal(&Metadata{AvailabilityZoneID: "use1-az3"}),
			method:          http.MethodGet,
			authToken:       "Basic " + testToken,
			expectedStatus:  http.StatusUnauthorized,
			expectedMetrics: Metrics{
				ClientErrors:    1,
				SuccessfulCalls: 0,
			},
		},
		{
			name:            "405/methodNotAllowed",
			initialMetadata: mustMarshal(&Metadata{AvailabilityZoneID: testAZID}),
			updatedMetadata: mustMarshal(&Metadata{AvailabilityZoneID: "use1-az3"}),
			method:          http.MethodPost,
			authToken:       "Bearer " + testToken,
			expectedStatus:  http.StatusMethodNotAllowed,
			expectedMetrics: Metrics{
				ClientErrors:    1,
				SuccessfulCalls: 0,
			},
		},
		{
			name:           "503/serviceUnavailable/uninitializedService",
			method:         http.MethodGet,
			authToken:      "Bearer " + testToken,
			expectedStatus: http.StatusServiceUnavailable,
			expectedMetrics: Metrics{
				ServerErrors:    1,
				ClientErrors:    0,
				SuccessfulCalls: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if tt.initialMaxAge == 0 {
				tt.initialMaxAge = 12 * time.Hour
			}
			if tt.updatedMaxAge == 0 {
				tt.updatedMaxAge = 12 * time.Hour
			}

			service := NewService(testToken)

			if tt.initialMetadata != nil {
				service.UpdateMetadata(MetadataConfig{
					Data:   tt.initialMetadata,
					MaxAge: tt.initialMaxAge,
				})
			}

			server := httptest.NewServer(service)
			defer server.Close()

			req, err := http.NewRequest(tt.method, server.URL, nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", tt.authToken)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			if resp.StatusCode == http.StatusOK {
				assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
				expectedCacheControl := fmt.Sprintf("private, max-age=%d, immutable", int(tt.initialMaxAge.Seconds()))
				assert.Equal(t, expectedCacheControl, resp.Header.Get("Cache-Control"))
			}

			if tt.expectedJSON != "" {
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				assert.JSONEq(t, tt.expectedJSON, string(body))
			}

			assert.Equal(t, tt.expectedMetrics, service.Metrics.Take(), "first request: metrics mismatch")

			if tt.updatedMetadata != nil {
				service.UpdateMetadata(MetadataConfig{
					Data:   tt.updatedMetadata,
					MaxAge: tt.updatedMaxAge,
				})
			}

			req2, err := http.NewRequest(tt.method, server.URL, nil)
			require.NoError(t, err)
			req2.Header.Set("Authorization", tt.authToken)

			resp2, err := http.DefaultClient.Do(req2)
			require.NoError(t, err)
			defer resp2.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp2.StatusCode)

			if resp2.StatusCode == http.StatusOK {
				assert.Equal(t, "application/json", resp2.Header.Get("Content-Type"))
				expectedCacheControl := fmt.Sprintf("private, max-age=%d, immutable", int(tt.updatedMaxAge.Seconds()))
				assert.Equal(t, expectedCacheControl, resp2.Header.Get("Cache-Control"))
			}

			if tt.updatedJSON != "" {
				body2, err := io.ReadAll(resp2.Body)
				require.NoError(t, err)
				assert.JSONEq(t, tt.updatedJSON, string(body2))
			}

			assert.Equal(t, tt.expectedMetrics, service.Metrics.Take(), "second request: metrics mismatch")

			assert.Equal(t, Metrics{}, service.Metrics.Take(), "after reset: metrics should be zero")
		})
	}
}
