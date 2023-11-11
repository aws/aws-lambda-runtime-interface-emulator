// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"time"

	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/metering"
)

func GetRuntimeDoneInvokeMetrics(runtimeStartedTime int64, invokeResponseMetrics *interop.InvokeResponseMetrics, runtimeDoneTime int64) *interop.RuntimeDoneInvokeMetrics {
	// time taken from sending the invoke to the sandbox until the runtime calls GET /next
	duration := CalculateDuration(runtimeStartedTime, runtimeDoneTime)
	if invokeResponseMetrics != nil && invokeResponseMetrics.RuntimeCalledResponse && runtimeStartedTime != -1 {
		return &interop.RuntimeDoneInvokeMetrics{
			ProducedBytes: invokeResponseMetrics.ProducedBytes,
			DurationMs:    duration,
		}
	}

	// when we get a reset before runtime called /response
	if runtimeStartedTime != -1 {
		return &interop.RuntimeDoneInvokeMetrics{
			ProducedBytes: int64(0),
			DurationMs:    duration,
		}
	}

	// We didn't have time to register the invokeReceiveTime, which means we crash/reset very early,
	// too early for the runtime to actual run. In such case, the runtimeDone event shouldn't be sent
	// Not returning Nil even in this improbable case guarantees that we will always have some metrics to send to FluxPump
	return &interop.RuntimeDoneInvokeMetrics{
		ProducedBytes: int64(0),
		DurationMs:    float64(0),
	}
}

const (
	InitInsideInitPhase   interop.InitPhase = "init"
	InitInsideInvokePhase interop.InitPhase = "invoke"
)

func InitPhaseFromLifecyclePhase(phase interop.LifecyclePhase) (interop.InitPhase, error) {
	switch phase {
	case interop.LifecyclePhaseInit:
		return InitInsideInitPhase, nil
	case interop.LifecyclePhaseInvoke:
		return InitInsideInvokePhase, nil
	default:
		return interop.InitPhase(""), fmt.Errorf("unexpected lifecycle phase: %v", phase)
	}
}

func GetRuntimeDoneSpans(runtimeStartedTime int64, invokeResponseMetrics *interop.InvokeResponseMetrics) []interop.Span {
	if invokeResponseMetrics != nil && invokeResponseMetrics.RuntimeCalledResponse && runtimeStartedTime != -1 {
		// time span from when the invoke is received in the sandbox to the moment the runtime calls PUT /response
		responseLatencyMsSpan := interop.Span{
			Name:       "responseLatency",
			Start:      GetEpochTimeInISO8601FormatFromMonotime(runtimeStartedTime),
			DurationMs: CalculateDuration(runtimeStartedTime, invokeResponseMetrics.StartReadingResponseMonoTimeMs),
		}

		// time span from when the runtime called PUT /response to the moment the body of the response is fully sent
		responseDurationMsSpan := interop.Span{
			Name:       "responseDuration",
			Start:      GetEpochTimeInISO8601FormatFromMonotime(invokeResponseMetrics.StartReadingResponseMonoTimeMs),
			DurationMs: CalculateDuration(invokeResponseMetrics.StartReadingResponseMonoTimeMs, invokeResponseMetrics.FinishReadingResponseMonoTimeMs),
		}
		return []interop.Span{responseLatencyMsSpan, responseDurationMsSpan}
	}

	return []interop.Span{}
}

// CalculateDuration calculates duration between two moments.
// The result is milliseconds with microsecond precision.
// Two assumptions here:
// 1. the passed values are nanoseconds
// 2. endNs > startNs
func CalculateDuration(startNs, endNs int64) float64 {
	microseconds := int64(endNs-startNs) / int64(time.Microsecond)
	return float64(microseconds) / 1000
}

const (
	InitTypeOnDemand               interop.InitType = "on-demand"
	InitTypeProvisionedConcurrency interop.InitType = "provisioned-concurrency"
	InitTypeInitCaching            interop.InitType = "snap-start"
)

func InferInitType(initCachingEnabled bool, sandboxType interop.SandboxType) interop.InitType {
	initSource := InitTypeOnDemand

	// ToDo: Unify this selection of SandboxType by using the START message
	// after having a roadmap on the combination of INIT modes
	if initCachingEnabled {
		initSource = InitTypeInitCaching
	} else if sandboxType == interop.SandboxPreWarmed {
		initSource = InitTypeProvisionedConcurrency
	}

	return initSource
}

func GetEpochTimeInISO8601FormatFromMonotime(monotime int64) string {
	return time.Unix(0, metering.MonoToEpoch(monotime)).Format("2006-01-02T15:04:05.000Z")
}

const (
	RuntimeDoneSuccess = "success"
	RuntimeDoneError   = "error"
)

type NoOpEventsAPI struct{}

func (s *NoOpEventsAPI) SetCurrentRequestID(interop.RequestID) {}

func (s *NoOpEventsAPI) SendInitStart(interop.InitStartData) error { return nil }

func (s *NoOpEventsAPI) SendInitRuntimeDone(interop.InitRuntimeDoneData) error { return nil }

func (s *NoOpEventsAPI) SendInitReport(interop.InitReportData) error { return nil }

func (s *NoOpEventsAPI) SendRestoreRuntimeDone(interop.RestoreRuntimeDoneData) error { return nil }

func (s *NoOpEventsAPI) SendInvokeStart(interop.InvokeStartData) error { return nil }

func (s *NoOpEventsAPI) SendInvokeRuntimeDone(interop.InvokeRuntimeDoneData) error { return nil }

func (s *NoOpEventsAPI) SendExtensionInit(interop.ExtensionInitData) error { return nil }

func (s *NoOpEventsAPI) SendEnd(interop.EndData) error { return nil }

func (s *NoOpEventsAPI) SendReportSpan(interop.Span) error { return nil }

func (s *NoOpEventsAPI) SendReport(interop.ReportData) error { return nil }

func (s *NoOpEventsAPI) SendFault(interop.FaultData) error { return nil }

func (s *NoOpEventsAPI) SendImageErrorLog(interop.ImageErrorLogData) {}

func (s *NoOpEventsAPI) FetchTailLogs(string) (string, error) { return "", nil }

func (s *NoOpEventsAPI) GetRuntimeDoneSpans(
	runtimeStartedTime int64,
	invokeResponseMetrics *interop.InvokeResponseMetrics,
	runtimeOverheadStartedTime int64,
	runtimeReadyTime int64,
) []interop.Span {
	return []interop.Span{}
}
