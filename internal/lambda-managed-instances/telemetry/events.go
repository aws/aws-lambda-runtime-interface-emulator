// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"log/slog"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func SendInitStartLogEvent(
	eventsAPI interop.EventsAPI,
	functionMetadata model.FunctionMetadata,
	logStreamName string,
	phase interop.LifecyclePhase,
) {
	initPhase, err := initPhaseFromLifecyclePhase(phase)
	if err != nil {
		slog.Error("failed to convert lifecycle phase into init phase", "err", err)
		return
	}

	initStartData := interop.InitStartData{
		InitializationType: interop.InitializationType,
		RuntimeVersion:     functionMetadata.RuntimeInfo.Version,
		RuntimeVersionArn:  functionMetadata.RuntimeInfo.Arn,
		FunctionName:       functionMetadata.FunctionName,
		FunctionVersion:    functionMetadata.FunctionVersion,

		InstanceID:        logStreamName,
		InstanceMaxMemory: functionMetadata.MemorySizeBytes,
		Phase:             initPhase,
	}
	slog.Debug("Init start data", "data", initStartData.String())

	if err := eventsAPI.SendInitStart(initStartData); err != nil {
		slog.Error("Failed to send Init START", "err", err)
	}
}

func prepareInitRuntimeDoneData(appCtx appctx.ApplicationContext, initError model.AppError, phase interop.LifecyclePhase) interop.InitRuntimeDoneData {
	initPhase, _ := initPhaseFromLifecyclePhase(phase)

	status := interop.BuildStatusFromError(initError)

	initRuntimeDoneData := interop.InitRuntimeDoneData{
		InitializationType: interop.InitializationType,
		Status:             status,
		Phase:              initPhase,
		ErrorType:          getFirstFatalError(appCtx, status),
	}
	return initRuntimeDoneData
}

func SendInitRuntimeDoneLogEvent(
	eventsAPI interop.EventsAPI,
	appCtx appctx.ApplicationContext,
	phase interop.LifecyclePhase,
	initError model.AppError,
) {
	initRuntimeDoneData := prepareInitRuntimeDoneData(appCtx, initError, phase)

	slog.Debug("Init runtime done data", "data", initRuntimeDoneData.String())

	if err := eventsAPI.SendInitRuntimeDone(initRuntimeDoneData); err != nil {
		slog.Error("Failed to send Init RTDONE event", "err", err)
	}
}

func SendInitReportLogEvent(
	eventsAPI interop.EventsAPI,
	appCtx appctx.ApplicationContext,
	initDuration time.Duration,
	phase interop.LifecyclePhase,
	initError model.AppError,
) {
	initPhase, err := initPhaseFromLifecyclePhase(phase)
	if err != nil {
		slog.Error("failed to convert lifecycle phase into init phase", "err", err)
		return
	}

	status := interop.BuildStatusFromError(initError)

	initReportData := interop.InitReportData{
		InitializationType: interop.InitializationType,
		Metrics: interop.InitReportMetrics{
			DurationMs: calculateDurationInMillis(initDuration),
		},
		Phase:     initPhase,
		Status:    status,
		ErrorType: getFirstFatalError(appCtx, status),
	}
	slog.Debug("Init report data", "data", initReportData.String())

	if err = eventsAPI.SendInitReport(initReportData); err != nil {
		slog.Error("Failed to send INIT REPORT", "err", err)
	}
}

func SendAgentsInitStatus(eventsAPI interop.EventsAPI, agents []core.AgentInfo) {
	for _, agent := range agents {
		extensionInitData := interop.ExtensionInitData{
			AgentName:     agent.Name,
			State:         agent.State,
			ErrorType:     string(agent.ErrorType),
			Subscriptions: agent.Subscriptions,
		}
		if err := eventsAPI.SendExtensionInit(extensionInitData); err != nil {
			slog.Error("Failed to send extension init", "err", err)
		}
	}
}

func SendImageError(eventsAPI interop.EventsAPI, execError model.RuntimeExecError, execConfig model.RuntimeExec) {
	eventsAPI.SendImageError(interop.ImageErrorLogData{
		ExecError:  execError,
		ExecConfig: execConfig,
	})
}

func getFirstFatalError(appCtx appctx.ApplicationContext, status string) *string {
	if status == interop.Success {
		return nil
	}

	customerError, found := appctx.LoadFirstFatalError(appCtx)
	var errorType model.ErrorType
	if !found {

		errorType = model.ErrorRuntimeUnknown
	} else {
		errorType = customerError.ErrorType()
	}
	stringifiedError := string(errorType)
	return &stringifiedError
}
