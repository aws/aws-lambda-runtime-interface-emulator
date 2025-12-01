// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	intmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	rapimodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type runningInvokeMocks struct {
	ctx                   context.Context
	staticData            interop.MockInitStaticDataProvider
	eaInvokeRequest       interop.MockInvokeRequest
	eaInvokeResponder     MockInvokeResponseSender
	runtimeRespReq        MockRuntimeResponseRequest
	runtimeErrorReq       MockRuntimeErrorRequest
	timeoutCache          *mockTimeoutCache
	metrics               interop.MockInvokeMetrics
	errorPayloadSizeBytes int

	runtimeNextRequest http.ResponseWriter
}

func newRunningInvokeMocks(t *testing.T) runningInvokeMocks {
	return runningInvokeMocks{
		ctx:                   context.TODO(),
		staticData:            interop.MockInitStaticDataProvider{},
		eaInvokeRequest:       interop.MockInvokeRequest{},
		runtimeRespReq:        MockRuntimeResponseRequest{},
		runtimeErrorReq:       MockRuntimeErrorRequest{},
		timeoutCache:          newMockTimeoutCache(t),
		metrics:               interop.MockInvokeMetrics{},
		errorPayloadSizeBytes: 0,
	}
}

func hijackRunningInvokeDeps(ri *runningInvokeImpl, mocks *runningInvokeMocks) {
	ri.sendInvokeToRuntime = func(context.Context, interop.InitStaticDataProvider, interop.InvokeRequest, http.ResponseWriter, string) (int64, time.Duration, time.Duration, model.AppError) {
		return 0, 0, 0, nil
	}

	ri.createTracingData = func(string, intmodel.XrayTracingMode, func() string) (string, *interop.TracingCtx) {
		traceId := "Root=root1;Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535"
		tracingCtx := &interop.TracingCtx{
			SpanID: "",
			Type:   rapimodel.XRayTracingType,
			Value:  traceId,
		}
		return traceId, tracingCtx
	}
}

func createMocksAndInitRunningInvoke(t *testing.T) (*runningInvokeMocks, *runningInvokeImpl) {
	mocks := newRunningInvokeMocks(t)

	ri := newRunningInvoke(
		mocks.runtimeNextRequest,
		func(ctx context.Context, ir interop.InvokeRequest) InvokeResponseSender {
			return &mocks.eaInvokeResponder
		},
		mocks.timeoutCache,
	)
	hijackRunningInvokeDeps(&ri, &mocks)

	return &mocks, &ri
}

func mockMetrics(mocks *runningInvokeMocks, invokeRes interface{}) {
	mockMetricsBeforeResponse(mocks)
	mocks.metrics.On("TriggerGetResponse").Return()
	mocks.metrics.On("TriggerSentResponse", true, invokeRes, mock.Anything, mocks.errorPayloadSizeBytes).Return()
}

func mockMetricsAnswerNotFinished(mocks *runningInvokeMocks, invokeRes interface{}) {
	mockMetricsBeforeResponse(mocks)
	mocks.metrics.On("TriggerGetResponse").Return()
	mocks.metrics.On("TriggerSentResponse", false, invokeRes, mock.Anything, mocks.errorPayloadSizeBytes).Return()
}

func mockMetricsUnfinished(mocks *runningInvokeMocks, invokeRes interface{}) {
	mockMetricsBeforeResponse(mocks)
	mocks.metrics.On("TriggerSentResponse", false, invokeRes, mock.Anything, mocks.errorPayloadSizeBytes).Return()
}

func mockMetricsBeforeResponse(mocks *runningInvokeMocks) {
	mocks.metrics.On("TriggerStartRequest")
	mocks.metrics.On("SendInvokeStartEvent", mock.AnythingOfType("*interop.TracingCtx")).Return(nil)
	mocks.metrics.On("TriggerSentRequest", mock.AnythingOfType("int64"), mock.AnythingOfType("time.Duration"), mock.AnythingOfType("time.Duration")).Return()
	mocks.metrics.On("SendInvokeFinishedEvent", mock.AnythingOfType("*interop.TracingCtx"), mock.AnythingOfType("json.RawMessage")).Return(nil)
}

func checkRunningInvokeMockExpectations(t *testing.T, mocks *runningInvokeMocks) {
	mocks.staticData.AssertExpectations(t)
	mocks.eaInvokeRequest.AssertExpectations(t)
	mocks.eaInvokeResponder.AssertExpectations(t)
	mocks.runtimeRespReq.AssertExpectations(t)
	mocks.runtimeErrorReq.AssertExpectations(t)
	mocks.metrics.AssertExpectations(t)
}

func TestRunInvokeAndSendResultSuccess_RuntimeResponse(t *testing.T) {
	t.Parallel()

	mocks, runInvoke := createMocksAndInitRunningInvoke(t)

	mocks.staticData.On("FunctionTimeout").Return(time.Second)
	mocks.staticData.On("XRayTracingMode").Return(intmodel.XRayTracingModePassThrough)

	mocks.eaInvokeRequest.On("TraceId").Return("Root=root1;Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535")

	mocks.runtimeRespReq.On("ParsingError").Return(nil)
	mocks.runtimeRespReq.On("ContentType").Return("")
	mocks.runtimeRespReq.On("ResponseMode").Return("")
	mocks.runtimeRespReq.On("TrailerError").Return(nil)

	mocks.eaInvokeResponder.On("SendRuntimeResponseHeaders", &mocks.staticData, mock.Anything, mock.Anything).Return()
	mocks.eaInvokeResponder.On("SendRuntimeResponseBody", mock.Anything, &mocks.runtimeRespReq, mock.Anything).Return(SendResponseBodyResult{})
	mocks.eaInvokeResponder.On("SendRuntimeResponseTrailers", &mocks.runtimeRespReq).Return()
	mocks.eaInvokeResponder.On("ErrorPayloadSizeBytes").Return(mocks.errorPayloadSizeBytes)
	mockMetrics(mocks, nil)

	wg := new(sync.WaitGroup)
	wg.Add(1)

	go func() {
		defer wg.Done()
		err := runInvoke.RuntimeResponse(mocks.ctx, &mocks.runtimeRespReq)
		assert.NoError(t, err)
	}()

	err := runInvoke.RunInvokeAndSendResult(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.metrics)
	assert.NoError(t, err)

	wg.Wait()
	checkRunningInvokeMockExpectations(t, mocks)
}

func TestRunInvokeAndSendResultSuccess_RuntimeError(t *testing.T) {
	t.Parallel()

	mocks, runInvoke := createMocksAndInitRunningInvoke(t)
	err := model.NewCustomerError(model.ErrorFunctionUnknown)

	mocks.staticData.On("FunctionTimeout").Return(time.Second)
	mocks.staticData.On("XRayTracingMode").Return(intmodel.XRayTracingModePassThrough)

	mocks.eaInvokeRequest.On("TraceId").Return("Root=root1;Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535")
	mocks.errorPayloadSizeBytes = 100

	mocks.runtimeErrorReq.On("GetError").Return(err)
	mocks.runtimeErrorReq.On("GetXrayErrorCause").Return(json.RawMessage(nil))
	mocks.eaInvokeResponder.On("SendRuntimeResponseHeaders", &mocks.staticData, mock.Anything, mock.Anything).Return().Once()
	mocks.eaInvokeResponder.On("SendErrorTrailers", mock.Anything, InvokeBodyResponseComplete).Return().Once()
	mocks.eaInvokeResponder.On("ErrorPayloadSizeBytes").Return(mocks.errorPayloadSizeBytes)

	mocks.runtimeErrorReq.On("IsRuntimeError", err).Return(true)
	mockMetrics(mocks, err)

	wg := new(sync.WaitGroup)
	wg.Add(1)

	go func() {
		defer wg.Done()
		err := runInvoke.RuntimeError(mocks.ctx, &mocks.runtimeErrorReq)
		assert.NoError(t, err)
	}()

	invokeErr := runInvoke.RunInvokeAndSendResult(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.metrics)
	assert.Error(t, invokeErr)

	wg.Wait()
	checkRunningInvokeMockExpectations(t, mocks)
}

func TestRunInvokeAndSendResultSuccess_RuntimeTrailerError(t *testing.T) {
	t.Parallel()

	mocks, runInvoke := createMocksAndInitRunningInvoke(t)

	trailerErrorType := model.ErrorType("Function.Unknown")
	expectedTrailerErr := model.NewCustomerError(trailerErrorType)

	mocks.staticData.On("FunctionTimeout").Return(time.Second)
	mocks.staticData.On("XRayTracingMode").Return(intmodel.XRayTracingModePassThrough)

	mocks.eaInvokeRequest.On("TraceId").Return("Root=root1;Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535")

	mocks.runtimeRespReq.On("ParsingError").Return(nil)
	mocks.runtimeRespReq.On("ContentType").Return("")
	mocks.runtimeRespReq.On("ResponseMode").Return("")
	mocks.runtimeRespReq.On("TrailerError").Return(expectedTrailerErr, NewMockErrorForInvoker(t))

	mocks.eaInvokeResponder.On("SendRuntimeResponseHeaders", &mocks.staticData, mock.Anything, mock.Anything).Return()
	mocks.eaInvokeResponder.On("SendRuntimeResponseBody", mock.Anything, &mocks.runtimeRespReq, mock.Anything).Return(SendResponseBodyResult{})
	mocks.eaInvokeResponder.On("SendErrorTrailers", expectedTrailerErr, InvokeBodyResponseComplete).Return().Once()
	mocks.eaInvokeResponder.On("ErrorPayloadSizeBytes").Return(mocks.errorPayloadSizeBytes)
	mockMetrics(mocks, mock.MatchedBy(func(err model.AppError) bool {
		return err != nil && err.ErrorType() == trailerErrorType
	}))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := runInvoke.RuntimeResponse(mocks.ctx, &mocks.runtimeRespReq)
		assert.Error(t, err)
		assert.Equal(t, trailerErrorType, err.ErrorType())
	}()

	err := runInvoke.RunInvokeAndSendResult(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.metrics)
	assert.Error(t, err)
	assert.Equal(t, trailerErrorType, err.ErrorType())

	wg.Wait()
	checkRunningInvokeMockExpectations(t, mocks)
}

func TestRuntimeErrorFailure_SendInvokeToRuntime_Error(t *testing.T) {
	t.Parallel()

	mocks, runInvoke := createMocksAndInitRunningInvoke(t)

	err := model.NewCustomerError(model.ErrorRuntimeUnknown)
	runInvoke.sendInvokeToRuntime = func(context.Context, interop.InitStaticDataProvider, interop.InvokeRequest, http.ResponseWriter, string) (int64, time.Duration, time.Duration, model.AppError) {
		return 0, 0, 0, err
	}

	mocks.staticData.On("FunctionTimeout").Return(time.Second)
	mocks.staticData.On("XRayTracingMode").Return(intmodel.XRayTracingModePassThrough)

	mocks.eaInvokeRequest.On("TraceId").Return("Root=root1;Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535")
	mocks.eaInvokeResponder.On("SendError", err, &mocks.staticData, mock.Anything).Return()
	mocks.eaInvokeResponder.On("ErrorPayloadSizeBytes").Return(mocks.errorPayloadSizeBytes)

	mocks.metrics.On("TriggerStartRequest")
	mocks.metrics.On("SendInvokeStartEvent", mock.AnythingOfType("*interop.TracingCtx")).Return(nil)
	mocks.metrics.On("TriggerSentResponse", false, err, mock.Anything, 0).Return()
	mocks.metrics.On("SendInvokeFinishedEvent", mock.AnythingOfType("*interop.TracingCtx"), mock.AnythingOfType("json.RawMessage")).Return(nil)

	invokeErr := runInvoke.RunInvokeAndSendResult(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.metrics)
	assert.Error(t, invokeErr)

	checkRunningInvokeMockExpectations(t, mocks)
}

func TestRuntimeErrorFailure_SendInvokeToRuntime_Timeout(t *testing.T) {
	t.Parallel()

	mocks, runInvoke := createMocksAndInitRunningInvoke(t)

	invokeID := "invoke-0"
	mocks.timeoutCache.On("Register", invokeID)
	mocks.eaInvokeRequest.On("InvokeID").Return(invokeID)

	err := BuildInvokeAppError(context.DeadlineExceeded, time.Second)
	runInvoke.sendInvokeToRuntime = func(context.Context, interop.InitStaticDataProvider, interop.InvokeRequest, http.ResponseWriter, string) (int64, time.Duration, time.Duration, model.AppError) {
		return 0, 0, 0, err
	}

	mocks.staticData.On("FunctionTimeout").Return(time.Second)
	mocks.staticData.On("XRayTracingMode").Return(intmodel.XRayTracingModePassThrough)

	mocks.eaInvokeRequest.On("TraceId").Return("Root=root1;Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535")
	mocks.eaInvokeResponder.On("SendError", err, &mocks.staticData, mock.Anything).Return()
	mocks.eaInvokeResponder.On("ErrorPayloadSizeBytes").Return(mocks.errorPayloadSizeBytes)

	mocks.metrics.On("TriggerStartRequest")
	mocks.metrics.On("SendInvokeStartEvent", mock.AnythingOfType("*interop.TracingCtx")).Return(nil)
	mocks.metrics.On("TriggerSentResponse", false, err, mock.Anything, 0).Return()
	mocks.metrics.On("SendInvokeFinishedEvent", mock.AnythingOfType("*interop.TracingCtx"), mock.AnythingOfType("json.RawMessage")).Return(nil)

	invokeErr := runInvoke.RunInvokeAndSendResult(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.metrics)
	assert.Error(t, invokeErr)

	checkRunningInvokeMockExpectations(t, mocks)
}

func TestRunInvokeAndSendResultFailure_Timeout(t *testing.T) {
	t.Parallel()

	mocks, runInvoke := createMocksAndInitRunningInvoke(t)

	invokeID := "invoke-0"
	mocks.timeoutCache.On("Register", invokeID)
	mocks.eaInvokeRequest.On("InvokeID").Return(invokeID)

	mocks.staticData.On("FunctionTimeout").Return(time.Millisecond)
	mocks.staticData.On("XRayTracingMode").Return(intmodel.XRayTracingModePassThrough)

	mocks.eaInvokeRequest.On("TraceId").Return("Root=root1;Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535")
	mocks.eaInvokeResponder.On("SendError", mock.Anything, &mocks.staticData, mock.Anything).Return()
	mocks.eaInvokeResponder.On("ErrorPayloadSizeBytes").Return(mocks.errorPayloadSizeBytes)
	mockMetricsUnfinished(mocks, mock.Anything)

	invokeErr := runInvoke.RunInvokeAndSendResult(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.metrics)
	assert.Error(t, invokeErr)

	checkRunningInvokeMockExpectations(t, mocks)
}

func TestRunInvokeAndSendResultFailure_TimeoutWhileResponse(t *testing.T) {
	t.Parallel()

	mocks, runInvoke := createMocksAndInitRunningInvoke(t)
	timeoutErr := model.NewCustomerError(model.ErrorSandboxTimedout)

	mocks.staticData.On("FunctionTimeout").Return(time.Second)
	mocks.staticData.On("XRayTracingMode").Return(intmodel.XRayTracingModePassThrough)

	mocks.eaInvokeRequest.On("TraceId").Return("Root=root1;Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535")

	mocks.runtimeRespReq.On("ParsingError").Return(nil)
	mocks.runtimeRespReq.On("ContentType").Return("")
	mocks.runtimeRespReq.On("ResponseMode").Return("")

	mocks.eaInvokeResponder.On("SendRuntimeResponseHeaders", &mocks.staticData, mock.Anything, mock.Anything).Return()
	mocks.eaInvokeResponder.On("SendRuntimeResponseBody", mock.Anything, &mocks.runtimeRespReq, mock.Anything).Return(SendResponseBodyResult{Err: timeoutErr})
	mocks.eaInvokeResponder.On("SendErrorTrailers", mock.Anything, invokeBodyResponseTimeout).Return()
	mocks.eaInvokeResponder.On("ErrorPayloadSizeBytes").Return(mocks.errorPayloadSizeBytes)
	mockMetricsAnswerNotFinished(mocks, mock.Anything)

	wg := new(sync.WaitGroup)
	wg.Add(1)

	go func() {
		defer wg.Done()
		err := runInvoke.RuntimeResponse(mocks.ctx, &mocks.runtimeRespReq)
		assert.Error(t, err)
	}()

	err := runInvoke.RunInvokeAndSendResult(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.metrics)
	assert.Error(t, err)

	wg.Wait()

	checkRunningInvokeMockExpectations(t, mocks)
}

func TestRunInvokeAndSendResultFailure_ContextCancelled(t *testing.T) {
	t.Parallel()

	mocks, runInvoke := createMocksAndInitRunningInvoke(t)
	err := model.NewPlatformError(nil, "test fatal error")

	mocks.staticData.On("FunctionTimeout").Return(time.Second)
	mocks.staticData.On("XRayTracingMode").Return(intmodel.XRayTracingModePassThrough)

	mocks.eaInvokeRequest.On("TraceId").Return("Root=root1;Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535")
	mocks.eaInvokeResponder.On("SendError", err, &mocks.staticData, mock.Anything).Return(nil)
	mocks.eaInvokeResponder.On("ErrorPayloadSizeBytes").Return(mocks.errorPayloadSizeBytes)
	mockMetricsUnfinished(mocks, mock.Anything)

	ch := make(chan struct{})

	go func() {
		<-ch
		runInvoke.CancelAsync(err)
	}()

	close(ch)
	invokeErr := runInvoke.RunInvokeAndSendResult(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.metrics)
	assert.Error(t, invokeErr)

	checkRunningInvokeMockExpectations(t, mocks)
}

func TestRuntimeResponseFailure_ResponseWhileResponse(t *testing.T) {
	t.Parallel()

	mocks, runInvoke := createMocksAndInitRunningInvoke(t)
	syncChan := make(chan time.Time)

	mocks.staticData.On("FunctionTimeout").Return(5 * time.Second)
	mocks.staticData.On("XRayTracingMode").Return(intmodel.XRayTracingModePassThrough)

	mocks.eaInvokeRequest.On("TraceId").Return("Root=root1;Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535")

	mocks.runtimeRespReq.On("ParsingError").Return(nil)
	mocks.runtimeRespReq.On("ContentType").Return("")
	mocks.runtimeRespReq.On("ResponseMode").Return("")
	mocks.runtimeRespReq.On("TrailerError").Return(nil)

	mocks.eaInvokeResponder.On("SendRuntimeResponseHeaders", &mocks.staticData, mock.Anything, mock.Anything).Return().Once()
	mocks.eaInvokeResponder.On("SendRuntimeResponseBody", mock.Anything, &mocks.runtimeRespReq, mock.Anything).Return(SendResponseBodyResult{}).WaitUntil(syncChan).Once()
	mocks.eaInvokeResponder.On("SendRuntimeResponseTrailers", &mocks.runtimeRespReq).Return().Once()
	mocks.eaInvokeResponder.On("ErrorPayloadSizeBytes").Return(mocks.errorPayloadSizeBytes)
	mockMetrics(mocks, nil)

	wg := new(sync.WaitGroup)
	ch := make(chan model.AppError, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		ch <- runInvoke.RuntimeResponse(mocks.ctx, &mocks.runtimeRespReq)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ch <- runInvoke.RuntimeResponse(mocks.ctx, &mocks.runtimeRespReq)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := runInvoke.RunInvokeAndSendResult(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.metrics)
		assert.NoError(t, err)
	}()

	err := <-ch
	assert.Equal(t, model.ErrorRuntimeInvokeResponseInProgress, err.ErrorType())

	close(syncChan)
	err = <-ch
	assert.NoError(t, err)

	wg.Wait()
	checkRunningInvokeMockExpectations(t, mocks)
}

func TestRuntimeErrorFailure_ErrorWhileError(t *testing.T) {
	t.Parallel()

	mocks, runInvoke := createMocksAndInitRunningInvoke(t)
	defer checkRunningInvokeMockExpectations(t, mocks)

	err := model.NewCustomerError(model.ErrorFunctionUnknown)

	mocks.staticData.On("FunctionTimeout").Return(time.Second)
	mocks.staticData.On("XRayTracingMode").Return(intmodel.XRayTracingModePassThrough)

	mocks.eaInvokeRequest.On("TraceId").Return("Root=root1;Parent=parent1;Sampled=1;Lineage=foo:1|bar:65535")

	mocks.errorPayloadSizeBytes = 100

	mocks.runtimeErrorReq.On("GetError").Return(err)
	mocks.runtimeErrorReq.On("GetXrayErrorCause").Return(json.RawMessage(nil))

	mocks.eaInvokeResponder.On("SendRuntimeResponseHeaders", &mocks.staticData, mock.Anything, mock.Anything).Return().Once()
	mocks.eaInvokeResponder.On("SendErrorTrailers", mock.Anything, InvokeBodyResponseComplete).Return().Once()
	mocks.eaInvokeResponder.On("ErrorPayloadSizeBytes").Return(mocks.errorPayloadSizeBytes)

	mocks.runtimeErrorReq.On("IsRuntimeError", err).Return(true)
	mockMetrics(mocks, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := runInvoke.RunInvokeAndSendResult(mocks.ctx, &mocks.staticData, &mocks.eaInvokeRequest, &mocks.metrics)
		assert.Error(t, err)
		assert.Equal(t, model.ErrorFunctionUnknown, err.ErrorType())
	}()

	assert.NoError(t, runInvoke.RuntimeError(mocks.ctx, &mocks.runtimeErrorReq))

	runtimeErrorErr := runInvoke.RuntimeError(mocks.ctx, &mocks.runtimeErrorReq)
	assert.Error(t, runtimeErrorErr)
	assert.Equal(t, model.ErrorRuntimeInvokeErrorInProgress, runtimeErrorErr.ErrorType())

	customerErr := runInvoke.RuntimeError(mocks.ctx, &mocks.runtimeErrorReq)
	assert.Error(t, customerErr)
	assert.Equal(t, model.ErrorRuntimeInvokeErrorInProgress, customerErr.ErrorType())
}
