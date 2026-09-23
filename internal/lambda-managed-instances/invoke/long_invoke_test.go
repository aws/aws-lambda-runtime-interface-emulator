// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
)

func newTestLongInvokerRouter(t *testing.T) *LongInvokerRouter {
	t.Helper()
	tc := newMockTimeoutCache(t)
	router := NewInvokeRouter(1, &telemetry.NoOpEventsAPI{}, tc)
	return NewLongInvokerRouter(router, func(_ context.Context, _ interop.InvokeRequest, _ http.ResponseWriter) InvokeResponseSender {
		return &MockInvokeResponseSender{}
	})
}

func TestLongPollInvoke_ReceivesResponse_WritesToWriter(t *testing.T) {
	t.Parallel()
	l := newTestLongInvokerRouter(t)
	w := httptest.NewRecorder()

	pi := newPendingInvoke("test-version", 5*time.Second, 50*time.Millisecond)

	rec := &ResponseWriterRecorder{statusCode: http.StatusOK, body: []byte("hello")}
	rec.header = http.Header{"Content-Type": []string{"application/json"}}
	go func() { pi.resultCh <- invokeResult{recorder: rec} }()

	res := l.longPollInvoke(context.Background(), "invoke-1", "test-version", w, pi.resultCh, pi.preemptCh.Load().(chan struct{}), NoopReconnectMetrics())

	assert.True(t, res.WasResponseSent)
	assert.NoError(t, res.Err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, interop.ReconnectOutcomeCompleted, res.Outcome)
	assert.Equal(t, "hello", w.Body.String())
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestLongPollInvoke_ContextTimeout_Returns202(t *testing.T) {
	t.Parallel()
	l := newTestLongInvokerRouter(t)
	w := httptest.NewRecorder()

	pi := newPendingInvoke("test-version", 5*time.Second, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	res := l.longPollInvoke(ctx, "invoke-1", "test-version", w, pi.resultCh, pi.preemptCh.Load().(chan struct{}), NoopReconnectMetrics())

	assert.True(t, res.WasResponseSent)
	assert.Equal(t, interop.ReconnectOutcomeTimeout, res.Outcome)
	assert.NoError(t, res.Err)
	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "invoke-1", w.Header().Get(headerInvokeID))
	assert.Equal(t, "test-version", w.Header().Get(headerFunctionVersionID))
	assert.Equal(t, string(WaitReasonStillRunning), w.Header().Get(headerWaitingReason))
}

func TestLongPollInvoke_ClosedChannel_ReturnsPendingExpired(t *testing.T) {
	t.Parallel()
	l := newTestLongInvokerRouter(t)
	w := httptest.NewRecorder()

	pi := newPendingInvoke("test-version", 5*time.Second, 50*time.Millisecond)
	close(pi.resultCh)

	res := l.longPollInvoke(context.Background(), "invoke-1", "test-version", w, pi.resultCh, pi.preemptCh.Load().(chan struct{}), NoopReconnectMetrics())

	assert.False(t, res.WasResponseSent)
	assert.Equal(t, interop.ReconnectOutcomeNotFound, res.Outcome)
	require.Error(t, res.Err)
	assert.Equal(t, model.ErrorInvalidInvokeId, res.Err.ErrorType())
}

func TestLongPollInvoke_InnerRouterError_ReturnsFalse(t *testing.T) {
	t.Parallel()
	l := newTestLongInvokerRouter(t)
	w := httptest.NewRecorder()

	pi := newPendingInvoke("test-version", 5*time.Second, 50*time.Millisecond)

	routerErr := model.NewClientError(ErrInvokeNoReadyRuntime, model.ErrorSeverityError, model.ErrorRuntimeUnavailable)
	go func() { pi.resultCh <- invokeResult{err: routerErr} }()

	res := l.longPollInvoke(context.Background(), "invoke-1", "test-version", w, pi.resultCh, pi.preemptCh.Load().(chan struct{}), NoopReconnectMetrics())

	assert.False(t, res.WasResponseSent)
	assert.Equal(t, interop.ReconnectOutcomeCompleted, res.Outcome)
	require.Error(t, res.Err)
	assert.Equal(t, model.ErrorRuntimeUnavailable, res.Err.ErrorType())
}

type longInvokeTestHarness struct {
	mocks          invokeRouterMocks
	innerRouter    *InvokeRouter
	longRouter     *LongInvokerRouter
	mockResponder  *MockInvokeResponseSender
	originalWriter *httptest.ResponseRecorder
	capturedWriter http.ResponseWriter
}

func newLongInvokeTestHarness(t *testing.T) *longInvokeTestHarness {
	t.Helper()
	h := &longInvokeTestHarness{
		mocks:          newInvokeRouterMocks(),
		originalWriter: httptest.NewRecorder(),
	}
	h.mockResponder = &MockInvokeResponseSender{}
	h.innerRouter = NewInvokeRouter(1, &telemetry.NoOpEventsAPI{}, h.mocks.timeoutCache)
	hijackInvokeRouterDeps(h.innerRouter, &h.mocks)
	h.longRouter = NewLongInvokerRouter(h.innerRouter, func(_ context.Context, _ interop.InvokeRequest, rw http.ResponseWriter) InvokeResponseSender {
		h.capturedWriter = rw
		return h.mockResponder
	})
	h.longRouter.LongInvokeThresholdMs = 0
	h.mocks.eaInvokeRequest.On("FunctionVersionID").Return("test-version")
	h.mocks.eaInvokeRequest.On("LongPollingConfig").Return(&interop.LongPollingConfig{
		ConnectionHoldTimeoutMs: 50,
		ResponseHoldTimeoutMs:   50,
	})
	return h
}

func (h *longInvokeTestHarness) prepareIdleRuntime(t *testing.T) {
	t.Helper()
	h.mocks.runnningInvoke.On("RuntimeNextWait", mock.Anything).Return(nil).Once()
	waiter, err := h.innerRouter.RuntimeNext(h.mocks.ctx, h.mocks.runtimeNextRequest)
	require.NoError(t, err)
	require.NoError(t, waiter.RuntimeNextWait(h.mocks.ctx))
}

func TestLongInvokerRouter_Invoke_NonLongInvoke_Passthrough(t *testing.T) {
	t.Parallel()
	h := newLongInvokeTestHarness(t)
	h.prepareIdleRuntime(t)

	h.mocks.eaInvokeRequest.On("ResolvedFunctionTimeoutMs").Return(int64(0))
	h.mocks.eaInvokeRequest.On("InvokeID").Return("normal-1")
	h.mocks.invokeMetrics.On("UpdateConcurrencyMetrics", mock.Anything, mock.Anything)
	h.mocks.invokeMetrics.On("SetInvokeMode", mock.Anything).Maybe()
	h.mocks.invokeMetrics.On("SetReservationUsed", false)
	h.mocks.runnningInvoke.On("RunInvokeAndSendResult", mock.Anything, &h.mocks.staticData, &h.mocks.eaInvokeRequest, mock.Anything, mock.Anything).Return(nil)

	err, sent, pending := h.longRouter.Invoke(h.mocks.ctx, &h.mocks.staticData, &h.mocks.eaInvokeRequest, &h.mocks.invokeMetrics, h.originalWriter)
	assert.NoError(t, err)
	assert.True(t, sent)
	assert.False(t, pending)
}

func TestLongInvokerRouter_Invoke_LongInvoke_HappyPath(t *testing.T) {
	t.Parallel()
	h := newLongInvokeTestHarness(t)
	h.prepareIdleRuntime(t)

	h.mocks.eaInvokeRequest.On("ResolvedFunctionTimeoutMs").Return(int64(900000))
	h.mocks.eaInvokeRequest.On("InvokeID").Return("long-1")
	h.mocks.invokeMetrics.On("UpdateConcurrencyMetrics", mock.Anything, mock.Anything)
	h.mocks.invokeMetrics.On("SetInvokeMode", mock.Anything).Maybe()
	h.mocks.invokeMetrics.On("SetReservationUsed", false)

	h.mocks.runnningInvoke.On("RunInvokeAndSendResult", mock.Anything, &h.mocks.staticData, &h.mocks.eaInvokeRequest, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			h.capturedWriter.Header().Set("Content-Type", "application/json")
			h.capturedWriter.WriteHeader(http.StatusOK)
			_, _ = h.capturedWriter.Write([]byte("hello"))
			sender := args.Get(4).(InvokeResponseSender)
			sender.SendRuntimeResponseTrailers(nil)
		}).Return(nil)
	h.mockResponder.On("SendRuntimeResponseTrailers", mock.Anything).Return()

	err, sent, pending := h.longRouter.Invoke(h.mocks.ctx, &h.mocks.staticData, &h.mocks.eaInvokeRequest, &h.mocks.invokeMetrics, h.originalWriter)
	assert.NoError(t, err)
	assert.True(t, sent)

	assert.Equal(t, http.StatusOK, h.originalWriter.Code)
	assert.Equal(t, "hello", h.originalWriter.Body.String())
	assert.Equal(t, "application/json", h.originalWriter.Header().Get("Content-Type"))
	assert.False(t, pending)
}

func TestLongInvokerRouter_Invoke_LongInvoke_202Timeout(t *testing.T) {
	t.Parallel()
	h := newLongInvokeTestHarness(t)
	h.prepareIdleRuntime(t)

	h.mocks.eaInvokeRequest.On("ResolvedFunctionTimeoutMs").Return(int64(900000))
	h.mocks.eaInvokeRequest.On("InvokeID").Return("long-timeout-1")
	h.mocks.invokeMetrics.On("UpdateConcurrencyMetrics", mock.Anything, mock.Anything)
	h.mocks.invokeMetrics.On("SetInvokeMode", mock.Anything).Maybe()
	h.mocks.invokeMetrics.On("SetReservationUsed", false)
	h.mocks.invokeMetrics.On("SetResponseWaitTime", mock.Anything).Maybe()
	h.mocks.invokeMetrics.On("SetResponseDeliveryLost").Maybe()
	h.mocks.invokeMetrics.On("SetResponseDeliverySent").Maybe()
	h.mocks.invokeMetrics.On("TriggerInvokeDone").Return(time.Duration(0), (*time.Duration)(nil), interop.InitStaticDataProvider(nil)).Maybe()
	h.mocks.invokeMetrics.On("SendMetrics", mock.Anything).Return(nil).Maybe()

	invokeStarted := make(chan struct{})
	invokeDone := make(chan struct{})
	h.mocks.runnningInvoke.On("RunInvokeAndSendResult", mock.Anything, &h.mocks.staticData, &h.mocks.eaInvokeRequest, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			close(invokeStarted)
			<-invokeDone
		}).Return(nil)

	err, sent, pending := h.longRouter.Invoke(h.mocks.ctx, &h.mocks.staticData, &h.mocks.eaInvokeRequest, &h.mocks.invokeMetrics, h.originalWriter)

	assert.NoError(t, err)
	assert.True(t, sent)
	assert.True(t, pending)
	assert.Equal(t, http.StatusAccepted, h.originalWriter.Code)

	<-invokeStarted
	close(invokeDone)

	time.Sleep(100 * time.Millisecond)
}

func TestLongInvokerRouter_Invoke_LongInvoke_DuplicateInvokeID(t *testing.T) {
	t.Parallel()
	h := newLongInvokeTestHarness(t)

	h.innerRouter = NewInvokeRouter(2, &telemetry.NoOpEventsAPI{}, h.mocks.timeoutCache)
	hijackInvokeRouterDeps(h.innerRouter, &h.mocks)
	h.longRouter = NewLongInvokerRouter(h.innerRouter, func(_ context.Context, _ interop.InvokeRequest, rw http.ResponseWriter) InvokeResponseSender {
		h.capturedWriter = rw
		return h.mockResponder
	})
	h.longRouter.LongInvokeThresholdMs = 0

	h.mocks.runnningInvoke.On("RuntimeNextWait", mock.Anything).Return(nil).Twice()
	w1, err := h.innerRouter.RuntimeNext(h.mocks.ctx, h.mocks.runtimeNextRequest)
	require.NoError(t, err)
	require.NoError(t, w1.RuntimeNextWait(h.mocks.ctx))
	w2, err := h.innerRouter.RuntimeNext(h.mocks.ctx, h.mocks.runtimeNextRequest)
	require.NoError(t, err)
	require.NoError(t, w2.RuntimeNextWait(h.mocks.ctx))

	h.mocks.eaInvokeRequest.On("ResolvedFunctionTimeoutMs").Return(int64(900001))
	h.mocks.eaInvokeRequest.On("InvokeID").Return("dup-1")
	h.mocks.invokeMetrics.On("UpdateConcurrencyMetrics", mock.Anything, mock.Anything)
	h.mocks.invokeMetrics.On("SetInvokeMode", mock.Anything).Maybe()
	h.mocks.invokeMetrics.On("SetReservationUsed", false)

	firstStarted := make(chan struct{})
	firstDone := make(chan struct{})
	h.mocks.runnningInvoke.On("RunInvokeAndSendResult", mock.Anything, &h.mocks.staticData, &h.mocks.eaInvokeRequest, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			close(firstStarted)
			<-firstDone
			h.capturedWriter.WriteHeader(http.StatusOK)
			sender := args.Get(4).(InvokeResponseSender)
			sender.SendRuntimeResponseTrailers(nil)
		}).Return(nil).Once()
	h.mockResponder.On("SendRuntimeResponseTrailers", mock.Anything).Return().Maybe()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _, _ = h.longRouter.Invoke(h.mocks.ctx, &h.mocks.staticData, &h.mocks.eaInvokeRequest, &h.mocks.invokeMetrics, h.originalWriter)
	}()
	<-firstStarted

	err2, sent2, pending2 := h.longRouter.Invoke(h.mocks.ctx, &h.mocks.staticData, &h.mocks.eaInvokeRequest, &h.mocks.invokeMetrics, h.originalWriter)
	assert.Error(t, err2)
	assert.False(t, sent2)
	assert.False(t, pending2)
	assert.Equal(t, model.ErrorDuplicatedInvokeId, err2.ErrorType())

	close(firstDone)
	wg.Wait()
}

func TestLongInvokerRouter_Invoke_LongInvoke_InnerRouterError(t *testing.T) {
	t.Parallel()
	h := newLongInvokeTestHarness(t)

	h.mocks.eaInvokeRequest.On("ResolvedFunctionTimeoutMs").Return(int64(900001))
	h.mocks.eaInvokeRequest.On("InvokeID").Return("err-1")
	h.mocks.invokeMetrics.On("UpdateConcurrencyMetrics", mock.Anything, mock.Anything)
	h.mocks.invokeMetrics.On("SetInvokeMode", mock.Anything)

	err, sent, pending := h.longRouter.Invoke(h.mocks.ctx, &h.mocks.staticData, &h.mocks.eaInvokeRequest, &h.mocks.invokeMetrics, h.originalWriter)

	assert.Error(t, err)
	assert.False(t, sent)
	assert.False(t, pending)
	assert.Equal(t, model.ErrorRuntimeUnavailable, err.ErrorType())
}

func TestSendInvokeResult_TTLExpiry_NoReader(t *testing.T) {
	t.Parallel()

	l := newTestLongInvokerRouter(t)

	pi := newPendingInvoke("test-version", 5*time.Second, 50*time.Millisecond)
	l.pendingInvokes.Set("ttl-1", pi)

	rec := ResponseWriterRecorder{statusCode: http.StatusOK, body: []byte("lost")}

	start := time.Now()
	l.sendInvokeResult("ttl-1", invokeResult{recorder: &rec})
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, 40*time.Millisecond)
	assert.Less(t, elapsed, 500*time.Millisecond)

	_, exists := l.pendingInvokes.Get("ttl-1")
	assert.False(t, exists)
}

func TestReconnect_UnknownInvokeID_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	l := newTestLongInvokerRouter(t)
	w := httptest.NewRecorder()

	res := l.Reconnect(context.Background(), "unknown-id", w, NoopReconnectMetrics())

	assert.False(t, res.WasResponseSent)
	require.Error(t, res.Err)
	assert.Equal(t, model.ErrorInvalidInvokeId, res.Err.ErrorType())
}

func TestReconnect_DeliversBufferedResponse(t *testing.T) {
	t.Parallel()
	l := newTestLongInvokerRouter(t)
	w := httptest.NewRecorder()

	pi := newPendingInvoke("test-version", 5*time.Second, 50*time.Millisecond)
	l.pendingInvokes.Set("invoke-1", pi)

	rec := &ResponseWriterRecorder{statusCode: http.StatusOK, body: []byte("buffered")}
	rec.header = http.Header{"Content-Type": []string{"application/json"}}
	go func() { pi.resultCh <- invokeResult{recorder: rec} }()

	res := l.Reconnect(context.Background(), "invoke-1", w, NoopReconnectMetrics())

	assert.True(t, res.WasResponseSent)
	assert.NoError(t, res.Err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "buffered", w.Body.String())
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestReconnect_HoldTimeout_Returns202(t *testing.T) {
	t.Parallel()
	l := newTestLongInvokerRouter(t)
	w := httptest.NewRecorder()

	pi := newPendingInvoke("test-version", 50*time.Millisecond, 10*time.Second)
	l.pendingInvokes.Set("invoke-1", pi)

	res := l.Reconnect(context.Background(), "invoke-1", w, NoopReconnectMetrics())

	assert.True(t, res.WasResponseSent)
	assert.NoError(t, res.Err)
	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestReconnect_ErrorResult_ReturnsError(t *testing.T) {
	t.Parallel()
	l := newTestLongInvokerRouter(t)
	w := httptest.NewRecorder()

	pi := newPendingInvoke("test-version", 5*time.Second, 50*time.Millisecond)
	l.pendingInvokes.Set("invoke-1", pi)

	routerErr := model.NewClientError(ErrInvokeNoReadyRuntime, model.ErrorSeverityError, model.ErrorRuntimeUnavailable)
	go func() { pi.resultCh <- invokeResult{err: routerErr} }()

	res := l.Reconnect(context.Background(), "invoke-1", w, NoopReconnectMetrics())

	assert.False(t, res.WasResponseSent)
	require.Error(t, res.Err)
	assert.Equal(t, model.ErrorRuntimeUnavailable, res.Err.ErrorType())
}

func TestReconnect_NewWins_PreemptsExistingPreempt(t *testing.T) {
	t.Parallel()
	l := newTestLongInvokerRouter(t)

	pi := newPendingInvoke("test-version", 5*time.Second, 50*time.Millisecond)
	l.pendingInvokes.Set("invoke-1", pi)

	wA := httptest.NewRecorder()
	resumeADone := make(chan struct{})
	go func() {
		l.Reconnect(context.Background(), "invoke-1", wA, NoopReconnectMetrics())
		close(resumeADone)
	}()

	time.Sleep(50 * time.Millisecond)

	wB := httptest.NewRecorder()
	rec := &ResponseWriterRecorder{statusCode: http.StatusOK, body: []byte("hello")}
	go func() {
		time.Sleep(50 * time.Millisecond)
		pi.resultCh <- invokeResult{recorder: rec}
	}()

	resB := l.Reconnect(context.Background(), "invoke-1", wB, NoopReconnectMetrics())

	<-resumeADone
	assert.Equal(t, http.StatusAccepted, wA.Code)

	assert.True(t, resB.WasResponseSent)
	assert.NoError(t, resB.Err)
	assert.Equal(t, http.StatusOK, wB.Code)
	assert.Equal(t, "hello", wB.Body.String())
}

func TestLongPollInvoke_Preempted_Returns202(t *testing.T) {
	t.Parallel()
	l := newTestLongInvokerRouter(t)
	w := httptest.NewRecorder()

	pi := newPendingInvoke("test-version", 5*time.Second, 50*time.Millisecond)
	preemptCh := pi.preemptCh.Load().(chan struct{})
	close(preemptCh)

	res := l.longPollInvoke(context.Background(), "invoke-1", "test-version", w, pi.resultCh, preemptCh, NoopReconnectMetrics())

	assert.True(t, res.WasResponseSent)
	assert.Equal(t, interop.ReconnectOutcomeDisplaced, res.Outcome)
	assert.NoError(t, res.Err)
	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "invoke-1", w.Header().Get(headerInvokeID))
	assert.Equal(t, "test-version", w.Header().Get(headerFunctionVersionID))
	assert.Equal(t, string(WaitReasonDisplaced), w.Header().Get(headerWaitingReason))
}

func TestSwapPreempt_ClosesOldChannel(t *testing.T) {
	t.Parallel()
	pi := newPendingInvoke("test-version", 5*time.Second, 50*time.Millisecond)
	oldCh := pi.preemptCh.Load().(chan struct{})

	pi.preempt()

	select {
	case <-oldCh:
	default:
		t.Fatal("old preemptCh should be closed")
	}
}

func TestSwapPreempt_ReturnsNewChannel(t *testing.T) {
	t.Parallel()
	pi := newPendingInvoke("test-version", 5*time.Second, 50*time.Millisecond)

	newCh := pi.preempt()

	select {
	case <-newCh:
		t.Fatal("new preemptCh should not be closed")
	default:
	}
}

func TestSwapPreempt_ConcurrentCallsNoRace(t *testing.T) {
	t.Parallel()
	pi := newPendingInvoke("test-version", 5*time.Second, 50*time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pi.preempt()
		}()
	}
	wg.Wait()
}

func TestGetActiveRuntimeCount(t *testing.T) {
	t.Parallel()

	tc := newMockTimeoutCache(t)
	router := NewInvokeRouter(10, &telemetry.NoOpEventsAPI{}, tc)
	l := NewLongInvokerRouter(router, func(_ context.Context, _ interop.InvokeRequest, _ http.ResponseWriter) InvokeResponseSender {
		return &MockInvokeResponseSender{}
	})

	assert.Equal(t, 0, l.GetActiveRuntimeCount())

	require.NoError(t, router.runtimePool.Add(newMockRunningInvoke(t)))
	require.NoError(t, router.runtimePool.Add(newMockRunningInvoke(t)))
	require.NoError(t, router.runtimePool.Add(newMockRunningInvoke(t)))

	assert.Equal(t, 3, l.GetActiveRuntimeCount())

	router.runningInvokes.Set("invoke-1", newMockRunningInvoke(t))

	assert.Equal(t, 4, l.GetActiveRuntimeCount())

	router.runningInvokes.Remove("invoke-1")

	assert.Equal(t, 3, l.GetActiveRuntimeCount())

	require.NoError(t, router.runtimePool.Add(newMockRunningInvoke(t)))

	assert.Equal(t, 4, l.GetActiveRuntimeCount())
}

func TestAbortRunningInvokes_DrainsWithLargeResponseHoldTimeout(t *testing.T) {
	t.Parallel()

	l := newTestLongInvokerRouter(t)

	pi := newPendingInvoke("test-version", 5*time.Second, 200*time.Millisecond)
	l.pendingInvokes.Set("large-timeout-1", pi)

	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		rec := &ResponseWriterRecorder{statusCode: http.StatusOK, body: []byte("result")}
		l.sendInvokeResult("large-timeout-1", invokeResult{recorder: rec})
	}()

	var shutdownMetrics interop.MockShutdownMetrics
	var durationMetric interop.MockDurationMetricTimer
	shutdownMetrics.On("CreateDurationMetric", interop.ShutdownAbortInvokesDurationMetric).Return(&durationMetric)
	durationMetric.On("Done").Return()

	start := time.Now()
	l.AbortRunningInvokes(&shutdownMetrics, nil)
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, 150*time.Millisecond)
	assert.Less(t, elapsed, 1*time.Second)

	_, exists := l.pendingInvokes.Get("large-timeout-1")
	assert.False(t, exists)
}
