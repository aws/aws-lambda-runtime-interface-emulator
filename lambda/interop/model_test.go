// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeSubscriptionMetrics(t *testing.T) {
	logsAPIMetrics := map[string]int{
		"server_error": 1,
		"client_error": 2,
	}

	telemetryAPIMetrics := map[string]int{
		"server_error": 1,
		"success":      5,
	}

	metrics := MergeSubscriptionMetrics(logsAPIMetrics, telemetryAPIMetrics)
	assert.Equal(t, 5, metrics["success"])
	assert.Equal(t, 2, metrics["server_error"])
	assert.Equal(t, 2, metrics["client_error"])
}
