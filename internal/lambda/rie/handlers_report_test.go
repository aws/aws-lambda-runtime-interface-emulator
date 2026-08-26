// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rie

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Telemetry API defines these metrics as numbers. A consumer decoding into a
// typed struct rejects the strings the memory size arrives as, so the record has
// to carry numbers however the value reached us.
func TestReportRecordMetricsAreNumbers(t *testing.T) {
	encoded, err := json.Marshal(reportRecord("abc", "success", 12.5, "512", 0))
	require.NoError(t, err)

	var decoded struct {
		RequestID string `json:"requestId"`
		Status    string `json:"status"`
		Metrics   struct {
			DurationMs       float64 `json:"durationMs"`
			BilledDurationMs float64 `json:"billedDurationMs"`
			MemorySizeMB     int     `json:"memorySizeMB"`
			MaxMemoryUsedMB  int     `json:"maxMemoryUsedMB"`
		} `json:"metrics"`
	}
	require.NoError(t, json.Unmarshal(encoded, &decoded), "a typed consumer must be able to decode this")

	assert.Equal(t, "abc", decoded.RequestID)
	assert.Equal(t, "success", decoded.Status)
	assert.Equal(t, 12.5, decoded.Metrics.DurationMs)
	assert.Equal(t, float64(13), decoded.Metrics.BilledDurationMs)
	assert.Equal(t, 512, decoded.Metrics.MemorySizeMB)
}

// An invocation that ran out of time is not a successful one. Extensions alarm on
// this field, so reporting a timeout as a success would hide exactly the failure
// they are watching for.
func TestReportRecordCarriesTheInvocationStatus(t *testing.T) {
	for _, status := range []string{"success", "timeout"} {
		t.Run(status, func(t *testing.T) {
			assert.Equal(t, status, reportRecord("abc", status, 1, "128", 0)["status"])
		})
	}
}

// A memory size that isn't a number should not make the record undecodable.
func TestReportRecordSurvivesAnUnparseableMemorySize(t *testing.T) {
	record := reportRecord("abc", "success", 1, "not-a-number", 0)
	metrics := record["metrics"].(map[string]interface{})
	assert.Equal(t, 0, metrics["memorySizeMB"])
}

// A cold start reports how long initialization took; a warm invocation has no
// such figure and must not report a zero one.
func TestReportRecordIncludesInitDurationOnlyOnColdStart(t *testing.T) {
	cold := reportRecord("abc", "success", 1, "128", 120.5)["metrics"].(map[string]interface{})
	assert.Equal(t, 120.5, cold["initDurationMs"])

	warm := reportRecord("abc", "success", 1, "128", 0)["metrics"].(map[string]interface{})
	assert.NotContains(t, warm, "initDurationMs")
}
