// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/telemetry"
)

// InitSubsegmentName provides name attribute for Init subsegment
const InitSubsegmentName = "Initialization"

// InvokeSubsegmentName provides name attribute for Invoke subsegment
const InvokeSubsegmentName = "Invocation"

// OverheadSubsegmentName provides name attribute for Overhead subsegment
const OverheadSubsegmentName = "Overhead"

type traceContextKey int

const (
	traceIDKey traceContextKey = iota
	documentIDKey
)

type StandaloneTracer struct {
	startFunction func(ctx context.Context, invoke *interop.Invoke, segmentName string)
	endFunction   func(ctx context.Context, invoke *interop.Invoke, segmentName string)
	functionName  string
	invoke        *interop.Invoke
}

func (t *StandaloneTracer) Configure(invoke *interop.Invoke) {

	t.invoke = invoke
}

func (t *StandaloneTracer) CaptureInvokeSegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return t.withStartAndEnd(ctx, criticalFunction, t.functionName)
}

func (t *StandaloneTracer) CaptureInitSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return t.withStartAndEnd(ctx, criticalFunction, InitSubsegmentName)
}

func (t *StandaloneTracer) CaptureInvokeSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return t.withStartAndEnd(ctx, criticalFunction, InvokeSubsegmentName)
}

func (t *StandaloneTracer) CaptureOverheadSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return t.withStartAndEnd(ctx, criticalFunction, OverheadSubsegmentName)
}

func (t *StandaloneTracer) withStartAndEnd(ctx context.Context, criticalFunction func(context.Context) error, segmentName string) error {
	t.startFunction(ctx, t.invoke, segmentName)
	err := criticalFunction(ctx)
	t.endFunction(ctx, t.invoke, segmentName)
	return err
}

func (t *StandaloneTracer) RecordInitStartTime()                                             {}
func (t *StandaloneTracer) RecordInitEndTime()                                               {}
func (t *StandaloneTracer) SendInitSubsegmentWithRecordedTimesOnce(ctx context.Context)      {}
func (t *StandaloneTracer) MarkError(ctx context.Context)                                    {}
func (t *StandaloneTracer) AttachErrorCause(ctx context.Context, errorCause json.RawMessage) {}

func (t *StandaloneTracer) WithErrorCause(ctx context.Context, appCtx appctx.ApplicationContext, criticalFunction func(ctx context.Context) error) func(ctx context.Context) error {
	return criticalFunction
}
func (t *StandaloneTracer) WithError(ctx context.Context, appCtx appctx.ApplicationContext, criticalFunction func(ctx context.Context) error) func(ctx context.Context) error {
	return criticalFunction
}
func (t *StandaloneTracer) TracingHeaderParser() func(context.Context, *interop.Invoke) string {
	getCustomerTracingHeader := func(ctx context.Context, invoke *interop.Invoke) string {
		var root, parent string
		var ok bool

		if root, ok = ctx.Value(traceIDKey).(string); !ok {
			return invoke.TraceID
		}

		if parent, ok = ctx.Value(documentIDKey).(string); !ok {
			return invoke.TraceID
		}

		return fmt.Sprintf("Root=%s;Parent=%s;Sampled=1", root, parent)
	}

	return getCustomerTracingHeader
}

func isTracingEnabled(root, parent, sampled string) bool {
	return len(root) != 0 && len(parent) != 0 && sampled == "1"
}

func NewStandaloneTracer(eventLog io.Writer, functionName string) *StandaloneTracer {
	traceFormat := "XRAY\tMessage: %s\tTraceID: %s\tSegmentName: %s\tSegmentID: %s"
	startCaptureFn := func(ctx context.Context, i *interop.Invoke, segmentName string) {
		root, parent, sampled := telemetry.ParseTraceID(i.TraceID)
		if isTracingEnabled(root, parent, sampled) {
			fmt.Fprintf(eventLog, traceFormat, "START", root, segmentName, parent)
		}
	}

	endCaptureFn := func(ctx context.Context, i *interop.Invoke, segmentName string) {
		root, parent, sampled := telemetry.ParseTraceID(i.TraceID)
		if isTracingEnabled(root, parent, sampled) {
			fmt.Fprintf(eventLog, traceFormat, "END", root, "", parent)
		}
	}

	return &StandaloneTracer{
		startFunction: startCaptureFn,
		endFunction:   endCaptureFn,
		functionName:  functionName,
	}
}
