// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package rapid implements synchronous even dispatch loop.
package rapid

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"go.amzn.com/lambda/agents"
	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/extensions"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/logging"
	"go.amzn.com/lambda/metering"
	"go.amzn.com/lambda/rapi"
	"go.amzn.com/lambda/rapi/rendering"
	"go.amzn.com/lambda/runtimecmd"
	"go.amzn.com/lambda/telemetry"

	log "github.com/sirupsen/logrus"
)

const (
	RuntimeAPIHost        = "127.0.0.1"
	RuntimeAPIPort        = 9001
	defaultAgentLocation  = "/opt/extensions"
	runtimeDeadlineShare  = 0.3
	disableExtensionsFile = "/opt/disable-extensions-jwigqn8j"
)

const (
	standaloneShutdownReason = "spindown"
)

var errResetReceived = errors.New("errResetReceived")

type rapidContext struct {
	bootstrap           Bootstrap
	interopServer       interop.Server
	server              *rapi.Server
	appCtx              appctx.ApplicationContext
	preLoadTimeNs       int64
	postLoadTimeNs      int64
	startRequest        *interop.Start
	initDone            bool
	initFlow            core.InitFlowSynchronization
	invokeFlow          core.InvokeFlowSynchronization
	registrationService core.RegistrationService
	renderingService    *rendering.EventRenderingService
	telemetryAPIEnabled bool
	telemetryService    telemetry.LogsAPIService
	xray                telemetry.Tracer
	exitPidChan         chan int
	resetChan           chan *interop.Reset
	environment         EnvironmentVariables
	standaloneMode      bool
	debugTailLogger     *logging.TailLogWriter
	platformLogger      logging.PlatformLogger
	extensionLogWriter  io.Writer
	runtimeLogWriter    io.Writer
}

func (c *rapidContext) HasActiveExtensions() bool {
	return extensions.AreEnabled() && c.registrationService.CountAgents() > 0
}

func logAgentsInitStatus(execCtx *rapidContext) {
	for _, agent := range execCtx.registrationService.AgentsInfo() {
		execCtx.platformLogger.LogExtensionInitEvent(agent.Name, agent.State, agent.ErrorType, agent.Subscriptions)
	}
}

func agentLaunchError(agent *core.ExternalAgent, appCtx appctx.ApplicationContext, launchError error) {
	if err := agent.LaunchError(launchError); err != nil {
		log.Warnf("LaunchError transition fail for %s from %s: %s", agent, agent.GetState().Name(), err)
	}
	appctx.StoreFirstFatalError(appCtx, fatalerror.AgentLaunchError)
}

func doInitExtensions(execCtx *rapidContext, watchdog *core.Watchdog) error {
	agentPaths := agents.ListExternalAgentPaths(defaultAgentLocation)
	initFlow := execCtx.registrationService.InitFlow()

	// we don't bring it into the loop below because we don't want unnecessary broadcasts on agent gate
	if err := initFlow.SetExternalAgentsRegisterCount(uint16(len(agentPaths))); err != nil {
		return err
	}

	for _, agentPath := range agentPaths {
		env := execCtx.environment.AgentExecEnv()
		agentLogSinks := execCtx.extensionLogWriter
		agentProc := agents.NewExternalAgentProcess(agentPath, env, agentLogSinks)

		agent, err := execCtx.registrationService.CreateExternalAgent(agentProc.Name())
		if err != nil {
			return err
		}

		if execCtx.registrationService.CountAgents() > core.MaxAgentsAllowed {
			agentLaunchError(agent, execCtx.appCtx, core.ErrTooManyExtensions)
			return core.ErrTooManyExtensions
		}

		if err := agentProc.Start(); err != nil {
			agentLaunchError(agent, execCtx.appCtx, err)
			return err
		}

		agent.Pid = watchdog.GoWait(&agentProc, fatalerror.AgentCrash)
	}

	if err := initFlow.AwaitExternalAgentsRegistered(); err != nil {
		return err
	}

	return nil
}

func doInit(ctx context.Context, execCtx *rapidContext, watchdog *core.Watchdog) error {
	execCtx.xray.RecordInitStartTime()
	defer execCtx.xray.RecordInitEndTime()

	if extensions.AreEnabled() {
		defer func() {
			logAgentsInitStatus(execCtx)
		}()

		if err := doInitExtensions(execCtx, watchdog); err != nil {
			return err
		}
	}

	initFlow := execCtx.registrationService.InitFlow()

	// Runtime state machine
	runtime := core.NewRuntime(initFlow, execCtx.invokeFlow)

	// Registration service keeps track of parties registered in the system and events they are registered for.
	// Runtime's use case is generalized, because runtime doesn't register itself, we preregister it in the system;
	// runtime is implicitly subscribed for certain lifecycle events.
	log.Debug("Preregister runtime")
	registrationService := execCtx.registrationService
	if err := registrationService.PreregisterRuntime(runtime); err != nil {
		return err
	}

	bootstrap := execCtx.bootstrap
	bootstrapCmd, err := bootstrap.Cmd()
	if err != nil {
		if fatalError, formattedLog, hasError := bootstrap.CachedFatalError(err); hasError {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalError)
			execCtx.platformLogger.Printf("%s", formattedLog)
		} else {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalerror.InvalidEntrypoint)
		}
		return err
	}

	bootstrapEnv := bootstrap.Env(execCtx.environment)
	bootstrapCwd, err := bootstrap.Cwd()
	if err != nil {
		if fatalError, formattedLog, hasError := bootstrap.CachedFatalError(err); hasError {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalError)
			execCtx.platformLogger.Printf("%s", formattedLog)
		} else {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalerror.InvalidWorkingDir)
		}
		return err
	}

	bootstrapExtraFiles := bootstrap.ExtraFiles()
	runtimeCmd := runtimecmd.NewCustomRuntimeCmd(ctx, bootstrapCmd, bootstrapCwd, bootstrapEnv, execCtx.runtimeLogWriter, bootstrapExtraFiles)

	log.Debug("Start runtime")
	err = runtimeCmd.Start()
	if err != nil {
		if fatalError, formattedLog, hasError := bootstrap.CachedFatalError(err); hasError {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalError)
			execCtx.platformLogger.Printf("%s", formattedLog)
		} else {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalerror.InvalidEntrypoint)
		}

		return err
	}

	registrationService.GetRuntime().Pid = watchdog.GoWait(runtimeCmd, fatalerror.RuntimeExit)

	if err := initFlow.AwaitRuntimeReady(); err != nil {
		return err
	}

	// Registration phase finished for agents - no more agents can be registered with the system
	registrationService.TurnOff()
	if extensions.AreEnabled() {
		// Initialize and activate the gate with the number of agent we wait to return ready
		if err := initFlow.SetAgentsReadyCount(registrationService.GetRegisteredAgentsSize()); err != nil {
			return err
		}
		if err := initFlow.AwaitAgentsReady(); err != nil {
			return err
		}
	}

	// Logs API subscription phase finished for agents - no more agents can be subscribed to the Logs API
	if execCtx.telemetryAPIEnabled {
		execCtx.telemetryService.TurnOff()
	}

	execCtx.initDone = true
	return nil
}

func doInvoke(ctx context.Context, execCtx *rapidContext, watchdog *core.Watchdog, invokeRequest *interop.Invoke, mx *rendering.InvokeRendererMetrics) error {
	appCtx := execCtx.appCtx
	appctx.StoreErrorResponse(appCtx, nil)

	if invokeRequest.NeedDebugLogs {
		execCtx.debugTailLogger.Enable()
	} else {
		execCtx.debugTailLogger.Disable()
	}

	xray := execCtx.xray
	xray.Configure(invokeRequest)

	return xray.CaptureInvokeSegment(ctx, xray.WithErrorCause(ctx, appCtx, func(ctx context.Context) error {
		if !execCtx.initDone {
			// do inline init
			if err := xray.CaptureInitSubsegment(ctx, func(ctx context.Context) error {
				return doInit(ctx, execCtx, watchdog)
			}); err != nil {
				return err
			}
		} else if execCtx.startRequest.SandboxType != interop.SandboxPreWarmed {
			xray.SendInitSubsegmentWithRecordedTimesOnce(ctx)
		}

		invokeFlow := execCtx.invokeFlow
		log.Debug("Initialize invoke flow barriers")
		err := invokeFlow.InitializeBarriers()
		if err != nil {
			return err
		}

		registrationService := execCtx.registrationService
		runtime := registrationService.GetRuntime()
		var intAgents []*core.InternalAgent
		var extAgents []*core.ExternalAgent

		if extensions.AreEnabled() {
			intAgents = registrationService.GetSubscribedInternalAgents(core.InvokeEvent)
			extAgents = registrationService.GetSubscribedExternalAgents(core.InvokeEvent)
			if err := invokeFlow.SetAgentsReadyCount(uint16(len(intAgents) + len(extAgents))); err != nil {
				return err
			}
		}

		// Invoke
		if err := xray.CaptureInvokeSubsegment(ctx, xray.WithError(ctx, appCtx, func(ctx context.Context) error {
			log.Debug("Set renderer for invoke")
			renderer := rendering.NewInvokeRenderer(ctx, invokeRequest, xray.TracingHeaderParser())
			defer func() {
				*mx = renderer.GetMetrics()
			}()

			execCtx.renderingService.SetRenderer(renderer)
			if extensions.AreEnabled() {
				log.Debug("Release agents conditions")
				for _, agent := range extAgents {
					agent.Release()
				}
				for _, agent := range intAgents {
					agent.Release()
				}
			}

			log.Debug("Release runtime condition")
			runtime.Release()
			log.Debug("Await runtime response")
			return invokeFlow.AwaitRuntimeResponse()
		})); err != nil {
			return err
		}

		// Runtime overhead
		if err := xray.CaptureOverheadSubsegment(ctx, func(ctx context.Context) error {
			log.Debug("Await runtime ready")
			return invokeFlow.AwaitRuntimeReady()
		}); err != nil {
			return err
		}

		// Extensions overhead
		if execCtx.HasActiveExtensions() {
			execCtx.interopServer.SendRuntimeReady()
			log.Debug("Await agents ready")
			if err := invokeFlow.AwaitAgentsReady(); err != nil {
				log.Warnf("AwaitAgentsReady() = %s", err)
				return err
			}
		}

		return nil
	}))
}

func extensionsDisabledByLayer() bool {
	_, err := os.Stat(disableExtensionsFile)
	log.Infof("extensionsDisabledByLayer(%s) -> %s", disableExtensionsFile, err)
	return err == nil
}

// acceptStartRequest is a second initialization phase, performed after receiving START
// initialized entities: _HANDLER, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN
func (c *rapidContext) acceptStartRequest(startRequest *interop.Start) {
	c.startRequest = startRequest
	c.environment.StoreEnvironmentVariablesFromInit(
		startRequest.CustomerEnvironmentVariables,
		startRequest.Handler,
		startRequest.AwsKey,
		startRequest.AwsSecret,
		startRequest.AwsSession,
		startRequest.FunctionName,
		startRequest.FunctionVersion)
	c.registrationService.SetFunctionMetadata(core.FunctionMetadata{
		FunctionName:    startRequest.FunctionName,
		FunctionVersion: startRequest.FunctionVersion,
		Handler:         startRequest.Handler,
	})

	if extensionsDisabledByLayer() {
		extensions.Disable()
	}
}

func handleStart(ctx context.Context, execCtx *rapidContext, watchdog *core.Watchdog, startRequest *interop.Start) {
	execCtx.acceptStartRequest(startRequest)

	interopServer, appCtx := execCtx.interopServer, execCtx.appCtx

	if err := interopServer.SendRunning(&interop.Running{
		PreLoadTimeNs:     execCtx.preLoadTimeNs,
		PostLoadTimeNs:    execCtx.postLoadTimeNs,
		WaitStartTimeNs:   execCtx.postLoadTimeNs,
		WaitEndTimeNs:     metering.Monotime(),
		ExtensionsEnabled: extensions.AreEnabled(),
	}); err != nil {
		log.Panic(err)
	}

	if !startRequest.SuppressInit {
		if err := doInit(ctx, execCtx, watchdog); err != nil {
			log.WithError(err).WithField("InvokeID", startRequest.InvokeID).Error("Init failed")
			doneFailMsg := generateDoneFail(execCtx, startRequest.CorrelationID, nil)
			handleInitError(doneFailMsg, execCtx, startRequest.InvokeID, interopServer, err)
			return
		}
	}

	doneMsg := &interop.Done{
		CorrelationID: startRequest.CorrelationID,
		Meta: interop.DoneMetadata{
			RuntimeRelease:      appctx.GetRuntimeRelease(appCtx),
			NumActiveExtensions: execCtx.registrationService.CountAgents(),
		},
	}
	if execCtx.telemetryAPIEnabled {
		doneMsg.Meta.LogsAPIMetrics = execCtx.telemetryService.FlushMetrics()
	}
	if err := interopServer.SendDone(doneMsg); err != nil {
		log.Panic(err)
	}
}

func generateDoneFail(execCtx *rapidContext, correlationID string, invokeMx *rendering.InvokeRendererMetrics) *interop.DoneFail {
	errorType, found := appctx.LoadFirstFatalError(execCtx.appCtx)
	if !found {
		errorType = fatalerror.Unknown
	}

	doneFailMsg := &interop.DoneFail{
		ErrorType:     errorType,
		CorrelationID: correlationID, // required for standalone mode
		Meta: interop.DoneMetadata{
			RuntimeRelease:      appctx.GetRuntimeRelease(execCtx.appCtx),
			NumActiveExtensions: execCtx.registrationService.CountAgents(),
		},
	}

	if invokeMx != nil {
		doneFailMsg.Meta.InvokeRequestReadTimeNs = invokeMx.ReadTime.Nanoseconds()
		doneFailMsg.Meta.InvokeRequestSizeBytes = int64(invokeMx.SizeBytes)
	}

	if execCtx.telemetryAPIEnabled {
		doneFailMsg.Meta.LogsAPIMetrics = execCtx.telemetryService.FlushMetrics()
	}

	return doneFailMsg
}

func handleInvoke(ctx context.Context, execCtx *rapidContext, watchdog *core.Watchdog, invokeRequest *interop.Invoke) {
	interopServer, appCtx := execCtx.interopServer, execCtx.appCtx

	invokeMx := rendering.InvokeRendererMetrics{}

	if err := doInvoke(ctx, execCtx, watchdog, invokeRequest, &invokeMx); err != nil {
		log.WithError(err).WithField("InvokeID", invokeRequest.ID).Error("Invoke failed")
		doneFailMsg := generateDoneFail(execCtx, invokeRequest.CorrelationID, &invokeMx)
		handleInvokeError(doneFailMsg, execCtx, invokeRequest.ID, interopServer, err)
		return
	}

	if err := execCtx.interopServer.CommitResponse(); err != nil {
		log.Panic(err)
	}

	var invokeCompletionTimeNs int64
	if responseTimeNs := execCtx.registrationService.GetRuntime().GetRuntimeDescription().State.ResponseTimeNs; responseTimeNs != 0 {
		invokeCompletionTimeNs = time.Now().UnixNano() - responseTimeNs
	}

	doneMsg := &interop.Done{
		CorrelationID: invokeRequest.CorrelationID,
		Meta: interop.DoneMetadata{
			RuntimeRelease:          appctx.GetRuntimeRelease(appCtx),
			NumActiveExtensions:     execCtx.registrationService.CountAgents(),
			InvokeRequestReadTimeNs: invokeMx.ReadTime.Nanoseconds(),
			InvokeRequestSizeBytes:  int64(invokeMx.SizeBytes),
			InvokeCompletionTimeNs:  invokeCompletionTimeNs,
		},
	}
	if execCtx.telemetryAPIEnabled {
		doneMsg.Meta.LogsAPIMetrics = execCtx.telemetryService.FlushMetrics()
	}

	if err := interopServer.SendDone(doneMsg); err != nil {
		log.Panic(err)
	}
}

func reinitialize(execCtx *rapidContext, watchdog *core.Watchdog) {
	execCtx.interopServer.Clear()
	execCtx.appCtx.Delete(appctx.AppCtxInvokeErrorResponseKey)
	execCtx.appCtx.Delete(appctx.AppCtxRuntimeReleaseKey)
	execCtx.appCtx.Delete(appctx.AppCtxFirstFatalErrorKey)
	execCtx.renderingService.SetRenderer(nil)
	execCtx.initDone = false
	execCtx.registrationService.Clear()
	execCtx.initFlow.Clear()
	execCtx.invokeFlow.Clear()
	if execCtx.telemetryAPIEnabled {
		execCtx.telemetryService.Clear()
	}
	watchdog.Clear()
}

func blockForever() {
	select {}
}

// handle notification of reset
func handleReset(execCtx *rapidContext, watchdog *core.Watchdog, reset *interop.Reset) {
	log.Warnf("Reset initiated: %s", reset.Reason)

	profiler := metering.ExtensionsResetDurationProfiler{}
	gracefulShutdown(execCtx, watchdog, &profiler, reset.DeadlineNs, execCtx.standaloneMode, reset.Reason)

	extensionsResetMs, resetTimeout := profiler.CalculateExtensionsResetMs()

	meta := interop.DoneMetadata{
		ExtensionsResetMs: extensionsResetMs,
	}

	if !execCtx.standaloneMode {
		// GIRP interopServer implementation sends GIRP RSTFAIL/RSTDONE
		if resetTimeout {
			// TODO: DoneFail must contain a reset timeout ErrorType for rapid local to distinguish errors
			doneFail := &interop.DoneFail{CorrelationID: reset.CorrelationID, Meta: meta}
			if err := execCtx.interopServer.SendDoneFail(doneFail); err != nil {
				log.Panicf("Failed to SendDoneFail: %s", err)
			}
		} else {
			done := &interop.Done{CorrelationID: reset.CorrelationID, Meta: meta}
			if err := execCtx.interopServer.SendDone(done); err != nil {
				log.Panicf("Failed to SendDone: %s", err)
			}
		}

		os.Exit(0)
	}

	reinitialize(execCtx, watchdog)

	fatalErrorType, _ := appctx.LoadFirstFatalError(execCtx.appCtx)

	if resetTimeout {
		doneFail := &interop.DoneFail{CorrelationID: reset.CorrelationID, ErrorType: fatalErrorType, Meta: meta}
		if err := execCtx.interopServer.SendDoneFail(doneFail); err != nil {
			log.Panicf("Failed to SendDoneFail: %s", err)
		}
	} else {
		done := &interop.Done{CorrelationID: reset.CorrelationID, ErrorType: fatalErrorType, Meta: meta}
		if err := execCtx.interopServer.SendDone(done); err != nil {
			log.Panicf("Failed to SendDone: %s", err)
		}
	}
}

// handle notification of shutdown
func handleShutdown(execCtx *rapidContext, watchdog *core.Watchdog, shutdown *interop.Shutdown, reason string) {
	log.Warnf("Shutdown initiated")

	gracefulShutdown(execCtx, watchdog, &metering.ExtensionsResetDurationProfiler{}, shutdown.DeadlineNs, true, reason)

	fatalErrorType, _ := appctx.LoadFirstFatalError(execCtx.appCtx)

	if err := execCtx.interopServer.SendDone(&interop.Done{CorrelationID: shutdown.CorrelationID, ErrorType: fatalErrorType}); err != nil {
		log.Panicf("Failed to SendDone: %s", err)
	}

	// Shutdown induces a terminal state and no further messages will be processed
	blockForever()
}

func start(signalCtx context.Context, execCtx *rapidContext) {
	watchdog := core.NewWatchdog(execCtx.registrationService.InitFlow(), execCtx.invokeFlow, execCtx.exitPidChan, execCtx.appCtx)

	interopServer := execCtx.interopServer

	// Start Runtime API Server
	err := execCtx.server.Listen()
	if err != nil {
		log.WithError(err).Panic("Runtime API Server failed to listen")
	}

	go func() { execCtx.server.Serve(signalCtx) }()

	// Note, most of initialization code should run before blocking to receive START,
	// code before START runs in parallel with code downloads.

	go func() {
		for {
			reset := <-interopServer.ResetChan()
			// In the event of a Reset during init/invoke, CancelFlows cancels execution
			// flows and return with the errResetReceived err - this error is special-cased
			// and not handled by the init/invoke (unexpected) error handling functions
			watchdog.CancelFlows(errResetReceived)
			execCtx.resetChan <- reset
		}
	}()

	for {
		select {
		case start := <-interopServer.StartChan():
			handleStart(signalCtx, execCtx, watchdog, start)
		case invoke := <-interopServer.InvokeChan():
			handleInvoke(signalCtx, execCtx, watchdog, invoke)
		case err := <-interopServer.TransportErrorChan():
			log.Panicf("Transport error emitted by interop server: %s", err)
		case reset := <-execCtx.resetChan:
			handleReset(execCtx, watchdog, reset)
		case shutdown := <-interopServer.ShutdownChan(): // only in standalone mode
			handleShutdown(execCtx, watchdog, shutdown, standaloneShutdownReason)
		}
	}
}
