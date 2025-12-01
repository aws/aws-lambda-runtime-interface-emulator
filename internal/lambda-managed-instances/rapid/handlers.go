// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/agents"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/invoke"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	internalmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	supvmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/supervisor/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils/invariant"
)

var ErrorInvalidEntryPoint = errors.New("invalid entrypoint, runtime process spawn failed")

const (
	runtimeProcessName = "runtime"

	maxExtensionNamesLength = 127
)

type processSupervisor struct {
	supvmodel.ProcessSupervisor
}

type rapidContext struct {
	interopServer            interop.Server
	initExecutionData        interop.InitExecutionData
	server                   *rapi.Server
	appCtx                   appctx.ApplicationContext
	supervisor               processSupervisor
	initFlow                 core.InitFlowSynchronization
	registrationService      core.RegistrationService
	renderingService         *rendering.EventRenderingService
	telemetrySubscriptionAPI telemetry.SubscriptionAPI
	logsEgressAPI            telemetry.StdLogsEgressAPI
	eventsAPI                interop.EventsAPI
	invokeRouter             *invoke.InvokeRouter
	initMetrics              interop.InitMetrics

	shutdownContext            *shutdownContext
	fileUtils                  utils.FileUtil
	RuntimeStartedTime         time.Time
	RuntimeOverheadStartedTime time.Time

	processTermChan chan rapidmodel.AppError
}

var _ interop.RapidContext = (*rapidContext)(nil)

func (r *rapidContext) ProcessTerminationNotifier() <-chan rapidmodel.AppError {
	return r.processTermChan
}

func (r *rapidContext) GetExtensionNames() string {
	var extensionNamesList []string
	for _, agent := range r.registrationService.AgentsInfo() {
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

func doInitExtensions(ctx context.Context, execCtx *rapidContext) rapidmodel.AppError {
	initFlow := execCtx.registrationService.InitFlow()

	bootstraps := agents.ListExternalAgentPaths(execCtx.fileUtils, agents.ExtensionsDir, "/")

	if err := initFlow.SetExternalAgentsRegisterCount(uint16(len(bootstraps))); err != nil {
		return rapidmodel.WrapErrorIntoPlatformFatalError(err, rapidmodel.ErrorAgentCountRegistrationFailed)
	}

	for _, agentPath := range bootstraps {

		agent, err := execCtx.registrationService.CreateExternalAgent(path.Base(agentPath))
		if err != nil {
			return rapidmodel.WrapErrorIntoPlatformFatalError(err, rapidmodel.ErrorAgentExtensionCreationFailed)
		}

		if execCtx.registrationService.CountAgents() > core.MaxAgentsAllowed {
			if err := agent.LaunchError(rapidmodel.ErrorAgentTooManyExtensions); err != nil {
				logging.Warn(ctx, "LaunchError transition fail", "agent", agent, "state", agent.GetState().Name(), "err", err)
			}
			customerErr := rapidmodel.WrapErrorIntoCustomerInvalidError(nil, rapidmodel.ErrorAgentTooManyExtensions)
			appctx.StoreFirstFatalError(execCtx.appCtx, customerErr)
			return customerErr
		}

		agentStdoutWriter, agentStderrWriter, err := execCtx.logsEgressAPI.GetExtensionSockets()
		if err != nil {
			return rapidmodel.WrapErrorIntoPlatformFatalError(
				fmt.Errorf("failed to get Extension Sockets: %w", err),
				rapidmodel.ErrSandboxLogSocketsUnavailable)
		}
		agentName := fmt.Sprintf("extension-%s", path.Base(agentPath))
		logging.Debug(ctx, "Starting extension", "name", agentName)
		execReq := &supvmodel.ExecRequest{
			Name: agentName,
			Path: agentPath,
			Env:  &execCtx.initExecutionData.ExtensionEnv,
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
		}
		if err := execCtx.supervisor.Exec(ctx, execReq); err != nil {
			logging.Warn(ctx, "Could not exec extension process", "err", err, "agent", agentName)
			errorType := core.MapErrorToAgentInfoErrorType(err)
			if launchErr := agent.LaunchError(errorType); launchErr != nil {
				logging.Warn(ctx, "LaunchError transition fail", "agent", agent, "state", agent.GetState().Name(), "error", launchErr)
			}

			customerErr := rapidmodel.WrapErrorIntoCustomerInvalidError(err, errorType)
			appctx.StoreFirstFatalError(execCtx.appCtx, customerErr)
			return customerErr
		}

		execCtx.shutdownContext.createExitedChannel(agentName)
	}

	if err := initFlow.AwaitExternalAgentsRegistered(ctx); err != nil {
		return resolveGateError(ctx, execCtx.appCtx, err, rapidmodel.ErrorAgentRegistrationFailed)
	}

	return nil
}

func prepareRuntimeBootstrap(execCtx *rapidContext, sbStaticData interop.InitExecutionData) ([]string, internalmodel.KVMap, string, *rapidmodel.CustomerError) {
	cmd := sbStaticData.Runtime.ExecConfig.Cmd
	env := sbStaticData.Runtime.ExecConfig.Env
	cwd := sbStaticData.Runtime.ExecConfig.WorkingDir

	if sbStaticData.StaticData.ArtefactType == internalmodel.ArtefactTypeZIP {
		slog.Debug("No bootstrap command provided, searching default locations")
		var bootstrap []string
		for _, path := range []string{"/var/runtime/bootstrap", "/var/task/bootstrap", "/opt/bootstrap"} {
			info, err := execCtx.fileUtils.Stat(path)
			if err == nil && !info.IsDir() {
				slog.Debug("Found bootstrap", "path", path)
				bootstrap = []string{path}
				cwd = "/var/task"
				break
			}
			slog.Warn("Ignoring invalid bootstrap path", "path", path, "err", err)
		}

		if len(bootstrap) == 0 {
			slog.Error("No valid bootstrap binary found in default locations")
			return failBootstrap(execCtx, sbStaticData, rapidmodel.InvalidEntrypoint, ErrorInvalidEntryPoint)
		}
		cmd = bootstrap
	}

	if _, err := execCtx.fileUtils.Stat(cwd); err != nil {
		slog.Warn("Invalid working directory", "cwd", cwd, "error", err)
		return failBootstrap(execCtx, sbStaticData, rapidmodel.InvalidWorkingDir, err)
	}

	return cmd, env, cwd, nil
}

func failBootstrap(execCtx *rapidContext, sbStaticData interop.InitExecutionData, errType rapidmodel.RuntimeExecErrorType, err error) ([]string, internalmodel.KVMap, string, *rapidmodel.CustomerError) {
	runtimeErr := &rapidmodel.RuntimeExecError{Type: errType, Err: err}
	customerErr := rapidmodel.WrapErrorIntoCustomerInvalidError(err, errType.FatalErrorType())
	appctx.StoreFirstFatalError(execCtx.appCtx, customerErr)
	execCtx.eventsAPI.SendImageError(interop.ImageErrorLogData{
		ExecError:  *runtimeErr,
		ExecConfig: sbStaticData.Runtime.ExecConfig,
	})
	return nil, nil, "", &customerErr
}

func (r *rapidContext) watchEvents(events <-chan supvmodel.Event) {
	for event := range events {
		slog.Debug("Received event", "event", event)

		switch event.Event.EvType {
		case supvmodel.EventLossType:

			invariant.Violatef("Lost %d events from supervisor", *event.Event.Size)

		case supvmodel.ProcessTerminationType:

			r.shutdownContext.processTermination(*event.Event.ProcessTerminated(), r)
		}
	}
}

func setupEventsWatcher(ctx context.Context, execCtx *rapidContext) error {
	events, err := execCtx.supervisor.Events(ctx)
	if err != nil {

		return fmt.Errorf("could not get runtime events stream from supervisor: %w", err)
	}
	go execCtx.watchEvents(events)
	return nil
}

func runtimeInitWithTelemetry(ctx context.Context, execCtx *rapidContext, phase interop.LifecyclePhase) rapidmodel.AppError {
	execCtx.initMetrics.TriggerStartRequest()
	telemetry.SendInitStartLogEvent(execCtx.eventsAPI, execCtx.initExecutionData.FunctionMetadata, execCtx.initExecutionData.LogStreamName, phase)

	err := doRuntimeInit(ctx, execCtx, phase)
	execCtx.initMetrics.TriggerInitCustomerPhaseDone()

	telemetry.SendAgentsInitStatus(execCtx.eventsAPI, execCtx.registrationService.AgentsInfo())
	execCtx.initMetrics.SetExtensionsNumber(len(execCtx.registrationService.GetInternalAgents()), len(execCtx.registrationService.GetExternalAgents()))

	telemetry.SendInitReportLogEvent(execCtx.eventsAPI, execCtx.appCtx, execCtx.initMetrics.RunDuration(), phase, err)

	logsAPIMetrics := execCtx.telemetrySubscriptionAPI.FlushMetrics()
	execCtx.initMetrics.SetLogsAPIMetrics(logsAPIMetrics)

	return err
}

func doRuntimeInit(ctx context.Context, execCtx *rapidContext, phase interop.LifecyclePhase) rapidmodel.AppError {
	if err := doInitExtensions(ctx, execCtx); err != nil {
		return err
	}

	execCtx.initMetrics.TriggerStartingRuntime()
	if err := doInitRuntime(ctx, execCtx, phase); err != nil {
		return err
	}
	execCtx.initMetrics.TriggerRuntimeDone()

	if err := waitExtensionsToBeReady(ctx, execCtx); err != nil {
		return err
	}

	return nil
}

func waitExtensionsToBeReady(ctx context.Context, execCtx *rapidContext) rapidmodel.AppError {
	initFlow := execCtx.registrationService.InitFlow()

	execCtx.registrationService.TurnOff()

	if err := initFlow.SetAgentsReadyCount(execCtx.registrationService.GetRegisteredAgentsSize()); err != nil {
		return rapidmodel.WrapErrorIntoCustomerFatalError(err, rapidmodel.ErrorAgentGateCreationFailed)
	}
	if err := initFlow.AwaitAgentsReady(ctx); err != nil {
		return resolveGateError(ctx, execCtx.appCtx, err, rapidmodel.ErrorAgentReadyFailed)
	}

	execCtx.telemetrySubscriptionAPI.TurnOff()

	return nil
}

func doInitRuntime(
	ctx context.Context,
	execCtx *rapidContext,
	phase interop.LifecyclePhase,
) rapidmodel.AppError {
	initFlow := execCtx.registrationService.InitFlow()
	runtimeFsm := core.NewRuntime(initFlow)

	logging.Debug(ctx, "Preregister runtime")
	registrationService := execCtx.registrationService
	if err := registrationService.PreregisterRuntime(runtimeFsm); err != nil {
		return rapidmodel.WrapErrorIntoPlatformFatalError(err, rapidmodel.ErrorRuntimeRegistrationFailed)
	}

	bootstrapCmd, bootstrapEnv, bootstrapCwd, runtimeErr := prepareRuntimeBootstrap(execCtx, execCtx.initExecutionData)
	if runtimeErr != nil {
		return *runtimeErr
	}

	runtimeStdoutWriter, runtimeStderrWriter, err := execCtx.logsEgressAPI.GetRuntimeSockets()
	if err != nil {
		return rapidmodel.WrapErrorIntoPlatformFatalError(err, rapidmodel.ErrSandboxLogSocketsUnavailable)
	}

	name := runtimeProcessName
	slog.Debug("Start runtime", "name", name)

	execReq := &supvmodel.ExecRequest{
		Name: name,
		Cwd:  &bootstrapCwd,
		Path: bootstrapCmd[0],
		Args: bootstrapCmd[1:],
		Env:  &bootstrapEnv,
		Logging: supvmodel.Logging{
			Managed: supvmodel.ManagedLogging{
				Topic:   supvmodel.RuntimeManagedLoggingTopic,
				Formats: execCtx.initExecutionData.RuntimeManagedLoggingFormats,
			},
		},
		StdoutWriter: runtimeStdoutWriter,
		StderrWriter: runtimeStderrWriter,
	}

	if err := execCtx.supervisor.Exec(ctx, execReq); err != nil {
		logging.Warn(ctx, "Could not Exec Runtime process", "err", err)
		execError := rapidmodel.RuntimeExecError{
			Type: rapidmodel.InvalidEntrypoint,
			Err:  ErrorInvalidEntryPoint,
		}
		customerErr := rapidmodel.WrapErrorIntoCustomerInvalidError(ErrorInvalidEntryPoint, execError.Type.FatalErrorType())
		appctx.StoreFirstFatalError(execCtx.appCtx, customerErr)
		telemetry.SendImageError(execCtx.eventsAPI, execError, execCtx.initExecutionData.Runtime.ExecConfig)
		telemetry.SendInitRuntimeDoneLogEvent(execCtx.eventsAPI, execCtx.appCtx, phase, customerErr)

		return customerErr
	}

	execCtx.shutdownContext.createExitedChannel(name)

	if err := initFlow.AwaitRuntimeReady(ctx); err != nil {
		customerErr := resolveGateError(ctx, execCtx.appCtx, err, rapidmodel.ErrorRuntimeReadyFailed)

		if err != interop.ErrTimeout {
			telemetry.SendInitRuntimeDoneLogEvent(execCtx.eventsAPI, execCtx.appCtx, phase, customerErr)
		}
		return customerErr
	}

	telemetry.SendInitRuntimeDoneLogEvent(execCtx.eventsAPI, execCtx.appCtx, phase, nil)

	return nil
}

func handleInit(ctx context.Context, execCtx *rapidContext) rapidmodel.AppError {
	execCtx.registrationService.SetFunctionMetadata(execCtx.initExecutionData.FunctionMetadata)
	if err := setupEventsWatcher(ctx, execCtx); err != nil {
		return rapidmodel.WrapErrorIntoPlatformFatalError(err, rapidmodel.ErrSandboxEventSetupFailure)
	}

	execCtx.telemetrySubscriptionAPI.Configure(execCtx.initExecutionData.TelemetryPassphrase(), execCtx.initExecutionData.TelemetryAPIAddr())

	return runtimeInitWithTelemetry(ctx, execCtx, interop.LifecyclePhaseInit)
}

func resolveGateError(ctx context.Context, appCtx appctx.ApplicationContext, gateErr error, errorType rapidmodel.ErrorType) rapidmodel.AppError {
	if fatalError, found := appctx.LoadFirstFatalError(appCtx); found {
		logging.Warn(ctx, "Ignoring gate error due to existing fatal error",
			"gateError", gateErr,
			"existingFatalError", fatalError)
		return fatalError
	}

	if gateErr == interop.ErrTimeout {
		logging.Warn(ctx, "Operation timed out", "err", gateErr, "errorType", errorType)
		return rapidmodel.WrapErrorIntoCustomerFatalError(gateErr, rapidmodel.ErrorSandboxTimedout)
	}
	logging.Warn(ctx, "Operation failed", "err", gateErr, "errorType", errorType)
	return rapidmodel.WrapErrorIntoCustomerFatalError(gateErr, errorType)
}
