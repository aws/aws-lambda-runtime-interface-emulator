// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"context"
	"log/slog"
	"net/http"
	"net/netip"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lmds"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/invoke"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi"
	rapimodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/servicelogs"
	supvmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/supervisor/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils"
)

const RuntimePoolSize = 10_000

type Dependencies struct {
	InteropServer            interop.Server
	TelemetrySubscriptionAPI telemetry.SubscriptionAPI
	LogsEgressAPI            telemetry.StdLogsEgressAPI
	EventsAPI                interop.EventsAPI
	Supervisor               supvmodel.ProcessSupervisor
	FileUtils                utils.FileUtil

	InvokeRouter    *invoke.LongInvokerRouter
	MetadataService *lmds.Service

	RuntimeAPIAddrPort netip.AddrPort
}

func Start(ctx context.Context, deps Dependencies) (interop.RapidContext, error) {

	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow)
	renderingService := rendering.NewRenderingService()

	server, err := rapi.NewServer(deps.RuntimeAPIAddrPort, appCtx, registrationService, renderingService, deps.TelemetrySubscriptionAPI, deps.InvokeRouter.InnerRouter(), deps.MetadataService)
	if err != nil {
		return nil, err
	}

	appctx.StoreInteropServer(appCtx, deps.InteropServer)

	execCtx := &rapidContext{

		server:              server,
		appCtx:              appCtx,
		initFlow:            initFlow,
		registrationService: registrationService,
		renderingService:    renderingService,
		shutdownContext:     newShutdownContext(),
		fileUtils:           deps.FileUtils,

		invokeRouter:    deps.InvokeRouter,
		processTermChan: make(chan model.AppError),

		telemetrySubscriptionAPI: deps.TelemetrySubscriptionAPI,
		logsEgressAPI:            deps.LogsEgressAPI,
		interopServer:            deps.InteropServer,
		eventsAPI:                deps.EventsAPI,
		supervisor: processSupervisor{
			ProcessSupervisor: deps.Supervisor,
		},
	}

	go func() {

		if err := execCtx.server.Serve(ctx); err != nil {
			slog.Error("Server error", "err", err)
		}

	}()

	return execCtx, nil
}

func (r *rapidContext) HandleInit(ctx context.Context, initData interop.InitExecutionData, initMetrics interop.InitMetrics) model.AppError {
	r.initExecutionData = initData
	r.initMetrics = initMetrics
	return handleInit(ctx, r)
}

func (r *rapidContext) HandleInvoke(ctx context.Context, invokeReq interop.InvokeRequest, metrics interop.InvokeMetrics, responseWriter http.ResponseWriter) (err model.AppError, wasResponseSent bool, invokePending bool) {
	if err := invokeReq.UpdateFromInitData(&r.initExecutionData); err != nil {
		return err, false, false
	}
	metrics.AttachDependencies(&r.initExecutionData, r.eventsAPI)
	return r.invokeRouter.Invoke(ctx, &r.initExecutionData, invokeReq, metrics, responseWriter)
}

func (r *rapidContext) HandleReconnect(ctx context.Context, invokeID interop.InvokeID, responseWriter http.ResponseWriter, metrics interop.ReconnectMetrics) interop.ReconnectResult {
	return r.invokeRouter.Reconnect(ctx, invokeID, responseWriter, metrics)
}

func (r *rapidContext) HandleShutdown(shutdownCause model.AppError, metrics interop.ShutdownMetrics) model.AppError {
	metrics.SetAgentCount(len(r.registrationService.GetInternalAgents()), len(r.registrationService.GetExternalAgents()))

	metrics.AddMetric(servicelogs.Counter(interop.RuntimeNextCountMetric, float64(r.invokeRouter.GetActiveRuntimeCount())))
	metrics.AddMetric(servicelogs.Counter(interop.RuntimeWorkerCountMetric, float64(r.initExecutionData.FunctionMetadata.RuntimeWorkerCount)))

	r.invokeRouter.AbortRunningInvokes(metrics, shutdownCause)

	reason := rapimodel.Spindown

	if shutdownCause != nil && shutdownCause.ErrorType() != model.ErrorExecutionEnvironmentShutdown {
		reason = rapimodel.Failure
	}

	slog.Info("ShutdownContext shutdown() initiated", "reason", reason)

	err := r.shutdownContext.shutdown(
		r.supervisor,
		r.renderingService,
		r.registrationService.GetExternalAgents(),
		r.registrationService.CountAgents(),
		reason,
		metrics,
		r.eventsAPI,
	)
	if err != nil {

		slog.Warn("Error during shutdown Context shutdown", "err", err)

		return nil
	}

	duration := metrics.CreateDurationMetric(interop.ShutdownRuntimeServerDuration)

	if err := r.server.Close(); err != nil {
		slog.Error("Error during runtime server close", "err", err)
	}
	duration.Done()

	return nil
}

func (r *rapidContext) RuntimeAPIAddrPort() netip.AddrPort {
	return r.server.AddrPort()
}

type InvokeRouter interface {
	Invoke(ctx context.Context, initData interop.InitStaticDataProvider, invokeReq interop.InvokeRequest, metrics interop.InvokeMetrics, responseWriter http.ResponseWriter) (err model.AppError, wasResponseSent bool, invokePending bool)
	Reconnect(ctx context.Context, invokeID interop.InvokeID, responseWriter http.ResponseWriter, metrics interop.ReconnectMetrics) interop.ReconnectResult
	AbortRunningInvokes(metrics interop.ShutdownMetrics, err model.AppError)
	GetActiveRuntimeCount() int
}
