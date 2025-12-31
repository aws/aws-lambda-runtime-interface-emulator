// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"

	cmap "github.com/orcaman/concurrent-map"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

var (
	ErrInvokeIdAlreadyExists = errors.New("invoke ID already exists")
	ErrInvokeNoReadyRuntime  = errors.New("no idle runtimes")
)

type RuntimeResponseRequest interface {
	ParsingError() model.AppError

	InvokeID() interop.InvokeID
	ContentType() string
	ResponseMode() string

	BodyReader() io.Reader

	TrailerError() ErrorForInvoker
}

type RuntimeErrorRequest interface {
	InvokeID() interop.InvokeID
	ContentType() string
	ErrorType() model.ErrorType
	ErrorCategory() model.ErrorCategory
	GetError() model.AppError
	IsRuntimeError(model.AppError) bool
	ReturnCode() int
	ErrorDetails() string
	GetXrayErrorCause() json.RawMessage
}

type runningInvoke interface {
	RunInvokeAndSendResult(context.Context, interop.InitStaticDataProvider, interop.InvokeRequest, interop.InvokeMetrics) model.AppError
	RuntimeNextWait(context.Context) model.AppError
	RuntimeResponse(context.Context, RuntimeResponseRequest) model.AppError
	RuntimeError(context.Context, RuntimeErrorRequest) model.AppError
	CancelAsync(model.AppError)
}

type timeoutCache interface {
	Register(invokeID interop.InvokeID)
	Consume(invokeID interop.InvokeID) (consumed bool)
}

type InvokeRouter struct {
	eventsApi interop.EventsAPI

	idleRuntimes chan runningInvoke

	runningInvokes cmap.ConcurrentMap

	wg sync.WaitGroup

	createRunningInvoke func(http.ResponseWriter) runningInvoke

	timeoutCache timeoutCache
}

func NewInvokeRouter(
	maxIdleRuntimesQueueSize int,
	telemetryEventsApi interop.EventsAPI,
	responderFactoryFunc ResponderFactoryFunc,
	timeoutCache timeoutCache,
) *InvokeRouter {
	return &InvokeRouter{

		idleRuntimes:   make(chan runningInvoke, maxIdleRuntimesQueueSize),
		runningInvokes: cmap.New(),
		eventsApi:      telemetryEventsApi,
		timeoutCache:   timeoutCache,
		createRunningInvoke: func(runtimeNext http.ResponseWriter) runningInvoke {
			r := newRunningInvoke(runtimeNext, responderFactoryFunc, timeoutCache)
			return &r
		},
	}
}

func (ir *InvokeRouter) Invoke(ctx context.Context, initData interop.InitStaticDataProvider, invokeReq interop.InvokeRequest, metrics interop.InvokeMetrics) (err model.AppError, wasResponseSent bool) {
	logging.Debug(ctx, "InvokeRouter: received Invoke")
	ir.wg.Add(1)
	defer ir.wg.Done()

	var idleRuntime runningInvoke

	metrics.UpdateConcurrencyMetrics(ir.runningInvokes.Count(), len(ir.idleRuntimes))

	if !ir.runningInvokes.SetIfAbsent(invokeReq.InvokeID(), idleRuntime) {
		logging.Warn(ctx, "InvokeRouter error: duplicated invokeId")
		return model.NewClientError(ErrInvokeIdAlreadyExists, model.ErrorSeverityError, model.ErrorDublicatedInvokeId), false
	}

	defer ir.runningInvokes.Remove(invokeReq.InvokeID())

	select {
	case idleRuntime = <-ir.idleRuntimes:

		ir.runningInvokes.Set(invokeReq.InvokeID(), idleRuntime)
	default:
		logging.Warn(ctx, "InvokeRouter: no ready runtimes")
		return model.NewClientError(ErrInvokeNoReadyRuntime, model.ErrorSeverityError, model.ErrorRuntimeUnavailable), false
	}

	return idleRuntime.RunInvokeAndSendResult(ctx, initData, invokeReq, metrics), true
}

func (ir *InvokeRouter) RuntimeNext(ctx context.Context, runtimeReq http.ResponseWriter) (model.RuntimeNextWaiter, model.AppError) {
	logging.Debug(ctx, "InvokeRouter: received runtime /next")

	newRunningInvoke := ir.createRunningInvoke(runtimeReq)

	if err := ir.addIdleRuntimeToQueue(newRunningInvoke); err != nil {
		logging.Error(ctx, "InvokeRouter: failed to add idle runtime to the queue", "err", err)
		return nil, err
	}

	return newRunningInvoke, nil
}

func (ir *InvokeRouter) RuntimeResponse(ctx context.Context, runtimeRespReq RuntimeResponseRequest) model.AppError {
	logging.Debug(ctx, "InvokeRouter: received runtime response")

	invoke, ok := ir.runningInvokes.Get(runtimeRespReq.InvokeID())
	if !ok {
		if ir.timeoutCache.Consume(runtimeRespReq.InvokeID()) {
			logging.Warn(ctx, "InvokeRouter: response is too late for timed out invoke")
			return model.NewCustomerError(model.ErrorRuntimeInvokeTimeout)
		}
		logging.Warn(ctx, "InvokeRouter: invoke id not found")
		return model.NewCustomerError(model.ErrorRuntimeInvalidInvokeId)
	}

	return invoke.(runningInvoke).RuntimeResponse(ctx, runtimeRespReq)
}

func (ir *InvokeRouter) RuntimeError(ctx context.Context, runtimeErrReq RuntimeErrorRequest) model.AppError {
	invoke, ok := ir.runningInvokes.Get(runtimeErrReq.InvokeID())
	if !ok {
		if ir.timeoutCache.Consume(runtimeErrReq.InvokeID()) {
			logging.Warn(ctx, "InvokeRouter: error is too late for timed out invoke")
			return model.NewCustomerError(model.ErrorRuntimeInvokeTimeout)
		}
		logging.Warn(ctx, "InvokeRouter: invoke id not found")
		return model.NewCustomerError(model.ErrorRuntimeInvalidInvokeId)
	}

	logging.Warn(ctx, "InvokeRouter: received Runtime error", "err", runtimeErrReq.GetError())
	return invoke.(runningInvoke).RuntimeError(ctx, runtimeErrReq)
}

func (ir *InvokeRouter) AbortRunningInvokes(metrics interop.ShutdownMetrics, err model.AppError) {
	duration := metrics.CreateDurationMetric(interop.ShutdownAbortInvokesDurationMetric)
	defer duration.Done()

	slog.Info("InvokeRouter: Aborting running invokes", "reason", err)

	ir.runningInvokes.IterCb(func(key string, v interface{}) {
		if runningInvoke, ok := v.(runningInvoke); ok {
			runningInvoke.CancelAsync(err)
		}
	})

	slog.Debug("InvokeRouter: Waiting for invokes to be aborted")
	ir.wg.Wait()

}

func (ir *InvokeRouter) addIdleRuntimeToQueue(invoke runningInvoke) model.AppError {
	select {
	case ir.idleRuntimes <- invoke:
		return nil
	default:
		return model.NewCustomerError(model.ErrorRuntimeTooManyIdleRuntimes)
	}
}

func (ir *InvokeRouter) GetRunningInvokesCount() int {
	return ir.runningInvokes.Count()
}

func (ir *InvokeRouter) GetIdleRuntimesCount() int {
	return len(ir.idleRuntimes)
}
