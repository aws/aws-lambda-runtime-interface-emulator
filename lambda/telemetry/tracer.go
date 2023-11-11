// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi/model"
)

type traceContextKey int

const (
	TraceIDKey traceContextKey = iota
	DocumentIDKey
)

type Tracer interface {
	Configure(invoke *interop.Invoke)
	CaptureInvokeSegment(ctx context.Context, criticalFunction func(context.Context) error) error
	CaptureInitSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error
	CaptureInvokeSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error
	CaptureOverheadSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error
	RecordInitStartTime()
	RecordInitEndTime()
	SendInitSubsegmentWithRecordedTimesOnce(ctx context.Context)
	SendRestoreSubsegmentWithRecordedTimesOnce(ctx context.Context)
	MarkError(ctx context.Context)
	AttachErrorCause(ctx context.Context, errorCause json.RawMessage)
	WithErrorCause(ctx context.Context, appCtx appctx.ApplicationContext, criticalFunction func(ctx context.Context) error) func(ctx context.Context) error
	WithError(ctx context.Context, appCtx appctx.ApplicationContext, criticalFunction func(ctx context.Context) error) func(ctx context.Context) error
	BuildTracingHeader() func(context.Context) string
	BuildTracingCtxForStart() *interop.TracingCtx
	BuildTracingCtxAfterInvokeComplete() *interop.TracingCtx
}

type NoOpTracer struct{}

func (t *NoOpTracer) Configure(invoke *interop.Invoke) {}

func (t *NoOpTracer) CaptureInvokeSegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return criticalFunction(ctx)
}

func (t *NoOpTracer) CaptureInitSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return criticalFunction(ctx)
}

func (t *NoOpTracer) CaptureInvokeSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return criticalFunction(ctx)
}

func (t *NoOpTracer) CaptureOverheadSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return criticalFunction(ctx)
}

func (t *NoOpTracer) RecordInitStartTime()                                             {}
func (t *NoOpTracer) RecordInitEndTime()                                               {}
func (t *NoOpTracer) SendInitSubsegmentWithRecordedTimesOnce(ctx context.Context)      {}
func (t *NoOpTracer) SendRestoreSubsegmentWithRecordedTimesOnce(ctx context.Context)   {}
func (t *NoOpTracer) MarkError(ctx context.Context)                                    {}
func (t *NoOpTracer) AttachErrorCause(ctx context.Context, errorCause json.RawMessage) {}

func (t *NoOpTracer) WithErrorCause(ctx context.Context, appCtx appctx.ApplicationContext, criticalFunction func(ctx context.Context) error) func(ctx context.Context) error {
	return criticalFunction
}
func (t *NoOpTracer) WithError(ctx context.Context, appCtx appctx.ApplicationContext, criticalFunction func(ctx context.Context) error) func(ctx context.Context) error {
	return criticalFunction
}
func (t *NoOpTracer) BuildTracingHeader() func(context.Context) string {
	// extract root trace ID and parent from context and build the tracing header
	return func(ctx context.Context) string {
		root, _ := ctx.Value(TraceIDKey).(string)
		parent, _ := ctx.Value(DocumentIDKey).(string)

		if root != "" && parent != "" {
			return fmt.Sprintf("Root=%s;Parent=%s;Sampled=1", root, parent)
		}

		return ""
	}
}

func (t *NoOpTracer) BuildTracingCtxForStart() *interop.TracingCtx {
	return nil
}
func (t *NoOpTracer) BuildTracingCtxAfterInvokeComplete() *interop.TracingCtx {
	return nil
}

func NewNoOpTracer() *NoOpTracer {
	return &NoOpTracer{}
}

// NewTraceContext returns new derived context with trace config set for testing
func NewTraceContext(ctx context.Context, root string, parent string) context.Context {
	ctxWithRoot := context.WithValue(ctx, TraceIDKey, root)
	return context.WithValue(ctxWithRoot, DocumentIDKey, parent)
}

// ParseTracingHeader extracts RootTraceID, ParentID, Sampled, and Lineage from a tracing header.
// Tracing header format is defined here:
// https://docs.aws.amazon.com/xray/latest/devguide/xray-concepts.html#xray-concepts-tracingheader
func ParseTracingHeader(tracingHeader string) (rootID, parentID, sampled, lineage string) {
	keyValuePairs := strings.Split(tracingHeader, ";")
	for _, pair := range keyValuePairs {
		var key, value string
		keyValue := strings.Split(pair, "=")
		if len(keyValue) == 2 {
			key = keyValue[0]
			value = keyValue[1]
		}
		switch key {
		case "Root":
			rootID = value
		case "Parent":
			parentID = value
		case "Sampled":
			sampled = value
		case "Lineage":
			lineage = value
		}
	}
	return
}

// BuildFullTraceID takes individual components of X-Ray trace header
// and puts them together into a formatted trace header.
// If root is empty, returns an empty string.
func BuildFullTraceID(root, parent, sample string) string {
	if root == "" {
		return ""
	}

	parts := make([]string, 0, 3)
	parts = append(parts, "Root="+root)
	if parent != "" {
		parts = append(parts, "Parent="+parent)
	}
	if sample == "" {
		sample = model.XRayNonSampled
	}
	parts = append(parts, "Sampled="+sample)

	return strings.Join(parts, ";")
}
