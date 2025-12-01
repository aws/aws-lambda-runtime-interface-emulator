// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
)

func TestBatch(t *testing.T) {
	tests := []struct {
		name         string
		bufCfg       BufferingConfig
		events       []string
		expectedFull []bool
		description  string
	}{
		{
			name: "sequential_event_addition_until_full",
			bufCfg: BufferingConfig{
				MaxItems: 3,
				MaxBytes: 1000,
				Timeout:  model.DurationMS(1 * time.Second),
			},
			events: []string{
				`{"event":1}`,
				`{"event":2}`,
				`{"event":3}`,
				`{"event":4}`,
			},
			expectedFull: []bool{false, false, true, true},
			description:  "Sequential addition should properly track fullness",
		},
		{
			name: "mixed_size_events_reach_byte_limit",
			bufCfg: BufferingConfig{
				MaxItems: 10,
				MaxBytes: 50,
				Timeout:  model.DurationMS(1 * time.Second),
			},
			events: []string{
				`{"a":1}`,
				`{"data":"test"}`,
				`{"largeData":"this is a long string"}`,
			},
			expectedFull: []bool{false, false, true},
			description:  "Mixed size events should trigger byte limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batch := newBatch(tt.bufCfg)
			var totalSize int
			for i, eventJSON := range tt.events {
				totalSize += len(eventJSON)
				expectedFull := tt.expectedFull[i]
				assert.Equal(t, expectedFull, batch.addEvent(json.RawMessage(eventJSON)), "Event %d: addEvent return value should match expected fullness", i+1)
				assert.Equal(t, expectedFull, batch.isFull(), "Event %d: addEvent return value should match isFull()", i+1)
			}
			assert.Equal(t, len(tt.events), len(batch.events), "Batch should contain events")
			assert.Equal(t, totalSize, batch.sizeBytes, "Batch sizeBytes should match sum of event lengths")
		})
	}
}
