// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"io"
	"net/http"

	"go.amzn.com/lambda/interop"
)

// SubscriptionAPI represents interface that implementations of Telemetry API have to satisfy to be RAPID-compatible
type SubscriptionAPI interface {
	Subscribe(agentName string, body io.Reader, headers map[string][]string, remoteAddr string) (resp []byte, status int, respHeaders map[string][]string, err error)
	RecordCounterMetric(metricName string, count int)
	FlushMetrics() interop.TelemetrySubscriptionMetrics
	Clear()
	TurnOff()
	GetEndpointURL() string
	GetServiceClosedErrorMessage() string
	GetServiceClosedErrorType() string
}

type NoOpSubscriptionAPI struct{}

// Subscribe writes response to a shared memory
func (m *NoOpSubscriptionAPI) Subscribe(agentName string, body io.Reader, headers map[string][]string, remoteAddr string) ([]byte, int, map[string][]string, error) {
	return []byte(`{}`), http.StatusOK, map[string][]string{}, nil
}

func (m *NoOpSubscriptionAPI) RecordCounterMetric(metricName string, count int) {}

func (m *NoOpSubscriptionAPI) FlushMetrics() interop.TelemetrySubscriptionMetrics {
	return interop.TelemetrySubscriptionMetrics(map[string]int{})
}

func (m *NoOpSubscriptionAPI) Clear() {}

func (m *NoOpSubscriptionAPI) TurnOff() {}

func (m *NoOpSubscriptionAPI) GetEndpointURL() string { return "" }

func (m *NoOpSubscriptionAPI) GetServiceClosedErrorMessage() string { return "" }

func (m *NoOpSubscriptionAPI) GetServiceClosedErrorType() string { return "" }
