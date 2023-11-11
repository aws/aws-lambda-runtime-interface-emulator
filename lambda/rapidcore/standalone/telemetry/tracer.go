// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/metering"
	"go.amzn.com/lambda/rapi/model"
	"go.amzn.com/lambda/telemetry"

	"github.com/sirupsen/logrus"
)

// InitSubsegmentName provides name attribute for Init subsegment
const InitSubsegmentName = "Initialization"

// RestoreSubsegmentName provides name attribute for Restore subsegment
const RestoreSubsegmentName = "Restore"

// InvokeSubsegmentName provides name attribute for Invoke subsegment
const InvokeSubsegmentName = "Invocation"

// OverheadSubsegmentName provides name attribute for Overhead subsegment
const OverheadSubsegmentName = "Overhead"

type StandaloneTracer struct {
	startFunction          func(ctx context.Context, invoke *interop.Invoke, segmentName string, timestamp int64)
	endFunction            func(ctx context.Context, invoke *interop.Invoke, segmentName string, timestamp int64)
	invoke                 *interop.Invoke
	tracingHeader          string
	rootTraceID            string
	parent                 string
	sampled                string
	lineage                string
	invocationSubsegmentID string
	initStartTime          int64
	initEndTime            int64
	restoreStartTime       int64
	restoreEndTime         int64
	restorePresent         bool
}

type TracingEvent struct {
	Message     string `json:"message"`
	TraceID     string `json:"trace_id"`
	SegmentName string `json:"segment_name"`
	SegmentID   string `json:"segment_id"`
	Timestamp   int64  `json:"timestamp"`
}

func (t *StandaloneTracer) Configure(invoke *interop.Invoke) {
	t.invoke = invoke
	t.tracingHeader = invoke.TraceID
	t.invocationSubsegmentID = ""
	t.rootTraceID, t.parent, t.sampled, t.lineage = telemetry.ParseTracingHeader(invoke.TraceID)
	if invoke.RestoreDurationNs == 0 {
		t.restorePresent = false
	} else {
		t.restorePresent = true
		t.restoreStartTime = metering.MonoToEpoch(invoke.RestoreStartTimeMonotime)
		t.restoreEndTime = t.restoreStartTime + invoke.RestoreDurationNs
	}
}

func (t *StandaloneTracer) CaptureInvokeSegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return t.withStartAndEnd(ctx, criticalFunction, "STANDALONE_FUNCTION_NAME")
}

func (t *StandaloneTracer) CaptureInitSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return t.withStartAndEnd(ctx, criticalFunction, InitSubsegmentName)
}

func (t *StandaloneTracer) CaptureInvokeSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	t.invocationSubsegmentID = InvokeSubsegmentName
	return t.withStartAndEnd(ctx, criticalFunction, InvokeSubsegmentName)
}

func (t *StandaloneTracer) CaptureOverheadSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return t.withStartAndEnd(ctx, criticalFunction, OverheadSubsegmentName)
}

func (t *StandaloneTracer) withStartAndEnd(ctx context.Context, criticalFunction func(context.Context) error, segmentName string) error {
	ctx = telemetry.NewTraceContext(ctx, t.rootTraceID, segmentName)
	t.startFunction(ctx, t.invoke, segmentName, time.Now().UnixNano())
	err := criticalFunction(ctx)
	t.endFunction(ctx, t.invoke, segmentName, time.Now().UnixNano())
	return err
}

func (t *StandaloneTracer) RecordInitStartTime() {
	t.initStartTime = time.Now().UnixNano()
}

func (t *StandaloneTracer) RecordInitEndTime() {
	t.initEndTime = time.Now().UnixNano()

}

func (t *StandaloneTracer) sendPrepSubsegment(ctx context.Context, subsegmentName string, startTime int64, endTime int64) {
	ctx = telemetry.NewTraceContext(ctx, t.rootTraceID, subsegmentName)
	t.startFunction(ctx, t.invoke, subsegmentName, startTime)
	t.endFunction(ctx, t.invoke, subsegmentName, endTime)
}

func (t *StandaloneTracer) SendInitSubsegmentWithRecordedTimesOnce(ctx context.Context) {
	t.sendPrepSubsegment(ctx, InitSubsegmentName, t.initStartTime, t.initEndTime)
}
func (t *StandaloneTracer) SendRestoreSubsegmentWithRecordedTimesOnce(ctx context.Context) {
	if t.restorePresent {
		t.sendPrepSubsegment(ctx, RestoreSubsegmentName, t.restoreStartTime, t.restoreEndTime)
	}
}
func (t *StandaloneTracer) MarkError(ctx context.Context)                                    {}
func (t *StandaloneTracer) AttachErrorCause(ctx context.Context, errorCause json.RawMessage) {}

func (t *StandaloneTracer) WithErrorCause(ctx context.Context, appCtx appctx.ApplicationContext, criticalFunction func(ctx context.Context) error) func(ctx context.Context) error {
	return criticalFunction
}
func (t *StandaloneTracer) WithError(ctx context.Context, appCtx appctx.ApplicationContext, criticalFunction func(ctx context.Context) error) func(ctx context.Context) error {
	return criticalFunction
}

func (t *StandaloneTracer) BuildTracingHeader() func(ctx context.Context) string {
	// extract root trace ID and parent from context and build the tracing header
	return func(ctx context.Context) string {
		var parent string
		var ok bool

		if parent, ok = ctx.Value(telemetry.DocumentIDKey).(string); !ok || parent == "" {
			return t.invoke.TraceID
		}

		if t.rootTraceID == "" || t.sampled == "" {
			return ""
		}

		var tracingHeader = "Root=%s;Parent=%s;Sampled=%s"

		if t.lineage == "" {
			return fmt.Sprintf(tracingHeader, t.rootTraceID, parent, t.sampled)
		}

		return fmt.Sprintf(tracingHeader+";Lineage=%s", t.rootTraceID, parent, t.sampled, t.lineage)
	}
}

func (t *StandaloneTracer) BuildTracingCtxForStart() *interop.TracingCtx {
	if t.rootTraceID == "" || t.sampled != model.XRaySampled {
		return nil
	}

	return &interop.TracingCtx{
		SpanID: t.parent,
		Type:   model.XRayTracingType,
		Value:  telemetry.BuildFullTraceID(t.rootTraceID, t.invoke.LambdaSegmentID, t.sampled),
	}
}
func (t *StandaloneTracer) BuildTracingCtxAfterInvokeComplete() *interop.TracingCtx {
	if t.rootTraceID == "" || t.sampled != model.XRaySampled || t.invocationSubsegmentID == "" {
		return nil
	}

	return &interop.TracingCtx{
		SpanID: t.invocationSubsegmentID,
		Type:   model.XRayTracingType,
		Value:  t.tracingHeader,
	}
}

func isTracingEnabled(root, parent, sampled string) bool {
	return len(root) != 0 && len(parent) != 0 && sampled == "1"
}

func NewStandaloneTracer(api *StandaloneEventsAPI) *StandaloneTracer {
	startCaptureFn := func(ctx context.Context, i *interop.Invoke, segmentName string, timestamp int64) {
		root, parent, sampled, _ := telemetry.ParseTracingHeader(i.TraceID)
		if isTracingEnabled(root, parent, sampled) {
			e := TracingEvent{
				Message:     "START",
				TraceID:     root,
				SegmentName: segmentName,
				SegmentID:   parent,
				Timestamp:   timestamp / int64(time.Millisecond),
			}
			api.LogTrace(e)
			log.WithFields(logrus.Fields{"trace": e}).Info("sandbox trace")
		}
	}

	endCaptureFn := func(ctx context.Context, i *interop.Invoke, segmentName string, timestamp int64) {
		root, parent, sampled, _ := telemetry.ParseTracingHeader(i.TraceID)
		if isTracingEnabled(root, parent, sampled) {
			e := TracingEvent{
				Message:     "END",
				TraceID:     root,
				SegmentName: "",
				SegmentID:   parent,
				Timestamp:   timestamp / int64(time.Millisecond),
			}
			api.LogTrace(e)
			log.WithFields(logrus.Fields{"trace": e}).Info("sandbox trace")
		}
	}

	return &StandaloneTracer{
		startFunction: startCaptureFn,
		endFunction:   endCaptureFn,
	}
}
