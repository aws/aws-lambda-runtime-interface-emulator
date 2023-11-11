// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi"
	"go.amzn.com/lambda/rapi/rendering"
	supvmodel "go.amzn.com/lambda/supervisor/model"
	"go.amzn.com/lambda/telemetry"
)

type Sandbox struct {
	EnableTelemetryAPI       bool
	StandaloneMode           bool
	InteropServer            interop.Server
	Tracer                   telemetry.Tracer
	LogsSubscriptionAPI      telemetry.SubscriptionAPI
	TelemetrySubscriptionAPI telemetry.SubscriptionAPI
	LogsEgressAPI            telemetry.StdLogsEgressAPI
	RuntimeStdoutWriter      io.Writer
	RuntimeStderrWriter      io.Writer
	Handler                  string
	EventsAPI                interop.EventsAPI
	InitCachingEnabled       bool
	Supervisor               supvmodel.ProcessSupervisor
	RuntimeFsRootPath        string // path to the root of the domain within the root mnt namespace. Reqired to find extensions
	RuntimeAPIHost           string
	RuntimeAPIPort           int
}

// Start pings Supervisor, and starts the Runtime API server. It allows the caller to configure:
// - Supervisor implementation: performs container construction & process management
// - Telemetry API and Logs API implementation: handling /logs and /telemetry of Runtime API
// - Events API implementation: handles platform log events emitted by Rapid (e.g. RuntimeDone, InitStart)
// - Logs Egress implementation: handling stdout/stderr logs from extension & runtime processes (TODO: remove & unify with Supervisor)
// - Tracer implementation: handling trace segments generate by platform (TODO: remove & unify with Events API)
// - InteropServer implementation: legacy interface for sending internal protocol messages, today only RuntimeReady remains (TODO: move RuntimeReady outside Core)
// - Feature flags:
//   - StandaloneMode: indicates if being called by Rapid Core's standalone HTTP frontend (TODO: remove after unifying error reporting)
//   - InitCachingEnabled: indicates if handlers must run Init Caching specific logic
//   - TelemetryAPIEnabled: indicates if /telemetry and /logs endpoint HTTP handlers must be mounted
//
// - Contexts & Data:
//   - ctx is used to gracefully terminate Runtime API HTTP Server on exit
func Start(ctx context.Context, s *Sandbox) (interop.RapidContext, interop.InternalStateGetter, string) {
	// Initialize internal state objects required by Rapid handlers
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	invokeFlow := core.NewInvokeFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow, invokeFlow)
	renderingService := rendering.NewRenderingService()
	credentialsService := core.NewCredentialsService()

	appctx.StoreInitType(appCtx, s.InitCachingEnabled)

	server := rapi.NewServer(s.RuntimeAPIHost, s.RuntimeAPIPort, appCtx, registrationService, renderingService, s.EnableTelemetryAPI, s.LogsSubscriptionAPI, s.TelemetrySubscriptionAPI, credentialsService)
	runtimeAPIAddr := fmt.Sprintf("%s:%d", server.Host(), server.Port())

	// TODO: pass this directly down to HTTP servers and handlers, instead of using
	// global state to share the interop server implementation
	appctx.StoreInteropServer(appCtx, s.InteropServer)

	execCtx := &rapidContext{
		// Internally initialized configurations
		server:                server,
		appCtx:                appCtx,
		initDone:              false,
		initFlow:              initFlow,
		invokeFlow:            invokeFlow,
		registrationService:   registrationService,
		renderingService:      renderingService,
		credentialsService:    credentialsService,
		handlerExecutionMutex: sync.Mutex{},
		shutdownContext:       newShutdownContext(),

		// Externally specified configurations (i.e. via SandboxBuilder)
		telemetryAPIEnabled:      s.EnableTelemetryAPI,
		logsSubscriptionAPI:      s.LogsSubscriptionAPI,
		telemetrySubscriptionAPI: s.TelemetrySubscriptionAPI,
		logsEgressAPI:            s.LogsEgressAPI,
		interopServer:            s.InteropServer,
		xray:                     s.Tracer,
		standaloneMode:           s.StandaloneMode,
		eventsAPI:                s.EventsAPI,
		initCachingEnabled:       s.InitCachingEnabled,
		supervisor: processSupervisor{
			ProcessSupervisor: s.Supervisor,
			RootPath:          s.RuntimeFsRootPath,
		},

		RuntimeStartedTime:         -1,
		RuntimeOverheadStartedTime: -1,
		InvokeResponseMetrics:      nil,
	}

	go startRuntimeAPI(ctx, execCtx)

	return execCtx, registrationService.GetInternalStateDescriptor(appCtx), runtimeAPIAddr
}

func (r *rapidContext) HandleInit(init *interop.Init, initSuccessResponseChan chan<- interop.InitSuccess, initFailureResponseChan chan<- interop.InitFailure) {
	r.handlerExecutionMutex.Lock()
	defer r.handlerExecutionMutex.Unlock()
	handleInit(r, init, initSuccessResponseChan, initFailureResponseChan)
}

func (r *rapidContext) HandleInvoke(invoke *interop.Invoke, sbInfoFromInit interop.SandboxInfoFromInit, requestBuffer *bytes.Buffer, responseSender interop.InvokeResponseSender) (interop.InvokeSuccess, *interop.InvokeFailure) {
	r.handlerExecutionMutex.Lock()
	defer r.handlerExecutionMutex.Unlock()
	// Clear the context used by the last invoke
	r.appCtx.Delete(appctx.AppCtxInvokeErrorTraceDataKey)
	return handleInvoke(r, invoke, sbInfoFromInit, requestBuffer, responseSender)
}

func (r *rapidContext) HandleReset(reset *interop.Reset) (interop.ResetSuccess, *interop.ResetFailure) {
	// In the event of a Reset during init/invoke, CancelFlows cancels execution
	// flows and return with the errResetReceived err - this error is special-cased
	// and not handled by the init/invoke (unexpected) error handling functions
	r.registrationService.CancelFlows(errResetReceived)

	// Wait until invoke error handling has returned before continuing execution
	r.handlerExecutionMutex.Lock()
	defer r.handlerExecutionMutex.Unlock()

	// Clear the context used by the last invoke
	r.appCtx.Delete(appctx.AppCtxInvokeErrorTraceDataKey)
	return handleReset(r, reset, r.RuntimeStartedTime, r.InvokeResponseMetrics)
}

func (r *rapidContext) HandleShutdown(shutdown *interop.Shutdown) interop.ShutdownSuccess {
	// Wait until invoke error handling has returned before continuing execution
	r.handlerExecutionMutex.Lock()
	defer r.handlerExecutionMutex.Unlock()
	// Shutdown doesn't cancel flows, so it can block forever
	return handleShutdown(r, shutdown, standaloneShutdownReason)
}

func (r *rapidContext) HandleRestore(restore *interop.Restore) (interop.RestoreResult, error) {
	return handleRestore(r, restore)
}

func (r *rapidContext) Clear() {
	reinitialize(r)
}

func (r *rapidContext) SetRuntimeStartedTime(runtimeStartedTime int64) {
	r.RuntimeStartedTime = runtimeStartedTime
}

func (r *rapidContext) SetRuntimeOverheadStartedTime(runtimeOverheadStartedTime int64) {
	r.RuntimeOverheadStartedTime = runtimeOverheadStartedTime
}

func (r *rapidContext) SetInvokeResponseMetrics(metrics *interop.InvokeResponseMetrics) {
	r.InvokeResponseMetrics = metrics
}

func (r *rapidContext) SetLogStreamName(logStreamName string) {
	r.logStreamName = logStreamName
}

func (r *rapidContext) SetEventsAPI(eventsAPI interop.EventsAPI) {
	r.eventsAPI = eventsAPI
}
