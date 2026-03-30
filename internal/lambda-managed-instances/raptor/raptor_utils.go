// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package raptor

import (
	"encoding/json"
	"net/netip"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lmds"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	internalModel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapidcore/env"
	supvmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/supervisor/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils/invariant"
)

func getInitExecutionData(initRequest *internalModel.InitRequestMessage, runtimeAPIAddrPort, telemetryFDSocketPath, metadataToken string) interop.InitExecutionData {

	metadataAPIAddrPort := runtimeAPIAddrPort

	runtimeEnv, extensionEnv := env.SetupEnvironment(initRequest, runtimeAPIAddrPort, telemetryFDSocketPath, metadataAPIAddrPort, metadataToken)

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
		Metadata: getMetadataConfig(initRequest.AvailabilityZoneId),
	}

	return initMessage
}

func getMetadataConfig(availabilityZoneId string) lmds.MetadataConfig {
	metadataBytes, err := json.Marshal(lmds.Metadata{
		AvailabilityZoneID: availabilityZoneId,
	})
	invariant.Checkf(err == nil, "could not marshal metadata json: %s", err)
	return lmds.MetadataConfig{
		Data:   metadataBytes,
		MaxAge: 12 * time.Hour,
	}
}
