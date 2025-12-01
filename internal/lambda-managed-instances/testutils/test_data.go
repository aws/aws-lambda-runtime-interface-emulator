// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
)

var (
	DefaultTestFunctionARN     = "arn:aws:lambda:us-east-1:123456789012:function:test_function"
	DefaultTestFunctionVersion = "$LATEST"
)

func MakeValidInitPayload(opts ...InitPayloadOption) string {
	return JsonEncode(MakeInitPayload(opts...))
}

func MakeInvalidInitPayload() string {
	return JsonEncode(MakeInitPayload(WithInvalidPayload()))
}

type InitPayloadOption func(*model.InitRequestMessage)

func MakeInitPayload(opts ...InitPayloadOption) model.InitRequestMessage {

	payload := model.InitRequestMessage{
		AccountID:  "123456789012",
		AwsKey:     "AKIAIOSFODNN7EXAMPLE",
		AwsSecret:  "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		AwsSession: "FwoGZXIvYXdzEMj//////////wEaDM1Qz0oN8BNwV9GqyyLVAebxhwq9ZGqojXZe1UTJkzK6F9V+VZHhT5JSWYzJUKEwOqOkQyQXJpfJsYHfkJEXtR6Kh9mXnEbqKi",
		AwsRegion:  "us-west-2",
		EnvVars: map[string]string{
			"CUSTOMER_ENV_VAR_1": "customer_env_value_1",
		},
		ArtefactType:         model.ArtefactTypeZIP,
		MemorySizeBytes:      3008 * 1024 * 1024,
		FunctionARN:          DefaultTestFunctionARN,
		FunctionVersion:      DefaultTestFunctionVersion,
		FunctionVersionID:    "test-function-version-id",
		TaskName:             "test_function",
		InvokeTimeout:        model.DurationMS(3 * time.Second),
		InitTimeout:          model.DurationMS(10 * time.Second),
		RuntimeWorkerCount:   1,
		LogFormat:            "json",
		LogLevel:             "info",
		LogGroupName:         "/aws/lambda/test_function",
		LogStreamName:        "$LATEST",
		TelemetryAPIAddress:  model.TelemetryAddr(netip.MustParseAddrPort("1.1.1.1:1234")),
		TelemetryPassphrase:  "hello",
		XRayDaemonAddress:    "2.2.2.2:2345",
		XrayTracingMode:      model.XRayTracingModeActive,
		RuntimeBinaryCommand: []string{"cmd", "arg1", "arg2"},
		CurrentWorkingDir:    "/",
		AmiId:                "ami-12345",
		AvailabilityZoneId:   "az-1",
		Handler:              "lambda_function.lambda_handler",
	}

	for _, opt := range opts {
		opt(&payload)
	}

	return payload
}

func JsonEncode(payload model.InitRequestMessage) string {
	jsonBytes, err := json.MarshalIndent(payload, "", "    ")
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal init payload: %v", err))
	}

	return string(jsonBytes)
}

func WithInvalidPayload() InitPayloadOption {
	return func(p *model.InitRequestMessage) {
		p.AwsKey = "AKIAIOSFODNN7EXAMPLE"

		p.RuntimeBinaryCommand = nil
	}
}

func WithTimeouts(invokeTimeout, initTimeout time.Duration) InitPayloadOption {
	return func(p *model.InitRequestMessage) {
		p.InvokeTimeout = model.DurationMS(invokeTimeout)
		p.InitTimeout = model.DurationMS(initTimeout)
	}
}

func WithLogFormat(format string) InitPayloadOption {
	return func(p *model.InitRequestMessage) {
		p.LogFormat = format
	}
}

func WithLogLevel(level string) InitPayloadOption {
	return func(p *model.InitRequestMessage) {
		p.LogLevel = level
	}
}

func WithArtefactType(typ model.ArtefactType) InitPayloadOption {
	return func(p *model.InitRequestMessage) {
		p.ArtefactType = typ
	}
}

func WithTelemetry(apiAddress netip.AddrPort, passphrase string) InitPayloadOption {
	return func(p *model.InitRequestMessage) {
		p.TelemetryAPIAddress = model.TelemetryAddr(apiAddress)
		p.TelemetryPassphrase = passphrase
	}
}

func WithExecValues(artefact model.ArtefactType, cmd []string, cwd string) InitPayloadOption {
	return func(p *model.InitRequestMessage) {
		p.ArtefactType = artefact
		p.RuntimeBinaryCommand = cmd
		p.CurrentWorkingDir = cwd
	}
}

func WithEnvVars(envVars model.KVMap) InitPayloadOption {
	return func(p *model.InitRequestMessage) {
		p.EnvVars = envVars
	}
}
