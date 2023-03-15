// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"time"

	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/metering"
	"go.amzn.com/lambda/rapi/model"
)

type RuntimeDoneInvokeMetrics struct {
	ProducedBytes int64
	DurationMs    float64
}

func GetRuntimeDoneInvokeMetrics(invokeReceivedTime int64, invokeResponseMetrics *interop.InvokeResponseMetrics, runtimeDoneTime int64) *RuntimeDoneInvokeMetrics {
	if invokeResponseMetrics != nil && invokeResponseMetrics.RuntimeCalledResponse && invokeReceivedTime != 0 {
		return &RuntimeDoneInvokeMetrics{
			ProducedBytes: invokeResponseMetrics.ProducedBytes,
			// time taken from sending the invoke to the sandbox until the runtime calls GET /next
			DurationMs: float64((runtimeDoneTime - invokeReceivedTime) / int64(time.Millisecond)),
		}
	}

	// when we get a reset before runtime called /response
	if invokeReceivedTime != 0 {
		return &RuntimeDoneInvokeMetrics{
			ProducedBytes: int64(0),
			DurationMs:    float64((runtimeDoneTime - invokeReceivedTime) / int64(time.Millisecond)),
		}
	}

	// We didn't have time to register the invokeReceiveTime, which means we crash/reset very early,
	// too early for the runtime to actual run. In such case, the runtimeDone event shouldn't be sent
	// Not returning Nil even in this improbable case guarantees that we will always have some metrics to send to FluxPump
	return &RuntimeDoneInvokeMetrics{
		ProducedBytes: int64(0),
		DurationMs:    float64(0),
	}
}

type InitRuntimeDoneData struct {
	InitSource string
	Status     string
}

type InvokeRuntimeDoneData struct {
	Status          string
	Metrics         *RuntimeDoneInvokeMetrics
	InternalMetrics *interop.InvokeResponseMetrics
	Tracing         *TracingCtx
	Spans           []Span
}

type Span struct {
	Name       string
	Start      string
	DurationMs float64
}

func GetRuntimeDoneSpans(invokeReceivedTime int64, invokeResponseMetrics *interop.InvokeResponseMetrics) []Span {
	if invokeResponseMetrics != nil && invokeResponseMetrics.RuntimeCalledResponse && invokeReceivedTime != 0 {
		// time span from when the invoke is received in the sandbox to the moment the runtime calls PUT /response
		responseLatencyMsSpan := Span{
			Name:       "responseLatency",
			Start:      getEpochTimeInISO8601FormatFromMonotime(invokeReceivedTime),
			DurationMs: float64((invokeResponseMetrics.StartReadingResponseMonoTimeMs - invokeReceivedTime) / int64(time.Millisecond)),
		}

		// time span from when the runtime called PUT /response to the moment the body of the response is fully sent
		responseDurationMsSpan := Span{
			Name:       "responseDuration",
			Start:      getEpochTimeInISO8601FormatFromMonotime(invokeResponseMetrics.StartReadingResponseMonoTimeMs),
			DurationMs: float64((invokeResponseMetrics.FinishReadingResponseMonoTimeMs - invokeResponseMetrics.StartReadingResponseMonoTimeMs) / int64(time.Millisecond)),
		}
		return []Span{responseLatencyMsSpan, responseDurationMsSpan}
	}

	return []Span{}
}

func getEpochTimeInISO8601FormatFromMonotime(monotime int64) string {
	return time.Unix(0, metering.MonoToEpoch(monotime)).Format("2006-01-02T15:04:05.000Z")
}

type TracingCtx struct {
	SpanID string
	Type   model.TracingType
	Value  string
}

func BuildTracingCtx(tracingType model.TracingType, traceID string, lambdaSegmentID string) *TracingCtx {
	// it takes current tracing context and change its parent value with the provided lambda segment id
	root, currentParent, sample := ParseTraceID(traceID)
	if root == "" || sample != model.XRaySampled {
		return nil
	}

	return &TracingCtx{
		SpanID: currentParent,
		Type:   tracingType,
		Value:  BuildFullTraceID(root, lambdaSegmentID, sample),
	}
}

const (
	RuntimeDoneSuccess = "success"
	RuntimeDoneFailure = "failure"
)

type EventsAPI interface {
	SetCurrentRequestID(requestID string)
	SendInitRuntimeDone(data *InitRuntimeDoneData) error
	SendRestoreRuntimeDone(status string) error
	SendRuntimeDone(data InvokeRuntimeDoneData) error
	SendExtensionInit(agentName, state, errorType string, subscriptions []string) error
	SendImageErrorLog(logline string)
}

type NoOpEventsAPI struct{}

func (s *NoOpEventsAPI) SetCurrentRequestID(requestID string) {}
func (s *NoOpEventsAPI) SendInitRuntimeDone(data *InitRuntimeDoneData) error {
	return nil
}
func (s *NoOpEventsAPI) SendRestoreRuntimeDone(status string) error {
	return nil
}
func (s *NoOpEventsAPI) SendRuntimeDone(data InvokeRuntimeDoneData) error {
	return nil
}
func (s *NoOpEventsAPI) SendExtensionInit(agentName, state, errorType string, subscriptions []string) error {
	return nil
}
func (s *NoOpEventsAPI) SendImageErrorLog(logline string) {}
