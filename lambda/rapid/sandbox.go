// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/metering"
	"go.amzn.com/lambda/rapi"
	"go.amzn.com/lambda/rapi/rendering"
	supvmodel "go.amzn.com/lambda/supervisor/model"
	"go.amzn.com/lambda/telemetry"

	log "github.com/sirupsen/logrus"
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
	PreLoadTimeNs            int64
	Handler                  string
	SignalCtx                context.Context
	EventsAPI                telemetry.EventsAPI
	InitCachingEnabled       bool
	Supervisor               supvmodel.Supervisor
	RuntimeAPIHost           string
	RuntimeAPIPort           int
}

// Start is a public version of start() that exports only configurable parameters
func Start(s *Sandbox) (interop.RapidContext, interop.InternalStateGetter, string) {
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	invokeFlow := core.NewInvokeFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow, invokeFlow)
	renderingService := rendering.NewRenderingService()
	credentialsService := core.NewCredentialsService()

	appctx.StoreInitType(appCtx, s.InitCachingEnabled)

	server := rapi.NewServer(s.RuntimeAPIHost, s.RuntimeAPIPort, appCtx, registrationService, renderingService, s.EnableTelemetryAPI, s.LogsSubscriptionAPI, s.TelemetrySubscriptionAPI, credentialsService, s.EventsAPI)
	runtimeAPIAddr := fmt.Sprintf("%s:%d", server.Host(), server.Port())

	postLoadTimeNs := metering.Monotime()

	// TODO: pass this directly down to HTTP servers and handlers, instead of using
	// global state to share the interop server implementation
	appctx.StoreInteropServer(appCtx, s.InteropServer)

	execCtx := &rapidContext{
		server:              server,
		appCtx:              appCtx,
		postLoadTimeNs:      postLoadTimeNs,
		initDone:            false,
		initFlow:            initFlow,
		invokeFlow:          invokeFlow,
		registrationService: registrationService,
		renderingService:    renderingService,
		credentialsService:  credentialsService,

		telemetryAPIEnabled:      s.EnableTelemetryAPI,
		logsSubscriptionAPI:      s.LogsSubscriptionAPI,
		telemetrySubscriptionAPI: s.TelemetrySubscriptionAPI,
		logsEgressAPI:            s.LogsEgressAPI,
		interopServer:            s.InteropServer,
		xray:                     s.Tracer,
		standaloneMode:           s.StandaloneMode,
		preLoadTimeNs:            s.PreLoadTimeNs,
		eventsAPI:                s.EventsAPI,
		initCachingEnabled:       s.InitCachingEnabled,
		signalCtx:                s.SignalCtx,
		supervisor:               s.Supervisor,
		executionMutex:           sync.Mutex{},
		shutdownContext:          newShutdownContext(),
	}

	// We call /ping on Supervisor before starting Rapid, since Rapid
	// depends on Supervisor setting up networking dependencies
	var startupErr error
	for retries := 1; retries <= 5; retries++ {
		if startupErr = s.Supervisor.Ping(); startupErr == nil {
			break
		}
		// Retry timeout: 5s, same order-of-mag as test client PING retries
		// TODO: revisit retry timeout, identify appropriate value for prod.
		time.Sleep(1000 * time.Millisecond)
	}

	if startupErr != nil {
		log.Panicf("Application ping to Supervisor failed, terminating Rapid Startup: %s", startupErr)
	}

	go start(s.SignalCtx, execCtx)

	return execCtx, registrationService.GetInternalStateDescriptor(appCtx), runtimeAPIAddr
}

func (r *rapidContext) HandleInit(init *interop.Init, initStartedResponseChan chan<- interop.InitStarted, initSuccessResponseChan chan<- interop.InitSuccess, initFailureResponseChan chan<- interop.InitFailure) {
	r.executionMutex.Lock()
	defer r.executionMutex.Unlock()
	handleInit(r, init, initStartedResponseChan, initSuccessResponseChan, initFailureResponseChan)
}

func (r *rapidContext) HandleInvoke(invoke *interop.Invoke, sbInfoFromInit interop.SandboxInfoFromInit) (interop.InvokeSuccess, *interop.InvokeFailure) {
	r.executionMutex.Lock()
	defer r.executionMutex.Unlock()
	// Clear the context used by the last invok
	r.appCtx.Delete(appctx.AppCtxInvokeErrorResponseKey)
	return handleInvoke(r, invoke, sbInfoFromInit)
}

func (r *rapidContext) HandleReset(reset *interop.Reset, invokeReceivedTime int64, InvokeResponseMetrics *interop.InvokeResponseMetrics) (interop.ResetSuccess, *interop.ResetFailure) {
	// In the event of a Reset during init/invoke, CancelFlows cancels execution
	// flows and return with the errResetReceived err - this error is special-cased
	// and not handled by the init/invoke (unexpected) error handling functions
	r.registrationService.CancelFlows(errResetReceived)

	// Wait until invoke error handling has returned before continuing execution
	r.executionMutex.Lock()
	defer r.executionMutex.Unlock()

	// Clear the context used by the last invoke, i.e. error message etc.
	r.appCtx.Delete(appctx.AppCtxInvokeErrorResponseKey)
	return handleReset(r, reset, invokeReceivedTime, InvokeResponseMetrics)
}

func (r *rapidContext) HandleShutdown(shutdown *interop.Shutdown) interop.ShutdownSuccess {
	// Wait until invoke error handling has returned before continuing execution
	r.executionMutex.Lock()
	defer r.executionMutex.Unlock()
	// Shutdown doesn't cancel flows, so it can block forever
	return handleShutdown(r, shutdown, standaloneShutdownReason)
}

func (r *rapidContext) HandleRestore(restore *interop.Restore) error {
	return handleRestore(r, restore)
}

func (r *rapidContext) Clear() {
	reinitialize(r)
}
