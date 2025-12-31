// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		destination SubscriptionDestination
		setupServer func() net.Listener
		expectError bool
		errorMsg    string
	}{

		{
			name: "creates_tcp_client_successfully",
			destination: SubscriptionDestination{
				Protocol: protocolTCP,
				Port:     0,
			},
			setupServer: func() net.Listener {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				return listener
			},
			expectError: false,
		},
		{
			name: "tcp_client_fails_no_server",
			destination: SubscriptionDestination{
				Protocol: protocolTCP,
				Port:     65432,
			},
			expectError: true,
			errorMsg:    "could not TCP dial",
		},
		{
			name: "tcp_client_with_zero_port",
			destination: SubscriptionDestination{
				Protocol: protocolTCP,
				Port:     0,
			},
			expectError: true,
			errorMsg:    "could not TCP dial",
		},

		{
			name: "creates_http_client_valid_sandbox_domain",
			destination: SubscriptionDestination{
				Protocol: protocolHTTP,
				URI:      "http://sandbox.localdomain:8080/telemetry",
			},
			expectError: false,
		},

		{
			name: "http_client_with_https",
			destination: SubscriptionDestination{
				Protocol: protocolHTTP,
				URI:      "https://sandbox.localdomain:9090/events",
			},
			expectError: false,
		},
		{
			name: "http_client_missing_port",
			destination: SubscriptionDestination{
				Protocol: protocolHTTP,
				URI:      "http://sandbox.localdomain/path",
			},
			expectError: false,
		},
		{
			name: "http_client_with_path_and_query",
			destination: SubscriptionDestination{
				Protocol: protocolHTTP,
				URI:      "http://sandbox.localdomain:8080/telemetry?param=value",
			},
			expectError: false,
		},
		{
			name: "http_client_invalid_uri_format",
			destination: SubscriptionDestination{
				Protocol: protocolHTTP,
				URI:      "not-a-valid-uri",
			},
			expectError: true,
			errorMsg:    "destination.URI host must be sandbox.localdomain",
		},
		{
			name: "http_client_invalid_hostname",
			destination: SubscriptionDestination{
				Protocol: protocolHTTP,
				URI:      "http://invalid.domain:8080/telemetry",
			},
			expectError: true,
			errorMsg:    "destination.URI host must be sandbox.localdomain",
		},

		{
			name: "invalid_protocol",
			destination: SubscriptionDestination{
				Protocol: "INVALID",
				URI:      "http://localhost:8080",
			},
			expectError: true,
			errorMsg:    "unknown protocol: INVALID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupServer != nil {
				listener := tt.setupServer()
				defer func() { require.NoError(t, listener.Close()) }()

				if tt.destination.Protocol == protocolTCP && tt.destination.Port == 0 {
					_, portStr, err := net.SplitHostPort(listener.Addr().String())
					require.NoError(t, err)
					port, err := strconv.Atoi(portStr)
					require.NoError(t, err)
					tt.destination.Port = uint16(port)
				}
			}

			_, err := NewClient(tt.destination)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPClient_send(t *testing.T) {
	tests := []struct {
		name         string
		setupServer  func() *httptest.Server
		batch        batch
		setupContext func() context.Context
		expectError  bool
		errorMsg     string
		validateReq  func(t *testing.T, body []byte)
	}{
		{
			name: "sends_batch_successfully",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, http.MethodPost, r.Method)
					w.WriteHeader(http.StatusOK)
					_, err := w.Write([]byte("OK"))
					assert.NoError(t, err)
				}))
			},
			batch: batch{
				events: []json.RawMessage{
					json.RawMessage(`{"type":"platform.start","timestamp":"2023-01-01T00:00:00Z"}`),
					json.RawMessage(`{"type":"platform.report","timestamp":"2023-01-01T00:01:00Z"}`),
				},
			},
			setupContext: context.Background,
			expectError:  false,
			validateReq: func(t *testing.T, body []byte) {
				var events []json.RawMessage
				err := json.Unmarshal(body, &events)
				assert.NoError(t, err)
				assert.Len(t, events, 2)
			},
		},
		{
			name: "handles_http_error_response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					_, err := w.Write([]byte("Bad Request"))
					assert.NoError(t, err)
				}))
			},
			batch: batch{
				events: []json.RawMessage{
					json.RawMessage(`{"type":"test"}`),
				},
			},
			setupContext: context.Background,
			expectError:  true,
			errorMsg:     "http request failed with status 400 Bad Request",
		},
		{
			name: "handles_context_cancellation",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(200 * time.Millisecond)
					w.WriteHeader(http.StatusOK)
				}))
			},
			batch: batch{
				events: []json.RawMessage{
					json.RawMessage(`{"type":"test"}`),
				},
			},
			setupContext: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
				return ctx
			},
			expectError: true,
			errorMsg:    "context canceled",
		},
		{
			name: "sends_empty_batch",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, err := w.Write([]byte("OK"))
					assert.NoError(t, err)
				}))
			},
			batch:        batch{events: []json.RawMessage{}},
			setupContext: context.Background,
			expectError:  false,
			validateReq: func(t *testing.T, body []byte) {
				var events []json.RawMessage
				err := json.Unmarshal(body, &events)
				assert.NoError(t, err)
				assert.Len(t, events, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			client := &httpClient{addr: server.URL}
			err := client.send(tt.setupContext(), tt.batch)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
