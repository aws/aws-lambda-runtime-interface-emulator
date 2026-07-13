// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
)

const (
	testInvokeRouterMaxIdleRuntime = 5
)

type invokeRouterMocks struct {
	ctx                 context.Context
	staticData          interop.MockInitStaticDataProvider
	eaInvokeRequest     interop.MockInvokeRequest
	runnningInvoke      mockRunningInvoke
	runtimeRespReq      MockRuntimeResponseRequest
	runtimeErrorReq     MockRuntimeErrorRequest
	invokeMetrics       interop.MockInvokeMetrics
	shutdownMetrics     interop.MockShutdownMetrics
	durationMetricTimer *interop.MockDurationMetricTimer
	timeoutCache        *mockTimeoutCache

	runtimeNextRequest http.ResponseWriter
}

func newInvokeRouterMocks() invokeRouterMocks {
	return invokeRouterMocks{
		ctx:                 context.TODO(),
		staticData:          interop.MockInitStaticDataProvider{},
		eaInvokeRequest:     interop.MockInvokeRequest{},
		runnningInvoke:      mockRunningInvoke{},
		runtimeRespReq:      MockRuntimeResponseRequest{},
		runtimeErrorReq:     MockRuntimeErrorRequest{},
		invokeMetrics:       interop.MockInvokeMetrics{},
		shutdownMetrics:     interop.MockShutdownMetrics{},
		durationMetricTimer: &interop.MockDurationMetricTimer{},
		timeoutCache:        &mockTimeoutCache{},
	}
}

func hijackInvokeRouterDeps(router *InvokeRouter, mocks *invokeRouterMocks) {
	router.createRunningInvoke = func(runtimeNext http.ResponseWriter) runningInvoke {
		return &mocks.runnningInvoke
	}
}

func createMocksAndInitRouter() (*invokeRouterMocks, *InvokeRouter) {
	mocks := newInvokeRouterMocks()
	router := NewInvokeRouter(testInvokeRouterMaxIdleRuntime, &telemetry.NoOpEventsAPI{}, nil, mocks.timeoutCache)
	hijackInvokeRouterDeps(router, &mocks)

	return &mocks, router
}

func checkMockExpectations(t *testing.T, mocks *invokeRouterMocks) {
	mocks.staticData.AssertExpectations(t)
	mocks.eaInvokeRequest.AssertExpectations(t)
	mocks.runnningInvoke.AssertExpectations(t)
	mocks.runtimeRespReq.AssertExpectations(t)
	mocks.runtimeErrorReq.AssertExpectations(t)
	mocks.shutdownMetrics.AssertExpectations(t)
	mocks.durationMetricTimer.AssertExpectations(t)
	mocks.timeoutCache.AssertExpectations(t)
}

func TestInvokeSuccess(t *testing.T) {
	t.Parallel()

	mocks, router := createMocksAndInitRouter()
	defer checkMockExpectations(t, mocks)

	respChannel := make(chan time.Time)
	syncChan := make(chan struct{})

	mocks.runnningInvoke.On("RuntimeNextWait", mock.Anything).Return(nil).Once()
	runningInvoke, err := router.RuntimeNext(mocks.ctx, mocks.runtimeNextRequest)
	require.NoError(t, err)
	require.NoError(t, runningInvoke.RuntimeNextWait(mocks.ctx))

	mocks.invokeMetrics.On("UpdateConcurrencyMetrics", 0, 1)

	mocks.eaInvokeRequest.On("InvokeID").Return("123456")
	mocks.runnningInvoke.On("RunInvokeAndSendResult", mock.Anything, &mocks.staticData, &mocks.eaInvokeRequest, mock.Anything).Run(func(args mock.Arguments) {

		close(syncChan)

		<-respChannel
	}).Return(nil)

	mocks.invokeMetrics.On("SetReservationUsed", false)

	mocks.runtimeRespReq.On("InvokeID").Return("123456")
	mocks.runnningInvoke.On("RuntimeResponse", mock.Anything, &mocks.runtimeRespReq).Return(nil)

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		err, wasResponseSent := router.Invoke(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.invokeMetrics)
		assert.NoError(t, err)
		assert.True(t, wasResponseSent)
	}()

	<-syncChan

	err = router.RuntimeResponse(mocks.ctx, &mocks.runtimeRespReq)
	assert.NoError(t, err)

	close(respChannel)
	wg.Wait()

	checkMockExpectations(t, mocks)
}

func TestInvokeFailure_NoIdleRuntime(t *testing.T) {
	t.Parallel()

	mocks, router := createMocksAndInitRouter()
	defer checkMockExpectations(t, mocks)

	mocks.invokeMetrics.On("UpdateConcurrencyMetrics", 0, 0)

	mocks.eaInvokeRequest.On("InvokeID").Return("123456")

	err, wasResponseSent := router.Invoke(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.invokeMetrics)
	assert.Error(t, err)
	assert.False(t, wasResponseSent)
	assert.Equal(t, model.ErrorRuntimeUnavailable, err.ErrorType())
}

func TestInvokeFailure_DublicatedInvokeId(t *testing.T) {
	t.Parallel()

	mocks, router := createMocksAndInitRouter()
	defer checkMockExpectations(t, mocks)

	mocks.runnningInvoke.On("RuntimeNextWait", mock.Anything).Return(nil).Twice()
	runningInvoke, err := router.RuntimeNext(mocks.ctx, mocks.runtimeNextRequest)
	require.NoError(t, err)
	require.NoError(t, runningInvoke.RuntimeNextWait(mocks.ctx))

	runningInvoke, err = router.RuntimeNext(mocks.ctx, mocks.runtimeNextRequest)
	require.NoError(t, err)
	require.NoError(t, runningInvoke.RuntimeNextWait(mocks.ctx))

	respChannel := make(chan time.Time)

	mocks.invokeMetrics.On("UpdateConcurrencyMetrics", mock.AnythingOfType("int"), mock.AnythingOfType("int")).Twice()
	mocks.invokeMetrics.On("SetReservationUsed", mock.AnythingOfType("bool")).Maybe()

	mocks.eaInvokeRequest.On("InvokeID").Return("123456")
	mocks.runnningInvoke.On("RunInvokeAndSendResult", mock.Anything, &mocks.staticData, &mocks.eaInvokeRequest, mock.Anything).Return(nil).WaitUntil(respChannel).Once()

	wg := new(sync.WaitGroup)
	ch := make(chan model.AppError, 2)
	var wasResponseSentCnt atomic.Uint32

	wg.Add(1)
	go func() {
		defer wg.Done()
		err, wasResponseSent := router.Invoke(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.invokeMetrics)
		if wasResponseSent {
			wasResponseSentCnt.Add(1)
		}
		ch <- err
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err, wasResponseSent := router.Invoke(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.invokeMetrics)
		if wasResponseSent {
			wasResponseSentCnt.Add(1)
		}
		ch <- err
	}()

	err = <-ch
	assert.Error(t, err)
	assert.Equal(t, model.ErrorDuplicatedInvokeId, err.ErrorType())

	close(respChannel)
	err = <-ch
	assert.NoError(t, err)

	wg.Wait()

	assert.Equal(t, uint32(1), wasResponseSentCnt.Load())
	checkMockExpectations(t, mocks)
}

func TestRuntimeNextFailure_TooManyIdleInvokes(t *testing.T) {
	t.Parallel()

	mocks, router := createMocksAndInitRouter()
	defer checkMockExpectations(t, mocks)

	mocks.runnningInvoke.On("RuntimeNextWait", mock.Anything).Return(nil).Times(testInvokeRouterMaxIdleRuntime)
	for range testInvokeRouterMaxIdleRuntime {
		runningInvoke, err := router.RuntimeNext(mocks.ctx, mocks.runtimeNextRequest)
		require.NoError(t, err)
		assert.NoError(t, runningInvoke.RuntimeNextWait(mocks.ctx))
	}

	runningInvoke, err := router.RuntimeNext(mocks.ctx, mocks.runtimeNextRequest)
	require.Error(t, err)
	require.Nil(t, runningInvoke)
	assert.Equal(t, model.ErrorRuntimeTooManyIdleRuntimes, err.ErrorType())

	checkMockExpectations(t, mocks)
}

func TestRuntimeResponseFailure(t *testing.T) {
	tests := []struct {
		name           string
		inTimeoutCache bool
		wantErrorType  model.ErrorType
	}{
		{
			name:          "InvokeIdNotFound",
			wantErrorType: model.ErrorRuntimeInvalidInvokeId,
		},
		{
			name:           "InvokeTimeout",
			inTimeoutCache: true,
			wantErrorType:  model.ErrorRuntimeInvokeTimeout,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mocks, router := createMocksAndInitRouter()
			defer checkMockExpectations(t, mocks)
			mocks.runtimeRespReq.On("InvokeID").Return("123456").Twice()
			mocks.timeoutCache.On("Consume", "123456").Return(tt.inTimeoutCache).Once()

			err := router.RuntimeResponse(context.Background(), &mocks.runtimeRespReq)

			assert.Equal(t, tt.wantErrorType, err.ErrorType())
		})
	}
}

func TestRuntimeErrorFailure(t *testing.T) {
	tests := []struct {
		name           string
		inTimeoutCache bool
		wantErrorType  model.ErrorType
	}{
		{
			name:          "InvokeIdNotFound",
			wantErrorType: model.ErrorRuntimeInvalidInvokeId,
		},
		{
			name:           "InvokeTimeout",
			inTimeoutCache: true,
			wantErrorType:  model.ErrorRuntimeInvokeTimeout,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mocks, router := createMocksAndInitRouter()
			defer checkMockExpectations(t, mocks)
			mocks.runtimeErrorReq.On("InvokeID").Return("123456").Twice()
			mocks.timeoutCache.On("Consume", "123456").Return(tt.inTimeoutCache).Once()

			err := router.RuntimeError(mocks.ctx, &mocks.runtimeErrorReq)

			assert.Equal(t, tt.wantErrorType, err.ErrorType())
		})
	}
}

func TestAbortRunningInvokes(t *testing.T) {
	t.Parallel()

	mocks, router := createMocksAndInitRouter()
	defer checkMockExpectations(t, mocks)

	eaGracefulShutdownErr := model.NewClientError(nil, model.ErrorSeverityFatal, model.ErrorExecutionEnvironmentShutdown)
	mocks.shutdownMetrics.On("CreateDurationMetric", interop.ShutdownAbortInvokesDurationMetric).Return(mocks.durationMetricTimer)
	mocks.durationMetricTimer.On("Done").Return()

	idleRuntime1 := newMockRunningInvoke(t)
	idleRuntime2 := newMockRunningInvoke(t)

	mockRunningInvoke1 := newMockRunningInvoke(t)
	mockRunningInvoke2 := newMockRunningInvoke(t)

	require.NoError(t, router.runtimePool.Add(idleRuntime1))
	require.NoError(t, router.runtimePool.Add(idleRuntime2))

	mockRunningInvoke1.On("CancelAsync", eaGracefulShutdownErr).Return()
	mockRunningInvoke2.On("CancelAsync", eaGracefulShutdownErr).Return()

	router.runningInvokes.Set("1", mockRunningInvoke1)
	router.runningInvokes.Set("2", mockRunningInvoke2)

	router.AbortRunningInvokes(&mocks.shutdownMetrics, eaGracefulShutdownErr)

	mockRunningInvoke1.AssertExpectations(t)
	mockRunningInvoke2.AssertExpectations(t)

	idleRuntime1.AssertNumberOfCalls(t, "CancelAsync", 0)
	idleRuntime2.AssertNumberOfCalls(t, "CancelAsync", 0)

	mockRunningInvoke1.AssertNumberOfCalls(t, "CancelAsync", 1)
	mockRunningInvoke2.AssertNumberOfCalls(t, "CancelAsync", 1)
}

func TestInvokeRouter_Counters(t *testing.T) {
	t.Parallel()

	mocks, router := createMocksAndInitRouter()
	defer checkMockExpectations(t, mocks)

	idleRuntime1 := newMockRunningInvoke(t)
	idleRuntime2 := newMockRunningInvoke(t)

	mockRunningInvoke1 := newMockRunningInvoke(t)

	require.NoError(t, router.runtimePool.Add(idleRuntime1))
	require.NoError(t, router.runtimePool.Add(idleRuntime2))

	router.runningInvokes.Set("1", mockRunningInvoke1)

	assert.Equal(t, 2, router.GetRuntimePoolCounts().Idle)
	assert.Equal(t, 1, router.GetRunningInvokesCount())
}

func TestReserveIdleRuntime_Success(t *testing.T) {
	t.Parallel()

	mocks, router := createMocksAndInitRouter()

	mocks.runnningInvoke.On("RuntimeNextWait", mock.Anything).Return(nil).Once()
	_, err := router.RuntimeNext(mocks.ctx, mocks.runtimeNextRequest)
	require.NoError(t, err)

	resp, appErr := router.ReserveIdleRuntime(mocks.ctx, "reserve-success-1", 100*time.Millisecond)
	require.Nil(t, appErr)
	_, ok := resp.(interop.ReserveIdleRuntimeSuccessResponse)
	assert.True(t, ok, "expected ReserveIdleRuntimeSuccessResponse")
}

func TestReserveIdleRuntime_NoIdleRuntimes(t *testing.T) {
	t.Parallel()

	mocks, router := createMocksAndInitRouter()

	resp, appErr := router.ReserveIdleRuntime(mocks.ctx, "reserve-no-idle", 100*time.Millisecond)
	require.NotNil(t, appErr)
	failResp, ok := resp.(interop.ReserveIdleRuntimeFailureResponse)
	assert.True(t, ok, "expected ReserveIdleRuntimeFailureResponse")
	assert.Equal(t, model.ErrorRuntimeUnavailable, failResp.ErrorType)
	assert.Equal(t, model.ErrorRuntimeUnavailable, appErr.ErrorType())
}

func TestReserveIdleRuntime_DuplicateInvokeID(t *testing.T) {
	t.Parallel()

	mocks, router := createMocksAndInitRouter()

	mocks.runnningInvoke.On("RuntimeNextWait", mock.Anything).Return(nil).Twice()
	_, err := router.RuntimeNext(mocks.ctx, mocks.runtimeNextRequest)
	require.NoError(t, err)
	_, err = router.RuntimeNext(mocks.ctx, mocks.runtimeNextRequest)
	require.NoError(t, err)

	resp, appErr := router.ReserveIdleRuntime(mocks.ctx, "reserve-dup", 100*time.Millisecond)
	require.Nil(t, appErr)
	_, ok := resp.(interop.ReserveIdleRuntimeSuccessResponse)
	assert.True(t, ok, "expected ReserveIdleRuntimeSuccessResponse")

	resp, appErr = router.ReserveIdleRuntime(mocks.ctx, "reserve-dup", 100*time.Millisecond)
	require.NotNil(t, appErr)
	failResp, ok := resp.(interop.ReserveIdleRuntimeFailureResponse)
	assert.True(t, ok, "expected ReserveIdleRuntimeFailureResponse")
	assert.Equal(t, model.ErrorDuplicatedInvokeId, failResp.ErrorType)
	assert.Equal(t, model.ErrorDuplicatedInvokeId, appErr.ErrorType())
}

func TestReserveIdleRuntime_DuplicateAgainstRunningInvoke(t *testing.T) {
	t.Parallel()

	mocks, router := createMocksAndInitRouter()

	_, err := router.RuntimeNext(mocks.ctx, mocks.runtimeNextRequest)
	require.NoError(t, err)

	const inFlightID = "reserve-vs-running"
	router.runningInvokes.Set(inFlightID, &mocks.runnningInvoke)

	resp, appErr := router.ReserveIdleRuntime(mocks.ctx, inFlightID, 100*time.Millisecond)

	require.NotNil(t, appErr)
	failResp, ok := resp.(interop.ReserveIdleRuntimeFailureResponse)
	assert.True(t, ok, "expected ReserveIdleRuntimeFailureResponse")
	assert.Equal(t, model.ErrorDuplicatedInvokeId, failResp.ErrorType)
	assert.Equal(t, model.ErrorDuplicatedInvokeId, appErr.ErrorType())

	assert.Equal(t, 0, router.runtimePool.ReservedCount(), "no reservation should have been recorded")
	assert.Equal(t, 1, router.GetRuntimePoolCounts().Idle, "idle runtime should still be available")
}

func TestReserveIdleRuntime_Expiration(t *testing.T) {
	t.Parallel()

	mocks, router := createMocksAndInitRouter()

	mocks.runnningInvoke.On("RuntimeNextWait", mock.Anything).Return(nil).Once()
	_, err := router.RuntimeNext(mocks.ctx, mocks.runtimeNextRequest)
	require.NoError(t, err)

	_, appErr := router.ReserveIdleRuntime(mocks.ctx, "reserve-expire", 30*time.Millisecond)
	require.Nil(t, appErr)

	assert.Equal(t, 1, router.runtimePool.ReservedCount())

	time.Sleep(80 * time.Millisecond)

	assert.Equal(t, 0, router.runtimePool.ReservedCount())
	assert.Equal(t, 1, router.runtimePool.Counts().Total)
}

func TestReserveIdleRuntime_InvokeConsumesReservation(t *testing.T) {
	t.Parallel()

	mocks, router := createMocksAndInitRouter()

	mocks.runnningInvoke.On("RuntimeNextWait", mock.Anything).Return(nil).Once()
	_, err := router.RuntimeNext(mocks.ctx, mocks.runtimeNextRequest)
	require.NoError(t, err)

	_, appErr := router.ReserveIdleRuntime(mocks.ctx, "reserve-then-invoke", 500*time.Millisecond)
	require.Nil(t, appErr)

	mocks.invokeMetrics.On("UpdateConcurrencyMetrics", mock.AnythingOfType("int"), mock.AnythingOfType("int"))
	mocks.invokeMetrics.On("SetReservationUsed", true)
	mocks.eaInvokeRequest.On("InvokeID").Return("reserve-then-invoke")
	mocks.runnningInvoke.On("RunInvokeAndSendResult", mock.Anything, &mocks.staticData, &mocks.eaInvokeRequest, mock.Anything).Return(nil)

	invokeErr, wasResponseSent := router.Invoke(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.invokeMetrics)
	assert.NoError(t, invokeErr)
	assert.True(t, wasResponseSent)

	assert.Equal(t, 0, router.runtimePool.ReservedCount())
	assert.Equal(t, 0, router.runtimePool.Counts().Total)
}

func TestReserveIdleRuntime_InvokeWithoutReservation(t *testing.T) {
	t.Parallel()

	mocks, router := createMocksAndInitRouter()

	mocks.runnningInvoke.On("RuntimeNextWait", mock.Anything).Return(nil).Once()
	_, err := router.RuntimeNext(mocks.ctx, mocks.runtimeNextRequest)
	require.NoError(t, err)

	mocks.invokeMetrics.On("UpdateConcurrencyMetrics", mock.AnythingOfType("int"), mock.AnythingOfType("int"))
	mocks.invokeMetrics.On("SetReservationUsed", false)
	mocks.eaInvokeRequest.On("InvokeID").Return("no-reservation-invoke")
	mocks.runnningInvoke.On("RunInvokeAndSendResult", mock.Anything, &mocks.staticData, &mocks.eaInvokeRequest, mock.Anything).Return(nil)

	invokeErr, wasResponseSent := router.Invoke(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.invokeMetrics)
	assert.NoError(t, invokeErr)
	assert.True(t, wasResponseSent)
}

func TestReserveIdleRuntime_GetIdleRuntimesCount_ExcludesReserved(t *testing.T) {
	t.Parallel()

	mocks, router := createMocksAndInitRouter()

	mocks.runnningInvoke.On("RuntimeNextWait", mock.Anything).Return(nil).Twice()
	_, err := router.RuntimeNext(mocks.ctx, mocks.runtimeNextRequest)
	require.NoError(t, err)
	_, err = router.RuntimeNext(mocks.ctx, mocks.runtimeNextRequest)
	require.NoError(t, err)

	counts := router.GetRuntimePoolCounts()
	assert.Equal(t, 2, counts.Idle)
	assert.Equal(t, 0, counts.Reserved)

	_, appErr := router.ReserveIdleRuntime(mocks.ctx, "count-test", 500*time.Millisecond)
	require.Nil(t, appErr)

	counts = router.GetRuntimePoolCounts()
	assert.Equal(t, 1, counts.Idle)
	assert.Equal(t, 1, counts.Reserved)
	assert.Equal(t, 2, counts.Total)
}
