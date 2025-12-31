// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/aws-lambda-rie/internal/telemetry/internal"
)

func TestLogsEgress(t *testing.T) {
	tests := []struct {
		name             string
		socketType       string
		expectedCategory internal.EventCategory
	}{
		{
			name:             "extension_sockets_use_extension_category",
			socketType:       "extension",
			expectedCategory: internal.CategoryExtension,
		},
		{
			name:             "runtime_sockets_use_function_category",
			socketType:       "runtime",
			expectedCategory: internal.CategoryFunction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relay := newMockRelay(t)
			defer relay.AssertExpectations(t)

			egress := NewLogsEgress(relay, io.Discard)

			var stdout, stderr io.Writer
			var err error

			switch tt.socketType {
			case "extension":
				stdout, stderr, err = egress.GetExtensionSockets()
			case "runtime":
				stdout, stderr, err = egress.GetRuntimeSockets()
			}
			assert.NoError(t, err)
			require.NotNil(t, stdout)
			require.NotNil(t, stderr)

			line := []byte("test\n")
			relay.On("broadcast", "test", tt.expectedCategory, tt.expectedCategory).Twice()
			n, err := stdout.Write(line)
			assert.NoError(t, err)
			assert.Len(t, line, n)
			n, err = stderr.Write(line)
			assert.NoError(t, err)
			assert.Len(t, line, n)

			assert.Eventually(t, func() bool {
				return relay.AssertNumberOfCalls(t, "broadcast", 2)
			}, 1*time.Second, 10*time.Millisecond)
		})
	}
}
