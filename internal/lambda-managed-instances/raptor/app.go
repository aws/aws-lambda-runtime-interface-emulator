// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package raptor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	internalModel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/raptor/internal"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/servicelogs"
)

var (
	ErrNotInitialized         = errors.New("sandbox is not initialized")
	ErrorEnvironmentUnhealthy = errors.New("environment is unhealthy")
)

type App struct {
	rapidCtx     interop.RapidContext
	state        *internal.StateGuard
	shutdownOnce sync.Once

	err                   atomic.Value
	doneCh                chan struct{}
	telemetryFDSocketPath string
	raptorLogger          raptorLogger
}

func StartApp(deps rapid.Dependencies, telemetryFDSocketPath string, raptorLogger raptorLogger) (*App, error) {
	ctx := context.Background()
	rapidCtx, err := rapid.Start(ctx, deps)
	if err != nil {
		return nil, err
	}

	app := &App{
		rapidCtx:              rapidCtx,
		state:                 internal.NewStateGuard(),
		doneCh:                make(chan struct{}),
		telemetryFDSocketPath: telemetryFDSocketPath,
		raptorLogger:          raptorLogger,
	}

	app.StartProcessTerminationMonitor()

	return app, nil
}

func (a *App) Init(ctx context.Context, init *internalModel.InitRequestMessage, initMetrics interop.InitMetrics) model.AppError {

	if err := a.state.SetState(internal.Initializing); err != nil {
		logging.Error(ctx, "State error : can't switch to initializing", "state", a.state.GetState(), "err", err)
		return interop.ClientError{
			ClientError: model.NewClientError(
				err,
				model.ErrorSeverityFatal,
				model.ErrorInvalidRequest,
			),
		}
	}

	initMessage := getInitExecutionData(init, a.RuntimeAPIAddrPort().String(), a.telemetryFDSocketPath)
	a.raptorLogger.SetInitData(&initMessage)
	logging.Debug(ctx, "Start handling Init", "initRequest", init)
	initErr := a.rapidCtx.HandleInit(ctx, initMessage, initMetrics)

	if initErr != nil {
		logging.Err(ctx, "Received Init error", initErr)
		a.Shutdown(initErr)

		return initErr
	}

	logging.Debug(ctx, "Received Init Success")

	err := a.state.SetState(internal.Initialized)
	if err != nil {
		logging.Error(ctx, "State error : can't switch to initalized state")

		return model.NewClientError(err, model.ErrorSeverityFatal, model.ErrorInvalidRequest)
	}

	return nil
}

func (a *App) Invoke(ctx context.Context, invokeMsg interop.InvokeRequest, metrics interop.InvokeMetrics) (err model.AppError, wasResponseSent bool) {
	currState := a.state.GetState()
	switch currState {
	case internal.Initialized:
		return a.rapidCtx.HandleInvoke(ctx, invokeMsg, metrics)
	case internal.Idle, internal.Initializing:
		logging.Error(ctx, "Sandbox not Initialized", "state", currState)
		return interop.ClientError{
			ClientError: model.NewClientError(
				ErrNotInitialized,
				model.ErrorSeverityError,
				model.ErrorInitIncomplete,
			),
		}, false
	case internal.ShuttingDown, internal.Shutdown:
		logging.Error(ctx, "Invoke while Sandbox shutting down")
		return interop.ClientError{
			ClientError: model.NewClientError(
				ErrorEnvironmentUnhealthy,
				model.ErrorSeverityFatal,
				model.ErrorEnvironmentUnhealthy,
			),
		}, false
	default:
		panic(fmt.Sprintf("unknown current state: %d", currState))
	}
}

func (a *App) Shutdown(shutdownReason model.AppError) {

	a.shutdownOnce.Do(func() {

		if err := a.state.SetState(internal.ShuttingDown); err != nil {

			return
		}

		metrics := rapid.NewShutdownMetrics(a.raptorLogger, shutdownReason)
		shutdownDuration := metrics.CreateDurationMetric(interop.TotalDurationMetric)

		slog.Info("Shutting down", "reason", shutdownReason)

		if shutdownReason != nil {
			a.err.Store(shutdownReason)
		}

		var shutdownErr model.AppError
		if shutdownErr = a.rapidCtx.HandleShutdown(shutdownReason, metrics); shutdownErr != nil {
			slog.Warn("Shutdown error", "err", shutdownErr)
		}

		shutdownDuration.Done()
		metrics.SendMetrics(shutdownErr)

		if err := a.state.SetState(internal.Shutdown); err != nil {
			slog.Error("could not change status from ShuttingDown to Shutdown", "status", a.state.GetState())
		}
		close(a.doneCh)
	})
}

func (a *App) RuntimeAPIAddrPort() netip.AddrPort {
	return a.rapidCtx.RuntimeAPIAddrPort()
}

func (a *App) StartProcessTerminationMonitor() {

	go func() {

		appErr := <-a.rapidCtx.ProcessTerminationNotifier()

		slog.Debug("Process termination monitor received", "error", appErr)
		a.Shutdown(appErr)
	}()
}

func (a *App) HandleHealthCheck() interop.HealthCheckResponse {
	currState := a.state.GetState()

	switch currState {

	case internal.Idle, internal.Initializing, internal.Initialized:
		return interop.HealthyContainerResponse{}
	case internal.ShuttingDown, internal.Shutdown:
		if appError := a.Err(); appError != nil {
			return interop.UnhealthyContainerResponse{
				ErrorType: appError.ErrorType(),
			}
		}
		return interop.UnhealthyContainerResponse{
			ErrorType: "",
		}
	default:
		panic(fmt.Sprintf("unknown current state: %d", currState))
	}
}

func (a *App) Done() <-chan struct{} {
	return a.doneCh
}

func (a *App) Err() model.AppError {
	err := a.err.Load()
	if err != nil {
		return err.(model.AppError)
	}
	return nil
}

type raptorLogger interface {
	servicelogs.Logger
	SetInitData(initData interop.InitStaticDataProvider)
}
