// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	intmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry/xray"
)

const (
	stateNoResponse  uint32 = 0
	stateGotResponse uint32 = 1
	stateGotError    uint32 = 2
)

type ErrorForInvoker interface {
	ReturnCode() int
	ErrorCategory() model.ErrorCategory
	ErrorType() model.ErrorType
	ErrorDetails() string
}

type InvokeResponseSender interface {
	SendRuntimeResponseHeaders(initData interop.InitStaticDataProvider, contentType, responseMode string)

	SendRuntimeResponseBody(ctx context.Context, runtimeResp RuntimeResponseRequest, functionTimeout time.Duration) SendResponseBodyResult

	SendRuntimeResponseTrailers(RuntimeResponseRequest)

	SendError(ErrorForInvoker, interop.InitStaticDataProvider)

	SendErrorTrailers(ErrorForInvoker, InvokeBodyResponseStatus)

	ErrorPayloadSizeBytes() int
}

type ResponderFactoryFunc func(context.Context, interop.InvokeRequest) InvokeResponseSender

type SendResponseBodyResult struct {
	Metrics interop.InvokeResponseMetrics
	Err     model.AppError
}

type runningInvokeImpl struct {
	timeoutCache timeoutCache

	cancelAsyncCtx       context.Context
	cancelAsyncCtxCancel context.CancelCauseFunc

	responseState       atomic.Uint32
	invokeSentChan      chan struct{}
	responseSentChan    chan model.AppError
	errorSentChan       chan model.AppError
	runtimeResponseChan chan RuntimeResponseRequest
	runtimeErrorChan    chan RuntimeErrorRequest

	invokeRespSender InvokeResponseSender
	runtimeNext      http.ResponseWriter

	responderFactoryFunc ResponderFactoryFunc
	sendInvokeToRuntime  func(context.Context, interop.InitStaticDataProvider, interop.InvokeRequest, http.ResponseWriter, string) (int64, time.Duration, time.Duration, model.AppError)
	createTracingData    func(traceId string, tracingMode intmodel.XrayTracingMode, segmentIDGenerator func() string) (downstreamTraceId string, tracingCtx *interop.TracingCtx)
}

func newRunningInvoke(
	runtimeNext http.ResponseWriter,
	responderFactoryFunc ResponderFactoryFunc,
	timeoutCache timeoutCache,
) runningInvokeImpl {
	ctx, cancel := context.WithCancelCause(context.Background())
	return runningInvokeImpl{
		timeoutCache:         timeoutCache,
		cancelAsyncCtx:       ctx,
		cancelAsyncCtxCancel: cancel,
		invokeSentChan:       make(chan struct{}),
		responseSentChan:     make(chan model.AppError, 1),
		errorSentChan:        make(chan model.AppError, 1),
		runtimeResponseChan:  make(chan RuntimeResponseRequest, 1),
		runtimeErrorChan:     make(chan RuntimeErrorRequest, 1),
		runtimeNext:          runtimeNext,

		responderFactoryFunc: responderFactoryFunc,
		sendInvokeToRuntime:  sendInvokeToRuntime,
		createTracingData:    xray.CreateTracingData,
	}
}

func (r *runningInvokeImpl) RunInvokeAndSendResult(ctx context.Context, initData interop.InitStaticDataProvider, invokeReq interop.InvokeRequest, metrics interop.InvokeMetrics) model.AppError {
	downstreamTraceId, tracingCtx := r.createTracingData(invokeReq.TraceId(), initData.XRayTracingMode(), xray.GenerateSegmentID)

	metrics.TriggerStartRequest()
	if err := metrics.SendInvokeStartEvent(tracingCtx); err != nil {
		logging.Error(ctx, "Failed to send InvokeStartEvent", "err", err)
	}

	r.invokeRespSender = r.responderFactoryFunc(ctx, invokeReq)

	ctx, cancel := r.getInvokeCtx(ctx, initData.FunctionTimeout())
	defer cancel()

	logging.Debug(ctx, "Sending Invoke to Runtime")
	written, requestPayloadReadDuration, requestPayloadWriteDuration, err := r.sendInvokeToRuntime(ctx, initData, invokeReq, r.runtimeNext, downstreamTraceId)
	if err != nil {
		logging.Err(ctx, "Failed to send Invoke to Runtime", err)

		r.cancelAsyncCtxCancel(err)
		if err.ErrorType() == model.ErrorSandboxTimedout {

			r.timeoutCache.Register(invokeReq.InvokeID())
		}
		r.invokeRespSender.SendError(err, initData)
		metrics.TriggerSentResponse(false, err, nil, r.invokeRespSender.ErrorPayloadSizeBytes())
		if err := metrics.SendInvokeFinishedEvent(tracingCtx, nil); err != nil {
			logging.Error(ctx, "Failed to send InvokeFinishedEvent", "err", err)
		}
		return err
	}

	logging.Debug(ctx, "Waiting for Runtime response")
	metrics.TriggerSentRequest(written, requestPayloadReadDuration, requestPayloadWriteDuration)
	close(r.invokeSentChan)

	runtimeAnswerSent := false
	var invokeResponseMetrics *interop.InvokeResponseMetrics
	var xrayErrorCause json.RawMessage
	select {
	case runtimeResp := <-r.runtimeResponseChan:
		metrics.TriggerGetResponse()
		logging.Debug(ctx, "Received Runtime response")
		if err = runtimeResp.ParsingError(); err != nil {
			r.responseState.Store(stateGotError)
			r.invokeRespSender.SendError(err, initData)
			break
		}
		runtimeAnswerSent, invokeResponseMetrics, err, xrayErrorCause = r.sendRuntimeResponse(ctx, initData, runtimeResp)
	case runtimeErr := <-r.runtimeErrorChan:
		metrics.TriggerGetResponse()
		err = runtimeErr.GetError()

		logging.Warn(ctx, "Received Runtime error", "err", err)
		runtimeAnswerSent = r.sendRuntimeError(initData, runtimeErr)
		xrayErrorCause = runtimeErr.GetXrayErrorCause()
	case <-ctx.Done():

		r.responseState.Store(stateGotError)
		err = BuildInvokeAppError(context.Cause(ctx), initData.FunctionTimeout())
		logging.Info(ctx, "Received ctx cancellation", "err", err)

		if err.ErrorType() == model.ErrorSandboxTimedout {

			r.timeoutCache.Register(invokeReq.InvokeID())
		}

		r.invokeRespSender.SendError(err, initData)
	}

	metrics.TriggerSentResponse(runtimeAnswerSent, err, invokeResponseMetrics, r.invokeRespSender.ErrorPayloadSizeBytes())
	if err := metrics.SendInvokeFinishedEvent(tracingCtx, xrayErrorCause); err != nil {
		logging.Error(ctx, "Failed to send InvokeFinishedEvent", "err", err)
	}

	logging.Debug(ctx, "Notify response and error channels we sent", "sent_err", err)
	r.responseSentChan <- err
	r.errorSentChan <- err
	return err
}

func (r *runningInvokeImpl) getInvokeCtx(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, invokeCtxCancel1 := context.WithCancelCause(ctx)
	go func() {

		<-r.cancelAsyncCtx.Done()
		invokeCtxCancel1(context.Cause(r.cancelAsyncCtx))
	}()

	ctx, invokeCtxCancel2 := context.WithTimeout(ctx, timeout)

	return ctx, func() {

		invokeCtxCancel2()
		invokeCtxCancel1(nil)
		r.cancelAsyncCtxCancel(nil)
	}
}

func (r *runningInvokeImpl) sendRuntimeResponse(ctx context.Context, initData interop.InitStaticDataProvider, runtimeResp RuntimeResponseRequest) (bool, *interop.InvokeResponseMetrics, model.AppError, json.RawMessage) {
	logging.Debug(ctx, "Sending Runtime response headers")
	r.invokeRespSender.SendRuntimeResponseHeaders(initData, runtimeResp.ContentType(), runtimeResp.ResponseMode())

	childCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	sendBodyRes := make(chan SendResponseBodyResult)
	logging.Debug(ctx, "Sending Runtime response body")
	go func() {
		sendBodyRes <- r.invokeRespSender.SendRuntimeResponseBody(childCtx, runtimeResp, initData.FunctionTimeout())
	}()

	select {

	case res := <-sendBodyRes:
		if res.Err != nil {
			logging.Err(ctx, "Failed sending body", res.Err)
			r.invokeRespSender.SendErrorTrailers(res.Err, buildEndOfResponse(res.Err))
			return false, &res.Metrics, res.Err, nil
		}

		trailerErr := runtimeResp.TrailerError()
		if trailerErr != nil {
			logging.Warn(ctx, "Runtime sent error trailers after response body", "err", trailerErr)
			r.invokeRespSender.SendErrorTrailers(trailerErr, InvokeBodyResponseComplete)

			return true, &res.Metrics, model.NewCustomerError(trailerErr.ErrorType()), nil
		}

		logging.Debug(ctx, "Sending response trailers")
		r.invokeRespSender.SendRuntimeResponseTrailers(runtimeResp)
		return true, &res.Metrics, nil, nil
	case runtimeErr := <-r.runtimeErrorChan:
		logging.Debug(ctx, "Received Runtime Error while sending response body")
		cancelFunc()
		res := <-sendBodyRes

		logging.Debug(ctx, "Sending error trailers")
		r.invokeRespSender.SendErrorTrailers(runtimeErr, InvokeBodyResponseTruncated)
		return true, &res.Metrics, runtimeErr.GetError(), runtimeErr.GetXrayErrorCause()
	}
}

func buildEndOfResponse(err model.AppError) InvokeBodyResponseStatus {
	if err == nil {
		return InvokeBodyResponseComplete
	}

	switch err.ErrorType() {
	case model.ErrorSandboxTimedout:
		return invokeBodyResponseTimeout
	case model.ErrorRuntimeTruncatedResponse:
		return InvokeBodyResponseTruncated
	case model.ErrorFunctionOversizedResponse:
		return invokeBodyResponseOversized
	default:
		slog.Error("Unrecognized error", "err", err)

		return InvokeBodyResponseTruncated
	}
}

func (r *runningInvokeImpl) sendRuntimeError(initData interop.InitStaticDataProvider, runtimeErr RuntimeErrorRequest) bool {
	slog.Debug("Sending Runtime error headers and trailers", "err", runtimeErr.GetError())
	r.invokeRespSender.SendRuntimeResponseHeaders(initData, "", "")
	r.invokeRespSender.SendErrorTrailers(runtimeErr, InvokeBodyResponseComplete)

	return true
}

func (r *runningInvokeImpl) RuntimeNextWait(ctx context.Context) model.AppError {
	select {
	case <-r.invokeSentChan:
		return nil
	case <-ctx.Done():
		err := context.Cause(ctx)
		logging.Warn(ctx, "Runtime Context expired", "err", err)

		return model.NewCustomerError(model.ErrorRuntimeUnknown, model.WithCause(err))
	case <-r.cancelAsyncCtx.Done():
		err := context.Cause(r.cancelAsyncCtx)
		logging.Warn(r.cancelAsyncCtx, "Aborting blocking /next's", "err", err)

		return model.NewCustomerError(model.ErrorRuntimeUnknown, model.WithCause(err))
	}
}

func (r *runningInvokeImpl) RuntimeResponse(ctx context.Context, runtimeRespReq RuntimeResponseRequest) model.AppError {
	logging.Debug(ctx, "Recevied runtime response")
	if !r.responseState.CompareAndSwap(stateNoResponse, stateGotResponse) {
		logging.Warn(ctx, "Invalid Invoke state : Response in progress")

		return model.NewCustomerError(model.ErrorRuntimeInvokeResponseInProgress)
	}

	r.runtimeResponseChan <- runtimeRespReq
	return <-r.responseSentChan
}

func (r *runningInvokeImpl) RuntimeError(ctx context.Context, runtimeErrReq RuntimeErrorRequest) model.AppError {
	oldState := r.responseState.Swap(stateGotError)
	if oldState == stateGotError {
		logging.Warn(ctx, "Invalid invoke state : error in progress")
		return model.NewCustomerError(model.ErrorRuntimeInvokeErrorInProgress)
	}

	r.runtimeErrorChan <- runtimeErrReq

	err := <-r.errorSentChan

	ctx = logging.WithFields(ctx, "err", err)

	logging.Debug(ctx, "Received Runtime error")

	if err == nil {
		logging.Warn(ctx, "Sent Runtime response instead of error")

		return model.NewCustomerError(model.ErrorRuntimeInvokeResponseWasSent)
	} else if runtimeErrReq.IsRuntimeError(err) {

		logging.Debug(ctx, "Error was successfully sent")
		return nil
	}

	logging.Warn(ctx, "Sent another error", "err", err)
	return err
}

func (r *runningInvokeImpl) CancelAsync(err model.AppError) {
	slog.Info("cancel invoke", "reason", err)
	r.cancelAsyncCtxCancel(err)
}
