// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/metering"
)

func TestGetRuntimeDoneInvokeMetrics(t *testing.T) {
	now := metering.Monotime()

	runtimeStartedTime := now
	invokeResponseMetrics := &interop.InvokeResponseMetrics{
		ProducedBytes:         int64(100),
		RuntimeCalledResponse: true,
	}
	runtimeDoneTime := now + int64(time.Millisecond*time.Duration(10))

	expected := &interop.RuntimeDoneInvokeMetrics{
		ProducedBytes: int64(100),
		DurationMs:    float64(10),
	}

	assert.Equal(t, expected, GetRuntimeDoneInvokeMetrics(runtimeStartedTime, invokeResponseMetrics, runtimeDoneTime))
}

func TestGetRuntimeDoneInvokeMetricsWhenRuntimeCalledError(t *testing.T) {
	now := metering.Monotime()

	runtimeStartedTime := now
	invokeResponseMetrics := &interop.InvokeResponseMetrics{
		ProducedBytes:         int64(100),
		RuntimeCalledResponse: false,
	}
	// validating microsecond precision
	runtimeDoneTime := now + int64(time.Duration(10)*time.Millisecond+time.Duration(50)*time.Microsecond)

	expected := &interop.RuntimeDoneInvokeMetrics{
		ProducedBytes: int64(0),
		DurationMs:    float64(10.05),
	}

	assert.Equal(t, expected, GetRuntimeDoneInvokeMetrics(runtimeStartedTime, invokeResponseMetrics, runtimeDoneTime))
}

func TestGetRuntimeDoneInvokeMetricsWhenRuntimeStartedTimeIsMinusOne(t *testing.T) {
	now := int64(-1)
	runtimeStartedTime := now

	runtimeDoneTime := now + int64(time.Millisecond*time.Duration(10))

	expected := &interop.RuntimeDoneInvokeMetrics{
		ProducedBytes: int64(0),
		DurationMs:    float64(0),
	}
	actual := GetRuntimeDoneInvokeMetrics(runtimeStartedTime, nil, runtimeDoneTime)
	assert.Equal(t, expected, actual)
}

func TestGetRuntimeDoneInvokeMetricsWhenInvokeResponseMetricsIsNil(t *testing.T) {
	now := metering.Monotime()
	runtimeStartedTime := now

	runtimeDoneTime := now + int64(time.Millisecond*time.Duration(10))

	expected := &interop.RuntimeDoneInvokeMetrics{
		ProducedBytes: int64(0),
		DurationMs:    float64(10),
	}

	assert.Equal(t, expected, GetRuntimeDoneInvokeMetrics(runtimeStartedTime, nil, runtimeDoneTime))
}

func TestGetRuntimeDoneSpans(t *testing.T) {
	now := metering.Monotime()
	startReadingResponseMonoTimeMs := now + int64(time.Millisecond*time.Duration(5))
	finishReadingResponseMonoTimeMs := now + int64(time.Millisecond*time.Duration(7))

	runtimeStartedTime := now
	invokeResponseMetrics := &interop.InvokeResponseMetrics{
		StartReadingResponseMonoTimeMs:  startReadingResponseMonoTimeMs,
		FinishReadingResponseMonoTimeMs: finishReadingResponseMonoTimeMs,
		RuntimeCalledResponse:           true,
	}

	expectedResponseLatencyMsStartTime := GetEpochTimeInISO8601FormatFromMonotime(now)
	expectedResponseDurationMsStartTime := GetEpochTimeInISO8601FormatFromMonotime(startReadingResponseMonoTimeMs)
	expected := []interop.Span{
		{
			Name:       "responseLatency",
			Start:      expectedResponseLatencyMsStartTime,
			DurationMs: 5,
		},
		{
			Name:       "responseDuration",
			Start:      expectedResponseDurationMsStartTime,
			DurationMs: 2,
		},
	}

	assert.Equal(t, expected, GetRuntimeDoneSpans(runtimeStartedTime, invokeResponseMetrics))
}

func TestGetRuntimeDoneSpansWhenRuntimeCalledError(t *testing.T) {
	now := metering.Monotime()
	startReadingResponseMonoTimeMs := now + int64(time.Millisecond*time.Duration(5))
	finishReadingResponseMonoTimeMs := now + int64(time.Millisecond*time.Duration(7))

	runtimeStartedTime := now
	invokeResponseMetrics := &interop.InvokeResponseMetrics{
		StartReadingResponseMonoTimeMs:  startReadingResponseMonoTimeMs,
		FinishReadingResponseMonoTimeMs: finishReadingResponseMonoTimeMs,
		RuntimeCalledResponse:           false,
	}

	assert.Equal(t, []interop.Span{}, GetRuntimeDoneSpans(runtimeStartedTime, invokeResponseMetrics))
}

func TestGetRuntimeDoneSpansWhenInvokeResponseMetricsNil(t *testing.T) {
	runtimeStartedTime := metering.Monotime()

	assert.Equal(t, []interop.Span{}, GetRuntimeDoneSpans(runtimeStartedTime, nil))
}

func TestGetRuntimeDoneSpansWhenRuntimeStartedTimeIsMinusOne(t *testing.T) {
	now := int64(-1)
	runtimeStartedTime := now
	invokeResponseMetrics := &interop.InvokeResponseMetrics{
		StartReadingResponseMonoTimeMs:  now + int64(time.Millisecond*time.Duration(5)),
		FinishReadingResponseMonoTimeMs: now + int64(time.Millisecond*time.Duration(7)),
	}

	assert.Equal(t, []interop.Span{}, GetRuntimeDoneSpans(runtimeStartedTime, invokeResponseMetrics))
}

func TestInferInitType(t *testing.T) {
	testCases := map[string]struct {
		initCachingEnabled bool
		sandboxType        interop.SandboxType
		expected           interop.InitType
	}{
		"on demand": {
			initCachingEnabled: false,
			sandboxType:        interop.SandboxClassic,
			expected:           InitTypeOnDemand,
		},
		"pc": {
			initCachingEnabled: false,
			sandboxType:        interop.SandboxPreWarmed,
			expected:           InitTypeProvisionedConcurrency,
		},
		"snap-start for OD": {
			initCachingEnabled: true,
			sandboxType:        interop.SandboxClassic,
			expected:           InitTypeInitCaching,
		},
		"snap-start for PC": {
			initCachingEnabled: true,
			sandboxType:        interop.SandboxPreWarmed,
			expected:           InitTypeInitCaching,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			initType := InferInitType(tc.initCachingEnabled, tc.sandboxType)
			assert.Equal(t, tc.expected, initType)
		})
	}
}

func TestCalculateDuration(t *testing.T) {
	testCases := map[string]struct {
		start    int64
		end      int64
		expected float64
	}{
		"milliseconds only": {
			start:    int64(100 * time.Millisecond),
			end:      int64(120 * time.Millisecond),
			expected: 20,
		},
		"with microseconds": {
			start:    int64(100 * time.Millisecond),
			end:      int64(210*time.Millisecond + 65*time.Microsecond),
			expected: 110.065,
		},
		"nanoseconds must be dropped": {
			start:    int64(100 * time.Millisecond),
			end:      int64(140*time.Millisecond + 999*time.Nanosecond),
			expected: 40,
		},
		"microseconds presented, nanoseconds dropped": {
			start:    int64(100 * time.Millisecond),
			end:      int64(150*time.Millisecond + 2*time.Microsecond + 999*time.Nanosecond),
			expected: 50.002,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual := CalculateDuration(tc.start, tc.end)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
