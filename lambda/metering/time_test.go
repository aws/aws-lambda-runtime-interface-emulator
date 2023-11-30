// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package metering

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMonoToEpochPrecision(t *testing.T) {
	a := time.Now().UnixNano()
	b := MonoToEpoch(Monotime())

	// Conversion error is less than a millisecond.
	assert.True(t, math.Abs(float64(a-b)) < float64(time.Millisecond))
}

func TestEpochToMonoPrecision(t *testing.T) {
	a := Monotime()
	b := TimeToMono(time.Now())

	// Conversion error is less than a millisecond.
	assert.Less(t, math.Abs(float64(b-a)), float64(1*time.Millisecond))
}

func TestExtensionsResetDurationProfilerForExtensionsResetWithNoExtensions(t *testing.T) {
	mono := Monotime()
	profiler := ExtensionsResetDurationProfiler{}

	profiler.extensionsResetStartTimeNs = mono
	profiler.extensionsResetEndTimeNs = mono + time.Second.Nanoseconds()
	profiler.AvailableNs = 3 * time.Second.Nanoseconds()
	profiler.NumAgentsRegisteredForShutdown = 0
	extensionsResetMs, resetTimeout := profiler.CalculateExtensionsResetMs()

	assert.Equal(t, int64(0), extensionsResetMs)
	assert.Equal(t, false, resetTimeout)
}

func TestExtensionsResetDurationProfilerForExtensionsResetWithinDeadline(t *testing.T) {
	mono := Monotime()
	profiler := ExtensionsResetDurationProfiler{}

	profiler.extensionsResetStartTimeNs = mono
	profiler.extensionsResetEndTimeNs = mono + time.Second.Nanoseconds()
	profiler.AvailableNs = 3 * time.Second.Nanoseconds()
	profiler.NumAgentsRegisteredForShutdown = 1
	extensionsResetMs, resetTimeout := profiler.CalculateExtensionsResetMs()

	assert.Equal(t, time.Second.Milliseconds(), extensionsResetMs)
	assert.Equal(t, false, resetTimeout)
}

func TestExtensionsResetDurationProfilerForExtensionsResetTimeout(t *testing.T) {
	mono := Monotime()
	profiler := ExtensionsResetDurationProfiler{}

	profiler.extensionsResetStartTimeNs = mono
	profiler.extensionsResetEndTimeNs = mono + 3*time.Second.Nanoseconds()
	profiler.AvailableNs = time.Second.Nanoseconds()
	profiler.NumAgentsRegisteredForShutdown = 1
	extensionsResetMs, resetTimeout := profiler.CalculateExtensionsResetMs()

	assert.Equal(t, time.Second.Milliseconds(), extensionsResetMs)
	assert.Equal(t, true, resetTimeout)
}

func TestExtensionsResetDurationProfilerEndToEnd(t *testing.T) {
	profiler := ExtensionsResetDurationProfiler{}

	profiler.Start()
	time.Sleep(time.Second)
	profiler.Stop()

	profiler.AvailableNs = 2 * time.Second.Nanoseconds()
	profiler.NumAgentsRegisteredForShutdown = 1
	extensionsResetMs, _ := profiler.CalculateExtensionsResetMs()

	assert.GreaterOrEqual(t, 2*time.Second.Milliseconds(), extensionsResetMs)
	assert.LessOrEqual(t, time.Second.Milliseconds(), extensionsResetMs)
}
