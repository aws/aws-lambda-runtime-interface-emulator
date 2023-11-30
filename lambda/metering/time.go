// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package metering

import (
	_ "runtime" //for nanotime() and walltime()
	"time"
	_ "unsafe" //for go:linkname
)

//go:linkname Monotime runtime.nanotime
func Monotime() int64

// MonoToEpoch converts monotonic time nanos to unix epoch time nanos.
func MonoToEpoch(t int64) int64 {
	monoNsec := Monotime()
	wallNsec := time.Now().UnixNano()
	clockOffset := wallNsec - monoNsec
	return t + clockOffset
}

func TimeToMono(t time.Time) int64 {
	durNs := time.Since(t).Nanoseconds()
	return Monotime() - durNs
}

type ExtensionsResetDurationProfiler struct {
	NumAgentsRegisteredForShutdown int
	AvailableNs                    int64
	extensionsResetStartTimeNs     int64
	extensionsResetEndTimeNs       int64
}

func (p *ExtensionsResetDurationProfiler) Start() {
	p.extensionsResetStartTimeNs = Monotime()
}

func (p *ExtensionsResetDurationProfiler) Stop() {
	p.extensionsResetEndTimeNs = Monotime()
}

func (p *ExtensionsResetDurationProfiler) CalculateExtensionsResetMs() (int64, bool) {
	var extensionsResetDurationNs = p.extensionsResetEndTimeNs - p.extensionsResetStartTimeNs
	var extensionsResetMs int64
	timedOut := false

	if p.NumAgentsRegisteredForShutdown == 0 || p.AvailableNs < 0 || extensionsResetDurationNs < 0 {
		extensionsResetMs = 0
	} else if extensionsResetDurationNs > p.AvailableNs {
		extensionsResetMs = p.AvailableNs / time.Millisecond.Nanoseconds()
		timedOut = true
	} else {
		extensionsResetMs = extensionsResetDurationNs / time.Millisecond.Nanoseconds()
	}

	return extensionsResetMs, timedOut
}
