// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"io"
	"net/http"

	"go.amzn.com/lambda/interop"
)

// LogsSubscriptionAPI represents interface that implementations of Telemetry API have to satisfy to be RAPID-compatible
type LogsSubscriptionAPI interface {
	Subscribe(agentName string, body io.Reader, headers map[string][]string) (resp []byte, status int, respHeaders map[string][]string, err error)
	RecordCounterMetric(metricName string, count int)
	FlushMetrics() interop.LogsAPIMetrics
	Clear()
	TurnOff()
}

type NoOpLogsSubscriptionAPI struct{}

// Subscribe writes response to a shared memory
func (m *NoOpLogsSubscriptionAPI) Subscribe(agentName string, body io.Reader, headers map[string][]string) ([]byte, int, map[string][]string, error) {
	return []byte(`{}`), http.StatusOK, map[string][]string{}, nil
}

func (m *NoOpLogsSubscriptionAPI) RecordCounterMetric(metricName string, count int) {}

func (m *NoOpLogsSubscriptionAPI) FlushMetrics() interop.LogsAPIMetrics {
	return interop.LogsAPIMetrics(map[string]int{})
}

func (m *NoOpLogsSubscriptionAPI) Clear() {}

func (m *NoOpLogsSubscriptionAPI) TurnOff() {}
