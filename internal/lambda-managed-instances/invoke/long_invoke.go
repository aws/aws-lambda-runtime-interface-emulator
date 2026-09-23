// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	cmap "github.com/orcaman/concurrent-map"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

var (
	ErrInvokeSessionNotFound  = errors.New("invoke session not found")
	ErrPendingResponseExpired = errors.New("pending response expired")
)

const (
	DefaultPendingResponseTTL    = 10 * time.Second
	DefaultLongInvokeThresholdMs = int64(15 * time.Minute / time.Millisecond)

	maxShutdownDrainTimeout = 15 * time.Second

	headerInvokeID          = "invoke-id"
	headerFunctionVersionID = "invoked-function-version"
	headerWaitingReason     = "Invoke-Wait-Reason"
)

type WaitingReason string

const (
	WaitReasonStillRunning WaitingReason = "still_running"
	WaitReasonDisplaced    WaitingReason = "displaced"
)

type pendingInvoke struct {
	resultCh chan invokeResult

	preemptCh atomic.Value

	lastDisconnectTime atomic.Value

	functionVersionID string

	connectionHoldTimeout time.Duration

	responseHoldTimeout time.Duration
}

func newPendingInvoke(functionVersionID string, connectionHoldTimeout time.Duration, responseHoldTimeout time.Duration) *pendingInvoke {
	pi := &pendingInvoke{
		resultCh:              make(chan invokeResult),
		functionVersionID:     functionVersionID,
		connectionHoldTimeout: connectionHoldTimeout,
		responseHoldTimeout:   responseHoldTimeout,
	}
	pi.preemptCh.Store(make(chan struct{}))
	return pi
}

func (p *pendingInvoke) preempt() <-chan struct{} {
	newCh := make(chan struct{})
	old := p.preemptCh.Swap(newCh).(chan struct{})
	close(old)
	return newCh
}

func (r invokeResult) outcome(pollStart time.Time) interop.ReconnectOutcome {
	if !r.functionDoneTime.IsZero() && r.functionDoneTime.Before(pollStart) {
		return interop.ReconnectOutcomePendingHit
	}
	return interop.ReconnectOutcomeCompleted
}

func (p *pendingInvoke) recordDisconnectIfPending(outcome interop.ReconnectOutcome) bool {
	pending := outcome == interop.ReconnectOutcomeTimeout || outcome == interop.ReconnectOutcomeDisplaced
	if pending {
		p.lastDisconnectTime.Store(time.Now())
	}
	return pending
}

func (p *pendingInvoke) getLastDisconnectTime() *time.Time {
	if v := p.lastDisconnectTime.Load(); v != nil {
		t := v.(time.Time)
		return &t
	}
	return nil
}

type LongInvokerRouter struct {
	innerRouter      *InvokeRouter
	responderFactory ResponderFactoryFunc

	LongInvokeThresholdMs int64

	pendingInvokes cmap.ConcurrentMap

	wg sync.WaitGroup
}

func NewLongInvokerRouter(invokeRouter *InvokeRouter, responderFactory ResponderFactoryFunc) *LongInvokerRouter {
	return &LongInvokerRouter{
		innerRouter:           invokeRouter,
		responderFactory:      responderFactory,
		LongInvokeThresholdMs: DefaultLongInvokeThresholdMs,
		pendingInvokes:        cmap.New(),
	}
}

func (l *LongInvokerRouter) InnerRouter() *InvokeRouter { return l.innerRouter }

type invokeResult struct {
	recorder *ResponseWriterRecorder
	err      model.AppError
	metrics  interop.InvokeMetrics

	functionDoneTime time.Time
}

func newInvokeResult(recorder *ResponseWriterRecorder, err model.AppError, metrics interop.InvokeMetrics) invokeResult {
	return invokeResult{recorder: recorder, err: err, metrics: metrics, functionDoneTime: time.Now()}
}

func (l *LongInvokerRouter) Invoke(ctx context.Context, initData interop.InitStaticDataProvider, invokeReq interop.InvokeRequest, metrics interop.InvokeMetrics, responseWriter http.ResponseWriter) (err model.AppError, wasResponseSent bool, invokePending bool) {

	lpCfg := invokeReq.LongPollingConfig()
	isLongInvoke := lpCfg != nil && invokeReq.ResolvedFunctionTimeoutMs() > l.LongInvokeThresholdMs
	if !isLongInvoke {
		metrics.SetInvokeMode(string(InvokeModeNormal))
		directResponder := l.responderFactory(ctx, invokeReq, responseWriter)
		err, wasResponseSent := l.innerRouter.Invoke(ctx, initData, invokeReq, metrics, directResponder)
		return err, wasResponseSent, false
	}

	metrics.SetInvokeMode(string(InvokeModeLong))

	logging.Info(ctx, "LongInvokerRouter: starting long invoke")

	recorder := &ResponseWriterRecorder{}
	directResponder := l.responderFactory(ctx, invokeReq, recorder)

	connectionHoldTimeout := time.Duration(lpCfg.ConnectionHoldTimeoutMs) * time.Millisecond
	responseHoldTimeout := time.Duration(lpCfg.ResponseHoldTimeoutMs) * time.Millisecond

	pending := newPendingInvoke(invokeReq.FunctionVersionID(), connectionHoldTimeout, responseHoldTimeout)

	if !l.pendingInvokes.SetIfAbsent(invokeReq.InvokeID(), pending) {
		logging.Error(ctx, "LongInvokerRouter error: duplicated invokeId")
		return model.NewClientError(ErrInvokeIdAlreadyExists, model.ErrorSeverityError, model.ErrorDuplicatedInvokeId), false, false
	}

	bgCtx := context.WithoutCancel(ctx)
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		invokeErr, wasResponseSent := l.innerRouter.Invoke(bgCtx, initData, invokeReq, metrics, directResponder)
		if wasResponseSent {
			l.sendInvokeResult(invokeReq.InvokeID(), newInvokeResult(recorder, invokeErr, metrics))
		} else {
			l.sendInvokeResult(invokeReq.InvokeID(), newInvokeResult(nil, invokeErr, metrics))
		}
	}()

	pollCtx, pollCancel := context.WithTimeout(ctx, connectionHoldTimeout)
	defer pollCancel()

	pollResult := l.longPollInvoke(pollCtx, invokeReq.InvokeID(), pending.functionVersionID, responseWriter, pending.resultCh, pending.preemptCh.Load().(chan struct{}), NoopReconnectMetrics())
	invokePending = pending.recordDisconnectIfPending(pollResult.Outcome)
	return pollResult.Err, pollResult.WasResponseSent, invokePending
}

func (l *LongInvokerRouter) Reconnect(ctx context.Context, invokeID interop.InvokeID, responseWriter http.ResponseWriter, reconnectMetrics interop.ReconnectMetrics) interop.ReconnectResult {
	val, ok := l.pendingInvokes.Get(invokeID)
	if !ok {
		return interop.ReconnectResult{Outcome: interop.ReconnectOutcomeNotFound, Err: model.NewClientError(ErrInvokeSessionNotFound, model.ErrorSeverityInvalid, model.ErrorInvalidInvokeId)}
	}
	pending := val.(*pendingInvoke)

	reconnectMetrics.TriggerConnectionGap(pending.getLastDisconnectTime())

	preemptCh := pending.preempt()

	pollCtx, pollCancel := context.WithTimeout(ctx, pending.connectionHoldTimeout)
	defer pollCancel()

	result := l.longPollInvoke(pollCtx, invokeID, pending.functionVersionID, responseWriter, pending.resultCh, preemptCh, reconnectMetrics)
	pending.recordDisconnectIfPending(result.Outcome)

	return result
}

func (l *LongInvokerRouter) sendStatusAccepted(w http.ResponseWriter, invokeID interop.InvokeID, functionVersionID string, reason WaitingReason) {
	w.Header().Set(headerInvokeID, string(invokeID))
	w.Header().Set(headerFunctionVersionID, functionVersionID)
	w.Header().Set(headerWaitingReason, string(reason))
	w.WriteHeader(http.StatusAccepted)
}

func (l *LongInvokerRouter) longPollInvoke(ctx context.Context, invokeID interop.InvokeID, functionVersionID string, w http.ResponseWriter, resultCh <-chan invokeResult, preemptCh <-chan struct{}, reconnectMetrics interop.ReconnectMetrics) interop.ReconnectResult {
	pollStart := time.Now()
	reconnectMetrics.TriggerPollStart()
	select {
	case result := <-resultCh:
		reconnectMetrics.TriggerPollEnd()
		reconnectMetrics.SetFunctionDoneTime(result.functionDoneTime)
		if result.recorder == nil {
			if result.err != nil {

				return interop.ReconnectResult{InvokeMetrics: result.metrics, FunctionDoneTime: result.functionDoneTime, Outcome: interop.ReconnectOutcomeCompleted, Err: result.err}
			}
			return interop.ReconnectResult{Outcome: interop.ReconnectOutcomeNotFound, Err: model.NewClientError(ErrPendingResponseExpired, model.ErrorSeverityInvalid, model.ErrorInvalidInvokeId)}
		}
		logging.Info(ctx, "longPollInvoke: delivering recorded response", "invokeId", invokeID)
		reconnectMetrics.TriggerResponseReplayStart()
		if err := result.recorder.WriteTo(w); err != nil {
			reconnectMetrics.TriggerResponseReplayDone(result.recorder.BodySize())
			return interop.ReconnectResult{InvokeMetrics: result.metrics, FunctionDoneTime: result.functionDoneTime, Outcome: interop.ReconnectOutcomeError, Err: model.NewPlatformError(err, model.ErrorResponseReplayFailed), WasResponseSent: true}
		}
		reconnectMetrics.TriggerResponseReplayDone(result.recorder.BodySize())
		return interop.ReconnectResult{InvokeMetrics: result.metrics, FunctionDoneTime: result.functionDoneTime, Outcome: result.outcome(pollStart), Err: result.err, WasResponseSent: true}
	case <-preemptCh:
		reconnectMetrics.TriggerPollEnd()

		logging.Info(ctx, "longPollInvoke: preempted by new reconnect, returning 202", "invokeId", invokeID)
		l.sendStatusAccepted(w, invokeID, functionVersionID, WaitReasonDisplaced)
		return interop.ReconnectResult{Outcome: interop.ReconnectOutcomeDisplaced, WasResponseSent: true}
	case <-ctx.Done():
		reconnectMetrics.TriggerPollEnd()

		if errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
			logging.Info(ctx, "longPollInvoke: connection hold timeout, returning 202", "invokeId", invokeID)
			l.sendStatusAccepted(w, invokeID, functionVersionID, WaitReasonStillRunning)
			return interop.ReconnectResult{Outcome: interop.ReconnectOutcomeTimeout, WasResponseSent: true}
		}
		logging.Warn(ctx, "longPollInvoke: client disconnected", "invokeId", invokeID)
		return interop.ReconnectResult{Outcome: interop.ReconnectOutcomeTimeout}
	}
}

func (l *LongInvokerRouter) sendInvokeResult(invokeID interop.InvokeID, result invokeResult) {
	val, ok := l.pendingInvokes.Get(invokeID)
	if !ok {

		slog.Error("sendInvokeResult: invoke not found, possible duplicate completion or race with abort", "invokeId", invokeID)
		return
	}
	pending := val.(*pendingInvoke)

	select {
	case pending.resultCh <- result:

	case <-time.After(pending.responseHoldTimeout):
		slog.Error("sendInvokeResult: timed out waiting for poller, response lost", "invokeId", invokeID)
		if result.metrics != nil {
			result.metrics.SetResponseWaitTime(pending.responseHoldTimeout)
			result.metrics.SetResponseDeliveryLost()
			result.metrics.TriggerInvokeDone()
			if err := result.metrics.SendMetrics(result.err); err != nil {
				slog.Error("sendInvokeResult: failed to send metrics on TTL expiry", "invokeId", invokeID, "error", err)
			}
		}
	}

	l.pendingInvokes.RemoveCb(invokeID, func(key string, v interface{}, exists bool) bool {
		if exists {
			close(v.(*pendingInvoke).resultCh)
		}
		return true
	})
}

func (l *LongInvokerRouter) AbortRunningInvokes(metrics interop.ShutdownMetrics, err model.AppError) {

	l.innerRouter.AbortRunningInvokes(metrics, err)

	done := make(chan struct{})
	go func() { l.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(maxShutdownDrainTimeout):
		slog.Error("AbortRunningInvokes: safety cap reached, proceeding with shutdown",
			"timeout", maxShutdownDrainTimeout)
	}
}

func (l *LongInvokerRouter) GetActiveRuntimeCount() int {
	return l.innerRouter.GetRuntimePoolCounts().Total + l.innerRouter.GetRunningInvokesCount()
}
