// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package rapid implements synchronous even dispatch loop.
package rapid

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"go.amzn.com/lambda/agents"
	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/extensions"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/metering"
	"go.amzn.com/lambda/rapi"
	"go.amzn.com/lambda/rapi/model"
	"go.amzn.com/lambda/rapi/rendering"
	supvmodel "go.amzn.com/lambda/supervisor/model"
	"go.amzn.com/lambda/telemetry"

	"github.com/google/uuid"

	log "github.com/sirupsen/logrus"
)

const (
	RuntimeDomain         = "runtime"
	OperatorDomain        = "operator"
	defaultAgentLocation  = "/opt/extensions"
	disableExtensionsFile = "/opt/disable-extensions-jwigqn8j"
	runtimeProcessName    = "runtime"
)

const (
	// Same value as defined in LambdaSandbox minus 1.
	maxExtensionNamesLength = 127
	standaloneShutdownReason = "spindown"
)

var errResetReceived = errors.New("errResetReceived")

type rapidContext struct {
	interopServer            interop.Server
	server                   *rapi.Server
	appCtx                   appctx.ApplicationContext
	preLoadTimeNs            int64
	postLoadTimeNs           int64
	initDone                 bool
	supervisor               supvmodel.Supervisor
	runtimeDomainGeneration  uint32
	initFlow                 core.InitFlowSynchronization
	invokeFlow               core.InvokeFlowSynchronization
	registrationService      core.RegistrationService
	renderingService         *rendering.EventRenderingService
	telemetryAPIEnabled      bool
	logsSubscriptionAPI      telemetry.SubscriptionAPI
	telemetrySubscriptionAPI telemetry.SubscriptionAPI
	logsEgressAPI            telemetry.StdLogsEgressAPI
	xray                     telemetry.Tracer
	standaloneMode           bool
	eventsAPI                telemetry.EventsAPI
	initCachingEnabled       bool
	credentialsService       core.CredentialsService
	signalCtx                context.Context
	executionMutex           sync.Mutex
	shutdownContext          *shutdownContext
}

// Validate interface compliance
var _ interop.RapidContext = (*rapidContext)(nil)

type invokeMetrics struct {
	rendererMetrics rendering.InvokeRendererMetrics

	runtimeReadyTime int64
}

func (c *rapidContext) HasActiveExtensions() bool {
	return extensions.AreEnabled() && c.registrationService.CountAgents() > 0
}

func (c *rapidContext) GetExtensionNames() string {
	var extensionNamesList []string
	for _, agent := range c.registrationService.AgentsInfo() {
		extensionNamesList = append(extensionNamesList, agent.Name)
	}
	extensionNames := strings.Join(extensionNamesList, ";")
	if len(extensionNames) > maxExtensionNamesLength {
		if idx := strings.LastIndex(extensionNames[:maxExtensionNamesLength], ";"); idx != -1 {
			return extensionNames[:idx]
		}
		return ""
	}
	return extensionNames
}

func logAgentsInitStatus(execCtx *rapidContext) {
	for _, agent := range execCtx.registrationService.AgentsInfo() {
		execCtx.eventsAPI.SendExtensionInit(agent.Name, agent.State, agent.ErrorType, agent.Subscriptions)
	}
}

func agentLaunchError(agent *core.ExternalAgent, appCtx appctx.ApplicationContext, launchError error) {
	if err := agent.LaunchError(launchError); err != nil {
		log.Warnf("LaunchError transition fail for %s from %s: %s", agent, agent.GetState().Name(), err)
	}
	appctx.StoreFirstFatalError(appCtx, fatalerror.AgentLaunchError)
}

func doInitExtensions(domain string, agentPaths []string, execCtx *rapidContext, env interop.EnvironmentVariables) error {
	initFlow := execCtx.registrationService.InitFlow()

	// we don't bring it into the loop below because we don't want unnecessary broadcasts on agent gate
	if err := initFlow.SetExternalAgentsRegisterCount(uint16(len(agentPaths))); err != nil {
		return err
	}

	for _, agentPath := range agentPaths {
		// Using path.Base(agentPath) not agentName because the agent name is contact, as standalone can get the internal state.
		agent, err := execCtx.registrationService.CreateExternalAgent(path.Base(agentPath))

		if err != nil {
			return err
		}

		if execCtx.registrationService.CountAgents() > core.MaxAgentsAllowed {
			agentLaunchError(agent, execCtx.appCtx, core.ErrTooManyExtensions)
			return core.ErrTooManyExtensions
		}

		env := env.AgentExecEnv()

		agentStdoutWriter, agentStderrWriter, err := execCtx.logsEgressAPI.GetExtensionSockets()

		if err != nil {
			return err
		}
		agentName := fmt.Sprintf("extension-%s-%d", path.Base(agentPath), execCtx.runtimeDomainGeneration)

		err = execCtx.supervisor.Exec(&supvmodel.ExecRequest{
			Domain:       domain,
			Name:         agentName,
			Path:         agentPath,
			Env:          &env,
			StdoutWriter: agentStdoutWriter,
			StderrWriter: agentStderrWriter,
		})

		if err != nil {
			agentLaunchError(agent, execCtx.appCtx, err)
			return err
		}

		execCtx.shutdownContext.createExitedChannel(agentName)
	}

	if err := initFlow.AwaitExternalAgentsRegistered(); err != nil {
		return err
	}

	return nil
}

func doRuntimeBootstrap(execCtx *rapidContext, sbInfoFromInit interop.SandboxInfoFromInit) ([]string, map[string]string, string, []*os.File, error) {
	env := sbInfoFromInit.EnvironmentVariables
	runtimeBootstrap := sbInfoFromInit.RuntimeBootstrap
	bootstrapCmd, err := runtimeBootstrap.Cmd()
	if err != nil {
		if fatalError, formattedLog, hasError := runtimeBootstrap.CachedFatalError(err); hasError {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalError)
			execCtx.eventsAPI.SendImageErrorLog(formattedLog)
		} else {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalerror.InvalidEntrypoint)
		}
		return []string{}, map[string]string{}, "", []*os.File{}, err
	}

	bootstrapEnv := runtimeBootstrap.Env(env)
	bootstrapCwd, err := runtimeBootstrap.Cwd()
	if err != nil {
		if fatalError, formattedLog, hasError := runtimeBootstrap.CachedFatalError(err); hasError {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalError)
			execCtx.eventsAPI.SendImageErrorLog(formattedLog)
		} else {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalerror.InvalidWorkingDir)
		}
		return []string{}, map[string]string{}, "", []*os.File{}, err
	}

	bootstrapExtraFiles := runtimeBootstrap.ExtraFiles()

	return bootstrapCmd, bootstrapEnv, bootstrapCwd, bootstrapExtraFiles, nil
}

func (c *rapidContext) setupEventsWatcher(events <-chan supvmodel.Event) {
	go func() {
		for event := range events {
			var err error = nil
			log.Debugf("The events handler received the event %+v.", event)
			if loss := event.Event.EventLoss(); loss != nil {
				log.Panicf("Lost %d events from supervisor", *loss)
			}
			termination := event.Event.ProcessTerminated()

			// If we are not shutting down then we care if an unexpected exit happens.
			if !c.shutdownContext.isShuttingDown() {
				runtimeProcessName := fmt.Sprintf("%s-%d", runtimeProcessName, c.runtimeDomainGeneration)

				// If event from the runtime.
				if *termination.Name == runtimeProcessName {
					if termination.Success() {
						err = fmt.Errorf("Runtime exited without providing a reason")
					} else {
						err = fmt.Errorf("Runtime exited with error: %s", termination.String())
					}
					appctx.StoreFirstFatalError(c.appCtx, fatalerror.RuntimeExit)
				} else {
					if termination.Success() {
						err = fmt.Errorf("exit code 0")
					} else {
						err = fmt.Errorf(termination.String())
					}

					appctx.StoreFirstFatalError(c.appCtx, fatalerror.AgentCrash)
				}

				log.Warnf("Process %s exited: %+v", *termination.Name, termination)
			}

			// At the moment we only get termination events.
			// When their are other event types then we would need to be selective,
			// about what we send to handleShutdownEvent().
			c.shutdownContext.handleProcessExit(*termination)
			c.registrationService.CancelFlows(err)
		}
	}()
}

func doOperatorDomainInit(ctx context.Context, execCtx *rapidContext, operatorDomainExtraConfig interop.DynamicDomainConfig) error {
	events, err := execCtx.supervisor.Events()
	if err != nil {
		log.WithError(err).Panic("Could not get events stream from supervsior")
	}
	execCtx.setupEventsWatcher(events)

	log.Info("Configuring and starting Operator Domain")
	conf := operatorDomainExtraConfig
	err = execCtx.supervisor.Configure(&supvmodel.ConfigureRequest{
		Domain:               OperatorDomain,
		AdditionalStartHooks: conf.AdditionalStartHooks,
		Mounts:               conf.Mounts,
	})

	if err != nil {
		log.WithError(err).Error("Failed to configure operator domain")
		return err
	}

	err = execCtx.supervisor.Start(&supvmodel.StartRequest{
		Domain: OperatorDomain,
	})

	if err != nil {
		log.WithError(err).Error("Failed to start operator domain")
		return err
	}

	// we configure the runtime domain only once and not at
	// every init phase (e.g., suppressed or reset).
	err = execCtx.supervisor.Configure(&supvmodel.ConfigureRequest{
		Domain: RuntimeDomain,
	})

	if err != nil {
		log.WithError(err).Error("Failed to configure operator domain")
		return err
	}

	return nil

}

func doRuntimeDomainInit(ctx context.Context, execCtx *rapidContext, sbInfoFromInit interop.SandboxInfoFromInit) error {
	execCtx.xray.RecordInitStartTime()
	defer execCtx.xray.RecordInitEndTime()

	defer func() {
		if extensions.AreEnabled() {
			logAgentsInitStatus(execCtx)
		}
	}()

	log.Info("Starting runtime domain")
	err := execCtx.supervisor.Start(&supvmodel.StartRequest{
		Domain: RuntimeDomain,
	})
	if err != nil {
		log.WithError(err).Panic("Failed configuring runtime domain")
	}
	execCtx.runtimeDomainGeneration++

	if extensions.AreEnabled() {
		runtimeExtensions := agents.ListExternalAgentPaths(defaultAgentLocation,
			execCtx.supervisor.RuntimeConfig.RootPath)
		if err := doInitExtensions(RuntimeDomain, runtimeExtensions, execCtx, sbInfoFromInit.EnvironmentVariables); err != nil {
			return err
		}
	}

	appctx.StoreSandboxType(execCtx.appCtx, sbInfoFromInit.SandboxType)

	initFlow := execCtx.registrationService.InitFlow()

	// Runtime state machine
	runtime := core.NewRuntime(initFlow, execCtx.invokeFlow)

	// Registration service keeps track of parties registered in the system and events they are registered for.
	// Runtime's use case is generalized, because runtime doesn't register itself, we preregister it in the system;
	// runtime is implicitly subscribed for certain lifecycle events.
	log.Debug("Preregister runtime")
	registrationService := execCtx.registrationService
	err = registrationService.PreregisterRuntime(runtime)

	if err != nil {
		return err
	}

	bootstrapCmd, bootstrapEnv, bootstrapCwd, bootstrapExtraFiles, err := doRuntimeBootstrap(execCtx, sbInfoFromInit)

	if err != nil {
		return err
	}

	runtimeStdoutWriter, runtimeStderrWriter, err := execCtx.logsEgressAPI.GetRuntimeSockets()

	if err != nil {
		return err
	}

	log.Debug("Start runtime")
	checkCredentials(execCtx, bootstrapEnv)
	name := fmt.Sprintf("%s-%d", runtimeProcessName, execCtx.runtimeDomainGeneration)
	err = execCtx.supervisor.Exec(&supvmodel.ExecRequest{
		Domain:       RuntimeDomain,
		Name:         name,
		Cwd:          &bootstrapCwd,
		Path:         bootstrapCmd[0],
		Args:         bootstrapCmd[1:],
		Env:          &bootstrapEnv,
		StdoutWriter: runtimeStdoutWriter,
		StderrWriter: runtimeStderrWriter,
		ExtraFiles:   &bootstrapExtraFiles,
	})

	runtimeDoneStatus := telemetry.RuntimeDoneSuccess

	defer func() {
		sendInitRuntimeDoneLogEvent(execCtx, sbInfoFromInit.SandboxType, runtimeDoneStatus)
	}()

	if err != nil {
		if fatalError, formattedLog, hasError := sbInfoFromInit.RuntimeBootstrap.CachedFatalError(err); hasError {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalError)
			execCtx.eventsAPI.SendImageErrorLog(formattedLog)
		} else {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalerror.InvalidEntrypoint)
		}

		runtimeDoneStatus = telemetry.RuntimeDoneFailure
		return err
	}

	execCtx.shutdownContext.createExitedChannel(name)

	if err := initFlow.AwaitRuntimeRestoreReady(); err != nil {
		runtimeDoneStatus = telemetry.RuntimeDoneFailure
		return err
	}

	runtimeDoneStatus = telemetry.RuntimeDoneSuccess

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
		execCtx.logsSubscriptionAPI.TurnOff()
		execCtx.telemetrySubscriptionAPI.TurnOff()
	}

	execCtx.initDone = true

	return nil
}

func doInvoke(ctx context.Context, execCtx *rapidContext, invokeRequest *interop.Invoke, mx *invokeMetrics, sbInfoFromInit interop.SandboxInfoFromInit) error {
	execCtx.eventsAPI.SetCurrentRequestID(invokeRequest.ID)
	appCtx := execCtx.appCtx

	xray := execCtx.xray
	xray.Configure(invokeRequest)

	return xray.CaptureInvokeSegment(ctx, xray.WithErrorCause(ctx, appCtx, func(ctx context.Context) error {
		if !execCtx.initDone {
			// do inline init
			if err := xray.CaptureInitSubsegment(ctx, func(ctx context.Context) error {
				return doRuntimeDomainInit(ctx, execCtx, sbInfoFromInit)
			}); err != nil {
				return err
			}
		} else if sbInfoFromInit.SandboxType != interop.SandboxPreWarmed {
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
				mx.rendererMetrics = renderer.GetMetrics()
			}()

			execCtx.renderingService.SetRenderer(renderer)
			if extensions.AreEnabled() {
				log.Debug("Release agents conditions")
				for _, agent := range extAgents {
					//TODO handle Supervisors listening channel
					agent.Release()
				}
				for _, agent := range intAgents {
					//TODO handle Supervisors listening channel
					agent.Release()
				}
			}

			log.Debug("Release runtime condition")
			//TODO handle Supervisors listening channel
			runtime.Release()
			log.Debug("Await runtime response")
			//TODO handle Supervisors listening channel
			return invokeFlow.AwaitRuntimeResponse()
		})); err != nil {
			return err
		}

		// Runtime overhead
		if err := xray.CaptureOverheadSubsegment(ctx, func(ctx context.Context) error {
			log.Debug("Await runtime ready")
			//TODO handle Supervisors listening channel
			return invokeFlow.AwaitRuntimeReady()
		}); err != nil {
			return err
		}
		mx.runtimeReadyTime = metering.Monotime()

		runtimeDoneEventData := telemetry.InvokeRuntimeDoneData{
			Status:          telemetry.RuntimeDoneSuccess,
			Metrics:         telemetry.GetRuntimeDoneInvokeMetrics(invokeRequest.InvokeReceivedTime, invokeRequest.InvokeResponseMetrics, mx.runtimeReadyTime),
			InternalMetrics: invokeRequest.InvokeResponseMetrics,
			Tracing:         telemetry.BuildTracingCtx(model.XRayTracingType, invokeRequest.TraceID, invokeRequest.LambdaSegmentID),
			Spans:           telemetry.GetRuntimeDoneSpans(invokeRequest.InvokeReceivedTime, invokeRequest.InvokeResponseMetrics),
		}
		if err := execCtx.eventsAPI.SendRuntimeDone(runtimeDoneEventData); err != nil {
			log.Errorf("Failed to send RUNDONE: %s", err)
		}

		// Extensions overhead
		if execCtx.HasActiveExtensions() {
			execCtx.interopServer.SendRuntimeReady()
			log.Debug("Await agents ready")
			//TODO handle Supervisors listening channel
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

// acceptInitRequest is a second initialization phase, performed after receiving START
// initialized entities: _HANDLER, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN
func (c *rapidContext) acceptInitRequest(initRequest *interop.Init) *interop.Init {
	initRequest.EnvironmentVariables.StoreEnvironmentVariablesFromInit(
		initRequest.CustomerEnvironmentVariables,
		initRequest.Handler,
		initRequest.AwsKey,
		initRequest.AwsSecret,
		initRequest.AwsSession,
		initRequest.FunctionName,
		initRequest.FunctionVersion)
	c.registrationService.SetFunctionMetadata(core.FunctionMetadata{
		FunctionName:    initRequest.FunctionName,
		FunctionVersion: initRequest.FunctionVersion,
		Handler:         initRequest.Handler,
		RuntimeInfo:     initRequest.RuntimeInfo,
	})

	if extensionsDisabledByLayer() {
		extensions.Disable()
	}

	return initRequest
}

func (c *rapidContext) acceptInitRequestForInitCaching(initRequest *interop.Init) (*interop.Init, error) {
	log.Info("Configure environment for Init Caching.")
	randomUUID, err := uuid.NewRandom()

	if err != nil {
		return initRequest, err
	}

	initCachingToken := randomUUID.String()

	initRequest.EnvironmentVariables.StoreEnvironmentVariablesFromInitForInitCaching(
		c.server.Host(),
		c.server.Port(),
		initRequest.CustomerEnvironmentVariables,
		initRequest.Handler,
		initRequest.FunctionName,
		initRequest.FunctionVersion,
		initCachingToken)

	c.registrationService.SetFunctionMetadata(core.FunctionMetadata{
		FunctionName:    initRequest.FunctionName,
		FunctionVersion: initRequest.FunctionVersion,
		Handler:         initRequest.Handler,
	})

	c.credentialsService.SetCredentials(initCachingToken, initRequest.AwsKey, initRequest.AwsSecret, initRequest.AwsSession, initRequest.CredentialsExpiry)

	if extensionsDisabledByLayer() {
		extensions.Disable()
	}

	return initRequest, nil
}

func handleInit(execCtx *rapidContext, initRequest *interop.Init,
	initStartedResponse chan<- interop.InitStarted,
	initSuccessResponse chan<- interop.InitSuccess,
	initFailureResponse chan<- interop.InitFailure) {
	ctx := execCtx.signalCtx

	if execCtx.initCachingEnabled {
		var err error
		if initRequest, err = execCtx.acceptInitRequestForInitCaching(initRequest); err != nil {
			// TODO: call handleInitError only after sending the RUNNING, since
			// Slicer will fail receiving DONEFAIL here as it is expecting RUNNING
			handleInitError(execCtx, initRequest.InvokeID, err, initFailureResponse)
			return
		}
	} else {
		initRequest = execCtx.acceptInitRequest(initRequest)
	}

	initStartedMsg := interop.InitStarted{
		PreLoadTimeNs:     execCtx.preLoadTimeNs,
		PostLoadTimeNs:    execCtx.postLoadTimeNs,
		WaitStartTimeNs:   execCtx.postLoadTimeNs,
		WaitEndTimeNs:     metering.Monotime(),
		ExtensionsEnabled: extensions.AreEnabled(),
		Ack:               make(chan struct{}),
	}

	initStartedResponse <- initStartedMsg
	<-initStartedMsg.Ack

	// Operator domain init happens only once, it's never suppressed,
	// and it's terminal in case of failures
	if err := doOperatorDomainInit(ctx, execCtx, initRequest.OperatorDomainExtraConfig); err != nil {
		// TODO: I believe we need to handle this specially, because we want
		// to consider any failure here as terminal
		handleInitError(execCtx, initRequest.InvokeID, err, initFailureResponse)
		return
	}

	if !initRequest.SuppressInit {
		// doRuntimeDomainInit() is used in both init/invoke, so the signature requires sbInfo arg
		sbInfo := interop.SandboxInfoFromInit{
			EnvironmentVariables: initRequest.EnvironmentVariables,
			SandboxType:          initRequest.SandboxType,
			RuntimeBootstrap:     initRequest.Bootstrap,
		}
		if err := doRuntimeDomainInit(ctx, execCtx, sbInfo); err != nil {
			handleInitError(execCtx, initRequest.InvokeID, err, initFailureResponse)
			return
		}
	}

	initSuccessMsg := interop.InitSuccess{
		RuntimeRelease:      appctx.GetRuntimeRelease(execCtx.appCtx),
		NumActiveExtensions: execCtx.registrationService.CountAgents(),
		ExtensionNames:      execCtx.GetExtensionNames(),
		Ack:                 make(chan struct{}),
	}

	if execCtx.telemetryAPIEnabled {
		initSuccessMsg.LogsAPIMetrics = interop.MergeSubscriptionMetrics(execCtx.logsSubscriptionAPI.FlushMetrics(), execCtx.telemetrySubscriptionAPI.FlushMetrics())
	}

	initSuccessResponse <- initSuccessMsg
	<-initSuccessMsg.Ack
}

func handleInvoke(execCtx *rapidContext, invokeRequest *interop.Invoke, sbInfoFromInit interop.SandboxInfoFromInit) (interop.InvokeSuccess, *interop.InvokeFailure) {
	ctx := execCtx.signalCtx
	invokeMx := invokeMetrics{}

	if err := doInvoke(ctx, execCtx, invokeRequest, &invokeMx, sbInfoFromInit); err != nil {
		log.WithError(err).WithField("InvokeID", invokeRequest.ID).Error("Invoke failed")
		invokeFailure := handleInvokeError(execCtx, invokeRequest, &invokeMx, err)

		if invokeRequest.InvokeResponseMetrics != nil && interop.IsResponseStreamingMetrics(invokeRequest.InvokeResponseMetrics) {
			invokeFailure.ResponseMetrics = interop.ResponseMetrics{
				RuntimeTimeThrottledMs:       invokeRequest.InvokeResponseMetrics.TimeShapedNs / int64(time.Millisecond),
				RuntimeProducedBytes:         invokeRequest.InvokeResponseMetrics.ProducedBytes,
				RuntimeOutboundThroughputBps: invokeRequest.InvokeResponseMetrics.OutboundThroughputBps,
			}
		}
		return interop.InvokeSuccess{}, invokeFailure
	}

	var invokeCompletionTimeNs int64
	if responseTimeNs := execCtx.registrationService.GetRuntime().GetRuntimeDescription().State.ResponseTimeNs; responseTimeNs != 0 {
		invokeCompletionTimeNs = time.Now().UnixNano() - responseTimeNs
	}

	invokeSuccessMsg := interop.InvokeSuccess{
		RuntimeRelease:      appctx.GetRuntimeRelease(execCtx.appCtx),
		NumActiveExtensions: execCtx.registrationService.CountAgents(),
		ExtensionNames:      execCtx.GetExtensionNames(),
		InvokeMetrics: interop.InvokeMetrics{
			InvokeRequestReadTimeNs: invokeMx.rendererMetrics.ReadTime.Nanoseconds(),
			InvokeRequestSizeBytes:  int64(invokeMx.rendererMetrics.SizeBytes),
			RuntimeReadyTime:        invokeMx.runtimeReadyTime,
		},
		InvokeCompletionTimeNs: invokeCompletionTimeNs,
		InvokeReceivedTime:     invokeRequest.InvokeReceivedTime,
	}

	if invokeRequest.InvokeResponseMetrics != nil && interop.IsResponseStreamingMetrics(invokeRequest.InvokeResponseMetrics) {
		invokeSuccessMsg.ResponseMetrics = interop.ResponseMetrics{
			RuntimeTimeThrottledMs:       invokeRequest.InvokeResponseMetrics.TimeShapedNs / int64(time.Millisecond),
			RuntimeProducedBytes:         invokeRequest.InvokeResponseMetrics.ProducedBytes,
			RuntimeOutboundThroughputBps: invokeRequest.InvokeResponseMetrics.OutboundThroughputBps,
		}
	}

	if execCtx.telemetryAPIEnabled {
		invokeSuccessMsg.LogsAPIMetrics = interop.MergeSubscriptionMetrics(execCtx.logsSubscriptionAPI.FlushMetrics(), execCtx.telemetrySubscriptionAPI.FlushMetrics())
	}

	return invokeSuccessMsg, nil
}

func reinitialize(execCtx *rapidContext) {
	execCtx.appCtx.Delete(appctx.AppCtxInvokeErrorResponseKey)
	execCtx.appCtx.Delete(appctx.AppCtxRuntimeReleaseKey)
	execCtx.appCtx.Delete(appctx.AppCtxFirstFatalErrorKey)
	execCtx.renderingService.SetRenderer(nil)
	execCtx.initDone = false
	execCtx.registrationService.Clear()
	execCtx.initFlow.Clear()
	execCtx.invokeFlow.Clear()
	if execCtx.telemetryAPIEnabled {
		execCtx.logsSubscriptionAPI.Clear()
		execCtx.telemetrySubscriptionAPI.Clear()
	}
}

// handle notification of reset
func handleReset(execCtx *rapidContext, resetEvent *interop.Reset, invokeReceivedTime int64, invokeResponseMetrics *interop.InvokeResponseMetrics) (interop.ResetSuccess, *interop.ResetFailure) {
	log.Warnf("Reset initiated: %s", resetEvent.Reason)

	// Only send RuntimeDone event if we get a reset during an Invoke
	if resetEvent.Reason == "failure" || resetEvent.Reason == "timeout" {
		runtimeDoneEventData := telemetry.InvokeRuntimeDoneData{
			Status:          resetEvent.Reason,
			InternalMetrics: invokeResponseMetrics,
			Metrics:         telemetry.GetRuntimeDoneInvokeMetrics(invokeReceivedTime, invokeResponseMetrics, metering.Monotime()),
			Tracing:         telemetry.BuildTracingCtx(model.XRayTracingType, resetEvent.TraceID, resetEvent.LambdaSegmentID),
			Spans:           telemetry.GetRuntimeDoneSpans(invokeReceivedTime, invokeResponseMetrics),
		}
		if err := execCtx.eventsAPI.SendRuntimeDone(runtimeDoneEventData); err != nil {
			log.Errorf("Failed to send RUNDONE: %s", err)
		}
	}

	extensionsResetMs, resetTimeout, _ := execCtx.shutdownContext.shutdown(execCtx, resetEvent.DeadlineNs, resetEvent.Reason)

	log.Info("Starting runtime domain")
	err := execCtx.supervisor.Start(&supvmodel.StartRequest{
		Domain: RuntimeDomain,
	})
	if err != nil {
		log.WithError(err).Panic("Failed booting runtime domain")
	}
	execCtx.runtimeDomainGeneration++

	// Only used by standalone for more indepth assertions.
	var fatalErrorType fatalerror.ErrorType

	if execCtx.standaloneMode {
		fatalErrorType, _ = appctx.LoadFirstFatalError(execCtx.appCtx)
	}

	var responseMetrics interop.ResponseMetrics
	if resetEvent.InvokeResponseMetrics != nil && interop.IsResponseStreamingMetrics(resetEvent.InvokeResponseMetrics) {
		responseMetrics.RuntimeTimeThrottledMs = resetEvent.InvokeResponseMetrics.TimeShapedNs / int64(time.Millisecond)
		responseMetrics.RuntimeProducedBytes = resetEvent.InvokeResponseMetrics.ProducedBytes
		responseMetrics.RuntimeOutboundThroughputBps = resetEvent.InvokeResponseMetrics.OutboundThroughputBps
	}

	if resetTimeout {
		return interop.ResetSuccess{}, &interop.ResetFailure{
			ExtensionsResetMs: extensionsResetMs,
			ErrorType:         fatalErrorType,
			ResponseMetrics:   responseMetrics,
		}
	}

	return interop.ResetSuccess{
		ExtensionsResetMs: extensionsResetMs,
		ErrorType:         fatalErrorType,
		ResponseMetrics:   responseMetrics,
	}, nil
}

// handle notification of shutdown
func handleShutdown(execCtx *rapidContext, shutdownEvent *interop.Shutdown, reason string) interop.ShutdownSuccess {
	log.Warnf("Shutdown initiated: %s", reason)
	// TODO Handle shutdown error
	_, _, _ = execCtx.shutdownContext.shutdown(execCtx, shutdownEvent.DeadlineNs, reason)

	// Only used by standalone for more indepth assertions.
	var fatalErrorType fatalerror.ErrorType

	if execCtx.standaloneMode {
		fatalErrorType, _ = appctx.LoadFirstFatalError(execCtx.appCtx)
	}

	return interop.ShutdownSuccess{ErrorType: fatalErrorType}
}

func handleRestore(execCtx *rapidContext, restore *interop.Restore) error {
	err := execCtx.credentialsService.UpdateCredentials(restore.AwsKey, restore.AwsSecret, restore.AwsSession, restore.CredentialsExpiry)
	restoreStatus := telemetry.RuntimeDoneSuccess

	defer func() {
		sendRestoreRuntimeDoneLogEvent(execCtx, restoreStatus)
	}()

	if err != nil {
		return fmt.Errorf("error when updating credentials: %s", err)
	}
	renderer := rendering.NewRestoreRenderer()
	execCtx.renderingService.SetRenderer(renderer)

	registrationService := execCtx.registrationService
	runtime := registrationService.GetRuntime()

	// If runtime has not called /restore/next then just return
	// instead of releasing the Runtime since there is no need to release.
	// Then the runtime should be released only during Invoke
	if runtime.GetState() != runtime.RuntimeRestoreReadyState {
		restoreStatus = telemetry.RuntimeDoneSuccess
		log.Infof("Runtime is in state: %s just returning", runtime.GetState().Name())
		return nil
	}

	runtime.Release()

	initFlow := execCtx.initFlow
	err = initFlow.AwaitRuntimeReady()

	if err != nil {
		restoreStatus = telemetry.RuntimeDoneFailure
	} else {
		restoreStatus = telemetry.RuntimeDoneSuccess
	}

	return err
}

func start(signalCtx context.Context, execCtx *rapidContext) {
	// Start Runtime API Server
	err := execCtx.server.Listen()
	if err != nil {
		log.WithError(err).Panic("Runtime API Server failed to listen")
	}

	go func() { execCtx.server.Serve(signalCtx) }()

	// Note, most of initialization code should run before blocking to receive START,
	// code before START runs in parallel with code downloads.
}

func sendRestoreRuntimeDoneLogEvent(execCtx *rapidContext, status string) {
	if err := execCtx.eventsAPI.SendRestoreRuntimeDone(status); err != nil {
		log.Errorf("Failed to send RESTRD: %s", err)
	}
}

func sendInitRuntimeDoneLogEvent(execCtx *rapidContext, sandboxType interop.SandboxType, status string) {
	initSource := interop.InferTelemetryInitSource(execCtx.initCachingEnabled, sandboxType)

	runtimeDoneData := &telemetry.InitRuntimeDoneData{
		InitSource: initSource,
		Status:     status,
	}

	if err := execCtx.eventsAPI.SendInitRuntimeDone(runtimeDoneData); err != nil {
		log.Errorf("Failed to send INITRD: %s", err)
	}
}

// This function will log a line if AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, or AWS_SESSION_TOKEN is missing
// This is expected to happen in cases when credentials provider is not needed
func checkCredentials(execCtx *rapidContext, bootstrapEnv map[string]string) {
	credentialsKeys := []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN"}
	missingCreds := []string{}

	for _, credEnvVar := range credentialsKeys {
		if val, keyExists := bootstrapEnv[credEnvVar]; !keyExists || val == "" {
			missingCreds = append(missingCreds, credEnvVar)
		}
	}

	if len(missingCreds) > 0 {
		log.Infof("Starting runtime without %s , Expected?: %t", strings.Join(missingCreds[:], ", "), execCtx.initCachingEnabled)
	}
}
