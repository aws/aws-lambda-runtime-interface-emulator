// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/aws-lambda-rie/internal/telemetry/internal"
)

func TestSubscriptionAPI_Subscribe(t *testing.T) {
	tests := []struct {
		name           string
		agentName      string
		body           io.Reader
		expectedStatus int
		expectedError  bool
		setupMocks     func(*mockSubscriptionStore, internal.LogsDroppedEventAPI, *mockTelemetrySubscriptionEventAPI)
		validateResp   func(t *testing.T, resp []byte)
	}{
		{
			name:      "valid_http_subscription_request",
			agentName: "test-agent",
			body: strings.NewReader(`{
				"schemaVersion": "2025-01-29",
				"types": ["platform", "function"],
				"destination": {
					"protocol": "HTTP",
					"URI": "http://sandbox.localdomain:8080/telemetry"
				},
				"buffering": {
					"maxItems": 5000,
					"maxBytes": 262144,
					"timeoutMs": 500
				}
			}`),
			expectedStatus: 200,
			expectedError:  false,
			setupMocks: func(store *mockSubscriptionStore, logsAPI internal.LogsDroppedEventAPI, telemetryAPI *mockTelemetrySubscriptionEventAPI) {
				store.On("addSubscriber", mock.MatchedBy(func(s *internal.Subscriber) bool {
					return s.AgentName() == "test-agent"
				})).Return(nil).Once()
				telemetryAPI.On("sendTelemetrySubscription", "test-agent", "Subscribed", []string{"platform", "function"}).Return(nil).Once()
			},
			validateResp: func(t *testing.T, resp []byte) {
				assert.Equal(t, `"OK"`, string(resp))
			},
		},
		{
			name:      "tcp_connection_refused",
			agentName: "tcp-agent",
			body: strings.NewReader(`{
				"schemaVersion": "2025-01-29",
				"types": ["extension"],
				"destination": {
					"protocol": "TCP",
					"port": 8081
				}
			}`),
			expectedStatus: 400,
			expectedError:  false,
			setupMocks: func(store *mockSubscriptionStore, logsAPI internal.LogsDroppedEventAPI, telemetryAPI *mockTelemetrySubscriptionEventAPI) {

			},
			validateResp: func(t *testing.T, resp []byte) {
				assert.Contains(t, string(resp), "ValidationError")
				assert.Contains(t, string(resp), "Invalid destination")
			},
		},
		{
			name:      "subscription_with_default_buffering",
			agentName: "default-agent",
			body: strings.NewReader(`{
				"schemaVersion": "2025-01-29",
				"types": ["platform"],
				"destination": {
					"protocol": "HTTP",
					"URI": "http://sandbox.localdomain:8080/events"
				}
			}`),
			expectedStatus: 200,
			expectedError:  false,
			setupMocks: func(store *mockSubscriptionStore, logsAPI internal.LogsDroppedEventAPI, telemetryAPI *mockTelemetrySubscriptionEventAPI) {
				store.On("addSubscriber", mock.MatchedBy(func(s *internal.Subscriber) bool {
					return s.AgentName() == "default-agent"

				})).Return(nil).Once()
				telemetryAPI.On("sendTelemetrySubscription", "default-agent", "Subscribed", []string{"platform"}).Return(nil).Once()
			},
			validateResp: func(t *testing.T, resp []byte) {
				assert.Equal(t, `"OK"`, string(resp))
			},
		},
		{
			name:           "invalid_json_request",
			agentName:      "invalid-agent",
			body:           strings.NewReader(`{"invalid": json}`),
			expectedStatus: 400,
			expectedError:  false,
			setupMocks: func(store *mockSubscriptionStore, logsAPI internal.LogsDroppedEventAPI, telemetryAPI *mockTelemetrySubscriptionEventAPI) {

			},
			validateResp: func(t *testing.T, resp []byte) {
				assert.Contains(t, string(resp), "ValidationError")
				assert.Contains(t, string(resp), "failed to parse JSON")
			},
		},
		{
			name:      "schema_validation_error_missing_required_fields",
			agentName: "schema-error-agent",
			body: strings.NewReader(`{
				"schemaVersion": "2025-01-29"
			}`),
			expectedStatus: 400,
			expectedError:  false,
			setupMocks: func(store *mockSubscriptionStore, logsAPI internal.LogsDroppedEventAPI, telemetryAPI *mockTelemetrySubscriptionEventAPI) {

			},
			validateResp: func(t *testing.T, resp []byte) {
				assert.Contains(t, string(resp), "ValidationError")
				assert.Contains(t, string(resp), "schema validation error")
			},
		},
		{
			name:      "invalid_destination_protocol",
			agentName: "invalid-dest-agent",
			body: strings.NewReader(`{
				"schemaVersion": "2025-01-29",
				"types": ["platform"],
				"destination": {
					"protocol": "INVALID",
					"URI": "http://localhost:8080"
				}
			}`),
			expectedStatus: 400,
			expectedError:  false,
			setupMocks: func(store *mockSubscriptionStore, logsAPI internal.LogsDroppedEventAPI, telemetryAPI *mockTelemetrySubscriptionEventAPI) {

			},
			validateResp: func(t *testing.T, resp []byte) {
				assert.Contains(t, string(resp), "ValidationError")
				assert.Contains(t, string(resp), "schema validation error")
			},
		},
		{
			name:      "telemetry_subscription_event_api_error",
			agentName: "event-error-agent",
			body: strings.NewReader(`{
				"schemaVersion": "2025-01-29",
				"types": ["platform"],
				"destination": {
					"protocol": "HTTP",
					"URI": "http://sandbox.localdomain:8080/events"
				}
			}`),
			expectedStatus: 200,
			expectedError:  false,
			setupMocks: func(store *mockSubscriptionStore, logsAPI internal.LogsDroppedEventAPI, telemetryAPI *mockTelemetrySubscriptionEventAPI) {
				store.On("addSubscriber", mock.MatchedBy(func(s *internal.Subscriber) bool {
					return s.AgentName() == "event-error-agent"
				})).Return(nil).Once()
				telemetryAPI.On("sendTelemetrySubscription", "event-error-agent", "Subscribed", []string{"platform"}).Return(fmt.Errorf("event API error")).Once()
			},
			validateResp: func(t *testing.T, resp []byte) {

				assert.Equal(t, `"OK"`, string(resp))
			},
		},
		{
			name:      "partial_buffering_config",
			agentName: "partial-buffer-agent",
			body: strings.NewReader(`{
				"schemaVersion": "2025-01-29",
				"types": ["function"],
				"destination": {
					"protocol": "HTTP",
					"URI": "http://sandbox.localdomain:8080/events"
				},
				"buffering": {
					"maxItems": 2000
				}
			}`),
			expectedStatus: 200,
			expectedError:  false,
			setupMocks: func(store *mockSubscriptionStore, logsAPI internal.LogsDroppedEventAPI, telemetryAPI *mockTelemetrySubscriptionEventAPI) {
				store.On("addSubscriber", mock.MatchedBy(func(s *internal.Subscriber) bool {
					return s.AgentName() == "partial-buffer-agent"

				})).Return(nil).Once()
				telemetryAPI.On("sendTelemetrySubscription", "partial-buffer-agent", "Subscribed", []string{"function"}).Return(nil).Once()
			},
			validateResp: func(t *testing.T, resp []byte) {
				assert.Equal(t, `"OK"`, string(resp))
			},
		},
		{
			name:      "empty_types_array",
			agentName: "empty-types-agent",
			body: strings.NewReader(`{
				"schemaVersion": "2025-01-29",
				"types": [],
				"destination": {
					"protocol": "HTTP",
					"URI": "http://sandbox.localdomain:8080/events"
				}
			}`),
			expectedStatus: 400,
			expectedError:  false,
			setupMocks: func(store *mockSubscriptionStore, logsAPI internal.LogsDroppedEventAPI, telemetryAPI *mockTelemetrySubscriptionEventAPI) {

			},
			validateResp: func(t *testing.T, resp []byte) {
				assert.Contains(t, string(resp), "ValidationError")
				assert.Contains(t, string(resp), "schema validation error")
			},
		},
		{
			name:      "missing_required_field_destination",
			agentName: "missing-dest-agent",
			body: strings.NewReader(`{
				"schemaVersion": "2025-01-29",
				"types": ["platform"]
			}`),
			expectedStatus: 400,
			expectedError:  false,
			setupMocks: func(store *mockSubscriptionStore, logsAPI internal.LogsDroppedEventAPI, telemetryAPI *mockTelemetrySubscriptionEventAPI) {

			},
			validateResp: func(t *testing.T, resp []byte) {
				assert.Contains(t, string(resp), "ValidationError")
				assert.Contains(t, string(resp), "schema validation error")
			},
		},
		{
			name:      "invalid_schema_version",
			agentName: "invalid-version-agent",
			body: strings.NewReader(`{
				"schemaVersion": "invalid-version",
				"types": ["platform"],
				"destination": {
					"protocol": "HTTP",
					"URI": "http://sandbox.localdomain:8080/events"
				}
			}`),
			expectedStatus: 400,
			expectedError:  false,
			setupMocks: func(store *mockSubscriptionStore, logsAPI internal.LogsDroppedEventAPI, telemetryAPI *mockTelemetrySubscriptionEventAPI) {

			},
			validateResp: func(t *testing.T, resp []byte) {
				assert.Contains(t, string(resp), "ValidationError")
				assert.Contains(t, string(resp), "schema validation error")
			},
		},
		{
			name:      "tcp_destination_missing_port",
			agentName: "tcp-missing-port-agent",
			body: strings.NewReader(`{
				"schemaVersion": "2025-01-29",
				"types": ["platform"],
				"destination": {
					"protocol": "TCP"
				}
			}`),
			expectedStatus: 400,
			expectedError:  false,
			setupMocks: func(store *mockSubscriptionStore, logsAPI internal.LogsDroppedEventAPI, telemetryAPI *mockTelemetrySubscriptionEventAPI) {

			},
			validateResp: func(t *testing.T, resp []byte) {
				assert.Contains(t, string(resp), "ValidationError")
				assert.Contains(t, string(resp), "schema validation error")
			},
		},
		{
			name:      "http_destination_missing_uri",
			agentName: "http-missing-uri-agent",
			body: strings.NewReader(`{
				"schemaVersion": "2025-01-29",
				"types": ["platform"],
				"destination": {
					"protocol": "HTTP"
				}
			}`),
			expectedStatus: 400,
			expectedError:  false,
			setupMocks: func(store *mockSubscriptionStore, logsAPI internal.LogsDroppedEventAPI, telemetryAPI *mockTelemetrySubscriptionEventAPI) {

			},
			validateResp: func(t *testing.T, resp []byte) {
				assert.Contains(t, string(resp), "ValidationError")
				assert.Contains(t, string(resp), "schema validation error")
			},
		},
		{
			name:      "relay_add_subscriber_error",
			agentName: "store-error-agent",
			body: strings.NewReader(`{
				"schemaVersion": "2025-01-29",
				"types": ["platform"],
				"destination": {
					"protocol": "HTTP",
					"URI": "http://sandbox.localdomain:8080/events"
				}
			}`),
			expectedStatus: 0,
			expectedError:  true,
			setupMocks: func(store *mockSubscriptionStore, logsAPI internal.LogsDroppedEventAPI, telemetryAPI *mockTelemetrySubscriptionEventAPI) {
				store.On("addSubscriber", mock.MatchedBy(func(s *internal.Subscriber) bool {
					return s.AgentName() == "store-error-agent"
				})).Return(fmt.Errorf("subscription store disabled")).Once()
			},
			validateResp: func(t *testing.T, resp []byte) {

			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMockSubscriptionStore(t)
			defer store.AssertExpectations(t)
			logsDroppedAPI := internal.NewMockLogsDroppedEventAPI(t)
			telemetryAPI := newMockTelemetrySubscriptionEventAPI(t)

			tt.setupMocks(store, logsDroppedAPI, telemetryAPI)

			api := NewSubscriptionAPI(store, logsDroppedAPI, telemetryAPI)

			resp, status, headers, err := api.Subscribe(tt.agentName, tt.body, nil, "")

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, headers)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, headers)
			}

			assert.Equal(t, tt.expectedStatus, status)

			if tt.validateResp != nil {
				tt.validateResp(t, resp)
			}
		})
	}
}

func TestSubscriptionAPI_Subscribe_ReadError(t *testing.T) {
	relay := NewRelay()
	logsDroppedAPI := internal.NewMockLogsDroppedEventAPI(t)
	telemetryAPI := newMockTelemetrySubscriptionEventAPI(t)

	api := NewSubscriptionAPI(relay, logsDroppedAPI, telemetryAPI)

	failingReader := &failingReader{err: fmt.Errorf("read failed")}

	resp, status, headers, err := api.Subscribe("test-agent", failingReader, nil, "")

	assert.NoError(t, err)
	assert.Equal(t, 400, status)
	assert.NotNil(t, headers)
	assert.Contains(t, string(resp), "ValidationError")
	assert.Contains(t, string(resp), "Failed to read request body")
}

type failingReader struct {
	err error
}

func (r *failingReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}
