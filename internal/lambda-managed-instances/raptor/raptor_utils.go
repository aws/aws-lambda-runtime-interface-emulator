// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package raptor

import (
	"net/netip"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	internalModel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapidcore/env"
	supvmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/supervisor/model"
)

func getInitExecutionData(initRequest *internalModel.InitRequestMessage, runtimePort, telemetryFDSocketPath string) interop.InitExecutionData {

	runtimeEnv, extensionEnv := env.SetupEnvironment(initRequest, runtimePort, telemetryFDSocketPath)

	initMessage := interop.InitExecutionData{

		ExtensionEnv: extensionEnv,
		Runtime: model.Runtime{
			ExecConfig: model.RuntimeExec{
				Cmd:        initRequest.RuntimeBinaryCommand,
				WorkingDir: initRequest.CurrentWorkingDir,
				Env:        runtimeEnv,
			},
		},

		Credentials: model.Credentials{
			AwsKey:     initRequest.AwsKey,
			AwsSecret:  initRequest.AwsSecret,
			AwsSession: initRequest.AwsSession,
		},
		LogGroupName:  initRequest.LogGroupName,
		LogStreamName: initRequest.LogStreamName,
		FunctionMetadata: model.FunctionMetadata{
			AccountID:       initRequest.AccountID,
			FunctionName:    initRequest.TaskName,
			FunctionVersion: initRequest.FunctionVersion,
			MemorySizeBytes: uint64(initRequest.MemorySizeBytes),
			Handler:         initRequest.Handler,
			RuntimeInfo: model.RuntimeInfo{
				Arn:     initRequest.RuntimeArn,
				Version: initRequest.RuntimeVersion,
			},
		},
		RuntimeManagedLoggingFormats: []supvmodel.ManagedLoggingFormat{
			supvmodel.LineBasedManagedLogging,
		},

		StaticData: interop.EEStaticData{
			InitTimeout:        time.Duration(initRequest.InitTimeout),
			FunctionTimeout:    time.Duration(initRequest.InvokeTimeout),
			FunctionARN:        initRequest.FunctionARN,
			FunctionVersionID:  initRequest.FunctionVersionID,
			LogGroupName:       initRequest.LogGroupName,
			LogStreamName:      initRequest.LogStreamName,
			XRayTracingMode:    initRequest.XrayTracingMode,
			ArtefactType:       initRequest.ArtefactType,
			AmiId:              initRequest.AmiId,
			RuntimeVersion:     initRequest.RuntimeVersion,
			AvailabilityZoneId: initRequest.AvailabilityZoneId,
		},
		TelemetrySubscriptionConfig: interop.TelemetrySubscriptionConfig{
			APIAddr:    netip.AddrPort(initRequest.TelemetryAPIAddress),
			Passphrase: initRequest.TelemetryPassphrase,
		},
	}

	return initMessage
}
