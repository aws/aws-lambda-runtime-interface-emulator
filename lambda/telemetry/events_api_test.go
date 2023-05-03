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

	invokeReceivedTime := now
	invokeResponseMetrics := &interop.InvokeResponseMetrics{
		ProducedBytes:         int64(100),
		RuntimeCalledResponse: true,
	}
	runtimeDoneTime := now + int64(time.Millisecond*time.Duration(10))

	expected := &RuntimeDoneInvokeMetrics{
		ProducedBytes: int64(100),
		DurationMs:    float64(10),
	}

	assert.Equal(t, expected, GetRuntimeDoneInvokeMetrics(invokeReceivedTime, invokeResponseMetrics, runtimeDoneTime))
}

func TestGetRuntimeDoneInvokeMetricsWhenRuntimeCalledError(t *testing.T) {
	now := metering.Monotime()

	invokeReceivedTime := now
	invokeResponseMetrics := &interop.InvokeResponseMetrics{
		ProducedBytes:         int64(100),
		RuntimeCalledResponse: false,
	}
	runtimeDoneTime := now + int64(time.Millisecond*time.Duration(10))

	expected := &RuntimeDoneInvokeMetrics{
		ProducedBytes: int64(0),
		DurationMs:    float64(10),
	}

	assert.Equal(t, expected, GetRuntimeDoneInvokeMetrics(invokeReceivedTime, invokeResponseMetrics, runtimeDoneTime))
}

func TestGetRuntimeDoneInvokeMetricsWhenInvokeReceivedTimeIsZero(t *testing.T) {
	now := int64(0) // January 1st, 1970 at 00:00:00 UTC
	invokeReceivedTime := now

	runtimeDoneTime := now + int64(time.Millisecond*time.Duration(10))

	expected := &RuntimeDoneInvokeMetrics{
		ProducedBytes: int64(0),
		DurationMs:    float64(0),
	}
	actual := GetRuntimeDoneInvokeMetrics(invokeReceivedTime, nil, runtimeDoneTime)
	assert.Equal(t, expected, actual)
}

func TestGetRuntimeDoneInvokeMetricsWhenInvokeResponseMetricsIsNil(t *testing.T) {
	now := metering.Monotime()
	invokeReceivedTime := now

	runtimeDoneTime := now + int64(time.Millisecond*time.Duration(10))

	expected := &RuntimeDoneInvokeMetrics{
		ProducedBytes: int64(0),
		DurationMs:    float64(10),
	}

	assert.Equal(t, expected, GetRuntimeDoneInvokeMetrics(invokeReceivedTime, nil, runtimeDoneTime))
}

func TestGetRuntimeDoneSpans(t *testing.T) {
	now := metering.Monotime()
	startReadingResponseMonoTimeMs := now + int64(time.Millisecond*time.Duration(5))
	finishReadingResponseMonoTimeMs := now + int64(time.Millisecond*time.Duration(7))

	invokeReceivedTime := now
	invokeResponseMetrics := &interop.InvokeResponseMetrics{
		StartReadingResponseMonoTimeMs:  startReadingResponseMonoTimeMs,
		FinishReadingResponseMonoTimeMs: finishReadingResponseMonoTimeMs,
		RuntimeCalledResponse:           true,
	}

	expectedResponseLatencyMsStartTime := getEpochTimeInISO8601FormatFromMonotime(now)
	expectedResponseDurationMsStartTime := getEpochTimeInISO8601FormatFromMonotime(startReadingResponseMonoTimeMs)
	expected := []Span{
		Span{
			Name:       "responseLatency",
			Start:      expectedResponseLatencyMsStartTime,
			DurationMs: 5,
		},
		Span{
			Name:       "responseDuration",
			Start:      expectedResponseDurationMsStartTime,
			DurationMs: 2,
		},
	}

	assert.Equal(t, expected, GetRuntimeDoneSpans(invokeReceivedTime, invokeResponseMetrics))
}

func TestGetRuntimeDoneSpansWhenRuntimeCalledError(t *testing.T) {
	now := metering.Monotime()
	startReadingResponseMonoTimeMs := now + int64(time.Millisecond*time.Duration(5))
	finishReadingResponseMonoTimeMs := now + int64(time.Millisecond*time.Duration(7))

	invokeReceivedTime := now
	invokeResponseMetrics := &interop.InvokeResponseMetrics{
		StartReadingResponseMonoTimeMs:  startReadingResponseMonoTimeMs,
		FinishReadingResponseMonoTimeMs: finishReadingResponseMonoTimeMs,
		RuntimeCalledResponse:           false,
	}

	assert.Equal(t, []Span{}, GetRuntimeDoneSpans(invokeReceivedTime, invokeResponseMetrics))
}

func TestGetRuntimeDoneSpansWhenInvokeResponseMetricsNil(t *testing.T) {
	invokeReceivedTime := metering.Monotime()

	assert.Equal(t, []Span{}, GetRuntimeDoneSpans(invokeReceivedTime, nil))
}

func TestGetRuntimeDoneSpansWhenInvokeReceivedTimeIsZero(t *testing.T) {
	now := int64(0) // January 1st, 1970 at 00:00:00 UTC
	invokeReceivedTime := now
	invokeResponseMetrics := &interop.InvokeResponseMetrics{
		StartReadingResponseMonoTimeMs:  now + int64(time.Millisecond*time.Duration(5)),
		FinishReadingResponseMonoTimeMs: now + int64(time.Millisecond*time.Duration(7)),
	}

	assert.Equal(t, []Span{}, GetRuntimeDoneSpans(invokeReceivedTime, invokeResponseMetrics))
}
