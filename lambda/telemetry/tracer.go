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
)

type traceContextKey int

const (
	traceIDKey traceContextKey = iota
	documentIDKey
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
	MarkError(ctx context.Context)
	AttachErrorCause(ctx context.Context, errorCause json.RawMessage)
	WithErrorCause(ctx context.Context, appCtx appctx.ApplicationContext, criticalFunction func(ctx context.Context) error) func(ctx context.Context) error
	WithError(ctx context.Context, appCtx appctx.ApplicationContext, criticalFunction func(ctx context.Context) error) func(ctx context.Context) error
	TracingHeaderParser() func(context.Context, *interop.Invoke) string
}

type NoOpTracer struct{}

func (t *NoOpTracer) Configure(invoke *interop.Invoke) {}

func (t *NoOpTracer) CaptureInvokeSegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	criticalFunction(ctx)
	return nil
}

func (t *NoOpTracer) CaptureInitSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	criticalFunction(ctx)
	return nil
}

func (t *NoOpTracer) CaptureInvokeSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	criticalFunction(ctx)
	return nil
}

func (t *NoOpTracer) CaptureOverheadSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	criticalFunction(ctx)
	return nil
}

func (t *NoOpTracer) RecordInitStartTime()                                             {}
func (t *NoOpTracer) RecordInitEndTime()                                               {}
func (t *NoOpTracer) SendInitSubsegmentWithRecordedTimesOnce(ctx context.Context)      {}
func (t *NoOpTracer) MarkError(ctx context.Context)                                    {}
func (t *NoOpTracer) AttachErrorCause(ctx context.Context, errorCause json.RawMessage) {}

func (t *NoOpTracer) WithErrorCause(ctx context.Context, appCtx appctx.ApplicationContext, criticalFunction func(ctx context.Context) error) func(ctx context.Context) error {
	return criticalFunction
}
func (t *NoOpTracer) WithError(ctx context.Context, appCtx appctx.ApplicationContext, criticalFunction func(ctx context.Context) error) func(ctx context.Context) error {
	return criticalFunction
}
func (t *NoOpTracer) TracingHeaderParser() func(context.Context, *interop.Invoke) string {
	return GetCustomerTracingHeader
}

func NewNoOpTracer() *NoOpTracer {
	return &NoOpTracer{}
}

// NewTraceContext returns new derived context with trace config set for testing
func NewTraceContext(ctx context.Context, root string, parent string) context.Context {
	ctxWithRoot := context.WithValue(ctx, traceIDKey, root)
	return context.WithValue(ctxWithRoot, documentIDKey, parent)
}

// GetCustomerTracingHeader extracts the trace config from trace context and constructs header
func GetCustomerTracingHeader(ctx context.Context, invoke *interop.Invoke) string {
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

// ParseTraceID helps client to get TraceID, ParentID, Sampled information from a full trace
func ParseTraceID(fullTraceID string) (rootID, parentID, sample string) {
	traceIDInfo := strings.Split(fullTraceID, ";")
	for i := 0; i < len(traceIDInfo); i++ {
		if len(traceIDInfo[i]) == 0 {
			continue
		} else {
			var key string
			var value string
			keyValuePair := strings.Split(traceIDInfo[i], "=")
			if len(keyValuePair) == 2 {
				key = keyValuePair[0]
				value = keyValuePair[1]
			}
			switch key {
			case "Root":
				rootID = value
			case "Parent":
				parentID = value
			case "Sampled":
				sample = value
			}
		}
	}
	return
}
