// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package rapid implements synchronous even dispatch loop.
package rapid

import (
	"bytes"
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
	"go.amzn.com/lambda/rapi/rendering"
	"go.amzn.com/lambda/rapidcore/env"
	supvmodel "go.amzn.com/lambda/supervisor/model"
	"go.amzn.com/lambda/telemetry"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	RuntimeDomain        = "runtime"
	OperatorDomain       = "operator"
	defaultAgentLocation = "/opt/extensions"
	runtimeProcessName   = "runtime"
)

const (
	// Same value as defined in LambdaSandbox minus 1.
	maxExtensionNamesLength = 127
	standaloneShutdownReason = "spindown"
)

var errResetReceived = errors.New("errResetReceived")

type processSupervisor struct {
	supvmodel.ProcessSupervisor
	RootPath string
}

type rapidContext struct {
	interopServer            interop.Server
	server                   *rapi.Server
	appCtx                   appctx.ApplicationContext
	initDone                 bool
	supervisor               processSupervisor
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
	eventsAPI                interop.EventsAPI
	initCachingEnabled       bool
	credentialsService       core.CredentialsService
	handlerExecutionMutex    sync.Mutex
	shutdownContext          *shutdownContext
	logStreamName            string

	RuntimeStartedTime         int64
	RuntimeOverheadStartedTime int64
	InvokeResponseMetrics      *interop.InvokeResponseMetrics
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
		extensionInitData := interop.ExtensionInitData{
			AgentName:     agent.Name,
			State:         agent.State,
			ErrorType:     agent.ErrorType,
			Subscriptions: agent.Subscriptions,
		}
		execCtx.eventsAPI.SendExtensionInit(extensionInitData)
	}
}

func agentLaunchError(agent *core.ExternalAgent, appCtx appctx.ApplicationContext, launchError error) {
	if err := agent.LaunchError(launchError); err != nil {
		log.Warnf("LaunchError transition fail for %s from %s: %s", agent, agent.GetState().Name(), err)
	}
	appctx.StoreFirstFatalError(appCtx, fatalerror.AgentLaunchError)
}

func doInitExtensions(domain string, agentPaths []string, execCtx *rapidContext, env *env.Environment) error {
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

		err = execCtx.supervisor.Exec(context.Background(), &supvmodel.ExecRequest{
			Domain: domain,
			Name:   agentName,
			Path:   agentPath,
			Env:    &env,
			Logging: supvmodel.Logging{
				Managed: supvmodel.ManagedLogging{
					Topic: supvmodel.RtExtensionManagedLoggingTopic,
					Formats: []supvmodel.ManagedLoggingFormat{
						supvmodel.LineBasedManagedLogging,
					},
				},
			},
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
			execCtx.eventsAPI.SendImageErrorLog(interop.ImageErrorLogData(formattedLog))
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
			execCtx.eventsAPI.SendImageErrorLog(interop.ImageErrorLogData(formattedLog))
		} else {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalerror.InvalidWorkingDir)
		}
		return []string{}, map[string]string{}, "", []*os.File{}, err
	}

	bootstrapExtraFiles := runtimeBootstrap.ExtraFiles()

	return bootstrapCmd, bootstrapEnv, bootstrapCwd, bootstrapExtraFiles, nil
}

func (c *rapidContext) watchEvents(events <-chan supvmodel.Event) {
	for event := range events {
		var err error
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
}

// subscribe to /events for runtime domain in supervisor
func setupEventsWatcher(execCtx *rapidContext) error {
	eventsRequest := supvmodel.EventsRequest{
		Domain: RuntimeDomain,
	}

	events, err := execCtx.supervisor.Events(context.Background(), &eventsRequest)
	if err != nil {
		log.Errorf("Could not get events stream from supervisor: %s", err)
		return err
	}

	go execCtx.watchEvents(events)
	return nil
}

func doRuntimeDomainInit(execCtx *rapidContext, sbInfoFromInit interop.SandboxInfoFromInit, phase interop.LifecyclePhase) error {
	initStartTime := metering.Monotime()
	sendInitStartLogEvent(execCtx, sbInfoFromInit.SandboxType, phase)
	defer sendInitReportLogEvent(execCtx, sbInfoFromInit.SandboxType, initStartTime, phase)

	execCtx.xray.RecordInitStartTime()
	defer execCtx.xray.RecordInitEndTime()

	defer func() {
		if extensions.AreEnabled() {
			logAgentsInitStatus(execCtx)
		}
	}()

	execCtx.runtimeDomainGeneration++

	if extensions.AreEnabled() {
		runtimeExtensions := agents.ListExternalAgentPaths(defaultAgentLocation,
			execCtx.supervisor.RootPath)
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
	err := registrationService.PreregisterRuntime(runtime)
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

	err = execCtx.supervisor.Exec(context.Background(), &supvmodel.ExecRequest{
		Domain: RuntimeDomain,
		Name:   name,
		Cwd:    &bootstrapCwd,
		Path:   bootstrapCmd[0],
		Args:   bootstrapCmd[1:],
		Env:    &bootstrapEnv,
		Logging: supvmodel.Logging{
			Managed: supvmodel.ManagedLogging{
				Topic: supvmodel.RuntimeManagedLoggingTopic,
				Formats: []supvmodel.ManagedLoggingFormat{
					supvmodel.LineBasedManagedLogging,
					supvmodel.MessageBasedManagedLogging,
				},
			},
		},
		StdoutWriter: runtimeStdoutWriter,
		StderrWriter: runtimeStderrWriter,
		ExtraFiles:   &bootstrapExtraFiles,
	})

	runtimeDoneStatus := telemetry.RuntimeDoneSuccess

	defer func() {
		sendInitRuntimeDoneLogEvent(execCtx, sbInfoFromInit.SandboxType, runtimeDoneStatus, phase)
	}()

	if err != nil {
		if fatalError, formattedLog, hasError := sbInfoFromInit.RuntimeBootstrap.CachedFatalError(err); hasError {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalError)
			execCtx.eventsAPI.SendImageErrorLog(interop.ImageErrorLogData(formattedLog))
		} else {
			appctx.StoreFirstFatalError(execCtx.appCtx, fatalerror.InvalidEntrypoint)
		}

		runtimeDoneStatus = telemetry.RuntimeDoneError
		return err
	}

	execCtx.shutdownContext.createExitedChannel(name)

	if err := initFlow.AwaitRuntimeRestoreReady(); err != nil {
		runtimeDoneStatus = telemetry.RuntimeDoneError
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
			runtimeDoneStatus = telemetry.RuntimeDoneError
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

func doInvoke(execCtx *rapidContext, invokeRequest *interop.Invoke, mx *invokeMetrics, sbInfoFromInit interop.SandboxInfoFromInit, requestBuffer *bytes.Buffer) error {
	execCtx.eventsAPI.SetCurrentRequestID(interop.RequestID(invokeRequest.ID))
	appCtx := execCtx.appCtx

	xray := execCtx.xray
	xray.Configure(invokeRequest)

	ctx := context.Background()

	return xray.CaptureInvokeSegment(ctx, xray.WithErrorCause(ctx, appCtx, func(ctx context.Context) error {
		telemetryTracingCtx := xray.BuildTracingCtxForStart()

		if !execCtx.initDone {
			// do inline init
			if err := xray.CaptureInitSubsegment(ctx, func(ctx context.Context) error {
				return doRuntimeDomainInit(execCtx, sbInfoFromInit, interop.LifecyclePhaseInvoke)
			}); err != nil {
				sendInvokeStartLogEvent(execCtx, invokeRequest.ID, telemetryTracingCtx)
				return err
			}
		} else if sbInfoFromInit.SandboxType != interop.SandboxPreWarmed && !execCtx.initCachingEnabled {
			xray.SendInitSubsegmentWithRecordedTimesOnce(ctx)
		}

		xray.SendRestoreSubsegmentWithRecordedTimesOnce(ctx)

		sendInvokeStartLogEvent(execCtx, invokeRequest.ID, telemetryTracingCtx)

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
			renderer := rendering.NewInvokeRenderer(ctx, invokeRequest, requestBuffer, xray.BuildTracingHeader())
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
			execCtx.SetRuntimeStartedTime(metering.Monotime())
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
			execCtx.SetRuntimeOverheadStartedTime(metering.Monotime())
			//TODO handle Supervisors listening channel
			return invokeFlow.AwaitRuntimeReady()
		}); err != nil {
			return err
		}
		mx.runtimeReadyTime = metering.Monotime()

		runtimeDoneEventData := interop.InvokeRuntimeDoneData{
			Status:          telemetry.RuntimeDoneSuccess,
			Metrics:         telemetry.GetRuntimeDoneInvokeMetrics(execCtx.RuntimeStartedTime, invokeRequest.InvokeResponseMetrics, mx.runtimeReadyTime),
			InternalMetrics: invokeRequest.InvokeResponseMetrics,
			Tracing:         xray.BuildTracingCtxAfterInvokeComplete(),
			Spans:           execCtx.eventsAPI.GetRuntimeDoneSpans(execCtx.RuntimeStartedTime, invokeRequest.InvokeResponseMetrics, execCtx.RuntimeOverheadStartedTime, mx.runtimeReadyTime),
		}
		log.Info(runtimeDoneEventData.String())
		if err := execCtx.eventsAPI.SendInvokeRuntimeDone(runtimeDoneEventData); err != nil {
			log.Errorf("Failed to send INVOKE RTDONE: %s", err)
		}

		// Extensions overhead
		if execCtx.HasActiveExtensions() {
			extensionOverheadStartTime := metering.Monotime()
			execCtx.interopServer.SendRuntimeReady()
			log.Debug("Await agents ready")
			//TODO handle Supervisors listening channel
			if err := invokeFlow.AwaitAgentsReady(); err != nil {
				log.Warnf("AwaitAgentsReady() = %s", err)
				return err
			}
			extensionOverheadEndTime := metering.Monotime()
			extensionOverheadMsSpan := interop.Span{
				Name:       "extensionOverhead",
				Start:      telemetry.GetEpochTimeInISO8601FormatFromMonotime(extensionOverheadStartTime),
				DurationMs: telemetry.CalculateDuration(extensionOverheadStartTime, extensionOverheadEndTime),
			}
			if err := execCtx.eventsAPI.SendReportSpan(extensionOverheadMsSpan); err != nil {
				log.WithError(err).Error("Failed to create REPORT Span")
			}
		}

		return nil
	}))
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
		AccountID:         initRequest.AccountID,
		FunctionName:      initRequest.FunctionName,
		FunctionVersion:   initRequest.FunctionVersion,
		InstanceMaxMemory: initRequest.InstanceMaxMemory,
		Handler:           initRequest.Handler,
		RuntimeInfo:       initRequest.RuntimeInfo,
	})
	c.SetLogStreamName(initRequest.LogStreamName)

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
		AccountID:         initRequest.AccountID,
		FunctionName:      initRequest.FunctionName,
		FunctionVersion:   initRequest.FunctionVersion,
		InstanceMaxMemory: initRequest.InstanceMaxMemory,
		Handler:           initRequest.Handler,
		RuntimeInfo:       initRequest.RuntimeInfo,
	})
	c.SetLogStreamName(initRequest.LogStreamName)

	c.credentialsService.SetCredentials(initCachingToken, initRequest.AwsKey, initRequest.AwsSecret, initRequest.AwsSession, initRequest.CredentialsExpiry)

	return initRequest, nil
}

func handleInit(execCtx *rapidContext, initRequest *interop.Init, initSuccessResponse chan<- interop.InitSuccess, initFailureResponse chan<- interop.InitFailure) {
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

	if err := setupEventsWatcher(execCtx); err != nil {
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
		if err := doRuntimeDomainInit(execCtx, sbInfo, interop.LifecyclePhaseInit); err != nil {
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

func handleInvoke(execCtx *rapidContext, invokeRequest *interop.Invoke, sbInfoFromInit interop.SandboxInfoFromInit, requestBuffer *bytes.Buffer, responseSender interop.InvokeResponseSender) (interop.InvokeSuccess, *interop.InvokeFailure) {
	appctx.StoreResponseSender(execCtx.appCtx, responseSender)
	invokeMx := invokeMetrics{}

	if err := doInvoke(execCtx, invokeRequest, &invokeMx, sbInfoFromInit, requestBuffer); err != nil {
		log.WithError(err).WithField("InvokeID", invokeRequest.ID).Error("Invoke failed")
		invokeFailure := handleInvokeError(execCtx, invokeRequest, &invokeMx, err)
		invokeFailure.InvokeResponseMode = invokeRequest.InvokeResponseMode

		if invokeRequest.InvokeResponseMetrics != nil && interop.IsResponseStreamingMetrics(invokeRequest.InvokeResponseMetrics) {
			invokeFailure.ResponseMetrics = interop.ResponseMetrics{
				RuntimeResponseLatencyMs:     telemetry.CalculateDuration(execCtx.RuntimeStartedTime, invokeRequest.InvokeResponseMetrics.StartReadingResponseMonoTimeMs),
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
		InvokeResponseMode:     invokeRequest.InvokeResponseMode,
	}

	if invokeRequest.InvokeResponseMetrics != nil && interop.IsResponseStreamingMetrics(invokeRequest.InvokeResponseMetrics) {
		invokeSuccessMsg.ResponseMetrics = interop.ResponseMetrics{
			RuntimeResponseLatencyMs:     telemetry.CalculateDuration(execCtx.RuntimeStartedTime, invokeRequest.InvokeResponseMetrics.StartReadingResponseMonoTimeMs),
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
	execCtx.appCtx.Delete(appctx.AppCtxInvokeErrorTraceDataKey)
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
func handleReset(execCtx *rapidContext, resetEvent *interop.Reset, runtimeStartedTime int64, invokeResponseMetrics *interop.InvokeResponseMetrics) (interop.ResetSuccess, *interop.ResetFailure) {
	log.Warnf("Reset initiated: %s", resetEvent.Reason)

	// Only send RuntimeDone event if we get a reset during an Invoke
	if resetEvent.Reason == "failure" || resetEvent.Reason == "timeout" {
		var errorType *string
		if resetEvent.Reason == "failure" {
			firstFatalError, found := appctx.LoadFirstFatalError(execCtx.appCtx)
			if !found {
				firstFatalError = fatalerror.SandboxFailure
			}
			stringifiedError := string(firstFatalError)
			errorType = &stringifiedError
		}

		var status string
		if resetEvent.Reason == "timeout" {
			status = "timeout"
		} else if strings.HasPrefix(*errorType, "Sandbox.") {
			status = "failure"
		} else {
			status = "error"
		}

		var runtimeReadyTime int64 = metering.Monotime()
		runtimeDoneEventData := interop.InvokeRuntimeDoneData{
			Status:          status,
			InternalMetrics: invokeResponseMetrics,
			Metrics:         telemetry.GetRuntimeDoneInvokeMetrics(runtimeStartedTime, invokeResponseMetrics, runtimeReadyTime),
			Tracing:         execCtx.xray.BuildTracingCtxAfterInvokeComplete(),
			Spans:           execCtx.eventsAPI.GetRuntimeDoneSpans(runtimeStartedTime, invokeResponseMetrics, execCtx.RuntimeOverheadStartedTime, runtimeReadyTime),
			ErrorType:       errorType,
		}
		if err := execCtx.eventsAPI.SendInvokeRuntimeDone(runtimeDoneEventData); err != nil {
			log.Errorf("Failed to send INVOKE RTDONE: %s", err)
		}
	}

	extensionsResetMs, resetTimeout, _ := execCtx.shutdownContext.shutdown(execCtx, resetEvent.DeadlineNs, resetEvent.Reason)

	execCtx.runtimeDomainGeneration++

	// Only used by standalone for more indepth assertions.
	var fatalErrorType fatalerror.ErrorType

	if execCtx.standaloneMode {
		fatalErrorType, _ = appctx.LoadFirstFatalError(execCtx.appCtx)
	}

	// TODO: move interop.ResponseMetrics{} to a factory method and initialize it there.
	// Initialization is very similar in handleInvoke's invokeFailure.ResponseMetrics and
	// invokeSuccessMsg.ResponseMetrics
	var responseMetrics interop.ResponseMetrics
	if resetEvent.InvokeResponseMetrics != nil && interop.IsResponseStreamingMetrics(resetEvent.InvokeResponseMetrics) {
		responseMetrics.RuntimeResponseLatencyMs = telemetry.CalculateDuration(execCtx.RuntimeStartedTime, resetEvent.InvokeResponseMetrics.StartReadingResponseMonoTimeMs)
		responseMetrics.RuntimeTimeThrottledMs = resetEvent.InvokeResponseMetrics.TimeShapedNs / int64(time.Millisecond)
		responseMetrics.RuntimeProducedBytes = resetEvent.InvokeResponseMetrics.ProducedBytes
		responseMetrics.RuntimeOutboundThroughputBps = resetEvent.InvokeResponseMetrics.OutboundThroughputBps
	}

	if resetTimeout {
		return interop.ResetSuccess{}, &interop.ResetFailure{
			ExtensionsResetMs:  extensionsResetMs,
			ErrorType:          fatalErrorType,
			ResponseMetrics:    responseMetrics,
			InvokeResponseMode: resetEvent.InvokeResponseMode,
		}
	}

	return interop.ResetSuccess{
		ExtensionsResetMs:  extensionsResetMs,
		ErrorType:          fatalErrorType,
		ResponseMetrics:    responseMetrics,
		InvokeResponseMode: resetEvent.InvokeResponseMode,
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

func handleRestore(execCtx *rapidContext, restore *interop.Restore) (interop.RestoreResult, error) {
	err := execCtx.credentialsService.UpdateCredentials(restore.AwsKey, restore.AwsSecret, restore.AwsSession, restore.CredentialsExpiry)
	restoreStatus := telemetry.RuntimeDoneSuccess

	restoreResult := interop.RestoreResult{}

	defer func() {
		sendRestoreRuntimeDoneLogEvent(execCtx, restoreStatus)
	}()

	if err != nil {
		log.Infof("error when updating credentials: %s", err)
		return restoreResult, interop.ErrRestoreUpdateCredentials
	}

	renderer := rendering.NewRestoreRenderer()
	execCtx.renderingService.SetRenderer(renderer)

	registrationService := execCtx.registrationService
	runtime := registrationService.GetRuntime()

	execCtx.SetLogStreamName(restore.LogStreamName)

	// If runtime has not called /restore/next then just return
	// instead of releasing the Runtime since there is no need to release.
	// Then the runtime should be released only during Invoke
	if runtime.GetState() != runtime.RuntimeRestoreReadyState {
		restoreStatus = telemetry.RuntimeDoneSuccess
		log.Infof("Runtime is in state: %s just returning", runtime.GetState().Name())

		return restoreResult, nil
	}

	deadlineNs := time.Now().Add(time.Duration(restore.RestoreHookTimeoutMs) * time.Millisecond).UnixNano()

	ctx, ctxCancel := context.WithDeadline(context.Background(), time.Unix(0, deadlineNs))

	defer ctxCancel()

	startTime := metering.Monotime()

	runtime.Release()

	initFlow := execCtx.initFlow
	err = initFlow.AwaitRuntimeReadyWithDeadline(ctx)

	fatalErrorType, fatalErrorFound := appctx.LoadFirstFatalError(execCtx.appCtx)

	// If there is an error occured when waiting runtime to complete the restore hook execution,
	// check if there is any error stored in appctx to get the root cause error type
	// Runtime.ExitError is an example to such a scenario
	if fatalErrorFound {
		err = fmt.Errorf(string(fatalErrorType))
	}

	if err != nil {
		restoreStatus = telemetry.RuntimeDoneError
	}

	endTime := metering.Monotime()
	restoreDuration := time.Duration(endTime - startTime)
	restoreResult.RestoreMs = restoreDuration.Milliseconds()

	return restoreResult, err
}

func startRuntimeAPI(ctx context.Context, execCtx *rapidContext) {
	// Start Runtime API Server
	err := execCtx.server.Listen()
	if err != nil {
		log.WithError(err).Panic("Runtime API Server failed to listen")
	}

	execCtx.server.Serve(ctx) // blocking until server exits

	// Note, most of initialization code should run before blocking to receive START,
	// code before START runs in parallel with code downloads.
}

func getFirstFatalError(execCtx *rapidContext, status string) *string {
	if status == telemetry.RuntimeDoneSuccess {
		return nil
	}

	firstFatalError, found := appctx.LoadFirstFatalError(execCtx.appCtx)
	if !found {
		// We will set errorType to "Runtime.Unknown" in case of INIT timeout and RESTORE timeout
		// This is a trade-off we are willing to make. We will improve this later
		firstFatalError = fatalerror.RuntimeUnknown
	}
	stringifiedError := string(firstFatalError)
	return &stringifiedError
}

func sendRestoreRuntimeDoneLogEvent(execCtx *rapidContext, status string) {
	firstFatalError := getFirstFatalError(execCtx, status)

	restoreRuntimeDoneData := interop.RestoreRuntimeDoneData{
		Status:    status,
		ErrorType: firstFatalError,
	}

	if err := execCtx.eventsAPI.SendRestoreRuntimeDone(restoreRuntimeDoneData); err != nil {
		log.Errorf("Failed to send RESTORE RTDONE: %s", err)
	}
}

func sendInitStartLogEvent(execCtx *rapidContext, sandboxType interop.SandboxType, phase interop.LifecyclePhase) {
	initPhase, err := telemetry.InitPhaseFromLifecyclePhase(phase)
	if err != nil {
		log.Errorf("failed to convert lifecycle phase into init phase: %s", err)
		return
	}

	functionMetadata := execCtx.registrationService.GetFunctionMetadata()
	initStartData := interop.InitStartData{
		InitializationType: telemetry.InferInitType(execCtx.initCachingEnabled, sandboxType),
		RuntimeVersion:     functionMetadata.RuntimeInfo.Version,
		RuntimeVersionArn:  functionMetadata.RuntimeInfo.Arn,
		FunctionName:       functionMetadata.FunctionName,
		FunctionVersion:    functionMetadata.FunctionVersion,
		// based on https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/semantic_conventions/faas.md
		// we're sending the logStream as the instance id
		InstanceID:        execCtx.logStreamName,
		InstanceMaxMemory: functionMetadata.InstanceMaxMemory,
		Phase:             initPhase,
	}
	log.Info(initStartData.String())

	if err := execCtx.eventsAPI.SendInitStart(initStartData); err != nil {
		log.Errorf("Failed to send INIT START: %s", err)
	}
}

func sendInitRuntimeDoneLogEvent(execCtx *rapidContext, sandboxType interop.SandboxType, status string, phase interop.LifecyclePhase) {
	initPhase, err := telemetry.InitPhaseFromLifecyclePhase(phase)
	if err != nil {
		log.Errorf("failed to convert lifecycle phase into init phase: %s", err)
		return
	}

	firstFatalError := getFirstFatalError(execCtx, status)

	initRuntimeDoneData := interop.InitRuntimeDoneData{
		InitializationType: telemetry.InferInitType(execCtx.initCachingEnabled, sandboxType),
		Status:             status,
		Phase:              initPhase,
		ErrorType:          firstFatalError,
	}

	log.Info(initRuntimeDoneData.String())

	if err := execCtx.eventsAPI.SendInitRuntimeDone(initRuntimeDoneData); err != nil {
		log.Errorf("Failed to send INIT RTDONE: %s", err)
	}
}

func sendInitReportLogEvent(
	execCtx *rapidContext,
	sandboxType interop.SandboxType,
	initStartMonotime int64,
	phase interop.LifecyclePhase,
) {
	initPhase, err := telemetry.InitPhaseFromLifecyclePhase(phase)
	if err != nil {
		log.Errorf("failed to convert lifecycle phase into init phase: %s", err)
		return
	}

	initReportData := interop.InitReportData{
		InitializationType: telemetry.InferInitType(execCtx.initCachingEnabled, sandboxType),
		Metrics: interop.InitReportMetrics{
			DurationMs: telemetry.CalculateDuration(initStartMonotime, metering.Monotime()),
		},
		Phase: initPhase,
	}
	log.Info(initReportData.String())

	if err = execCtx.eventsAPI.SendInitReport(initReportData); err != nil {
		log.Errorf("Failed to send INIT REPORT: %s", err)
	}
}

func sendInvokeStartLogEvent(execCtx *rapidContext, invokeRequestID string, tracingCtx *interop.TracingCtx) {
	invokeStartData := interop.InvokeStartData{
		RequestID: invokeRequestID,
		Version:   execCtx.registrationService.GetFunctionMetadata().FunctionVersion,
		Tracing:   tracingCtx,
	}
	log.Info(invokeStartData.String())

	if err := execCtx.eventsAPI.SendInvokeStart(invokeStartData); err != nil {
		log.Errorf("Failed to send INVOKE START: %s", err)
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
