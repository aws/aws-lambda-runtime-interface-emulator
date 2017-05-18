// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"io"

	"go.amzn.com/lambda/interop"
)

// LogsAPIService represents interface that implementations of Telemetry API have to satisfy to be RAPID-compatible
type LogsAPIService interface {
	Subscribe(agentName string, body io.Reader, headers map[string][]string) (resp []byte, status int, respHeaders map[string][]string, err error)
	RecordCounterMetric(metricName string, count int)
	FlushMetrics() interop.LogsAPIMetrics
	Clear()
	TurnOff()
}
