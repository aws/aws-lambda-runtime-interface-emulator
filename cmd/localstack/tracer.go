package main

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/interop"
)

type LocalStackTracer struct {
	invoke *interop.Invoke
}

func (t *LocalStackTracer) Configure(invoke *interop.Invoke) {
	t.invoke = invoke
}

func (t *LocalStackTracer) CaptureInvokeSegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return criticalFunction(ctx)
}

func (t *LocalStackTracer) CaptureInitSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return criticalFunction(ctx)
}

func (t *LocalStackTracer) CaptureInvokeSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return criticalFunction(ctx)
}

func (t *LocalStackTracer) CaptureOverheadSubsegment(ctx context.Context, criticalFunction func(context.Context) error) error {
	return criticalFunction(ctx)
}

func (t *LocalStackTracer) RecordInitStartTime()                                             {}
func (t *LocalStackTracer) RecordInitEndTime()                                               {}
func (t *LocalStackTracer) SendInitSubsegmentWithRecordedTimesOnce(ctx context.Context)      {}
func (t *LocalStackTracer) SendRestoreSubsegmentWithRecordedTimesOnce(ctx context.Context)   {}
func (t *LocalStackTracer) MarkError(ctx context.Context)                                    {}
func (t *LocalStackTracer) AttachErrorCause(ctx context.Context, errorCause json.RawMessage) {}

func (t *LocalStackTracer) WithErrorCause(ctx context.Context, appCtx appctx.ApplicationContext, criticalFunction func(ctx context.Context) error) func(ctx context.Context) error {
	return criticalFunction
}
func (t *LocalStackTracer) WithError(ctx context.Context, appCtx appctx.ApplicationContext, criticalFunction func(ctx context.Context) error) func(ctx context.Context) error {
	return criticalFunction
}
func (t *LocalStackTracer) BuildTracingHeader() func(context.Context) string {
	// extract root trace ID and parent from context and build the tracing header
	return func(ctx context.Context) string {
		return t.invoke.TraceID
	}
}

func (t *LocalStackTracer) BuildTracingCtxForStart() *interop.TracingCtx {
	return nil
}
func (t *LocalStackTracer) BuildTracingCtxAfterInvokeComplete() *interop.TracingCtx {
	return nil
}

func NewLocalStackTracer() *LocalStackTracer {
	return &LocalStackTracer{}
}
