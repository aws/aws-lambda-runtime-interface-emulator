// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
)

const (
	AWS_ACCESS_KEY_ID                        = "AWS_ACCESS_KEY_ID"
	AWS_DEFAULT_REGION                       = "AWS_DEFAULT_REGION"
	AWS_LAMBDA_FUNCTION_MEMORY_SIZE          = "AWS_LAMBDA_FUNCTION_MEMORY_SIZE"
	AWS_LAMBDA_FUNCTION_NAME                 = "AWS_LAMBDA_FUNCTION_NAME"
	AWS_LAMBDA_FUNCTION_VERSION              = "AWS_LAMBDA_FUNCTION_VERSION"
	AWS_LAMBDA_LOG_FORMAT                    = "AWS_LAMBDA_LOG_FORMAT"
	AWS_LAMBDA_LOG_GROUP_NAME                = "AWS_LAMBDA_LOG_GROUP_NAME"
	AWS_LAMBDA_LOG_LEVEL                     = "AWS_LAMBDA_LOG_LEVEL"
	AWS_LAMBDA_LOG_STREAM_NAME               = "AWS_LAMBDA_LOG_STREAM_NAME"
	AWS_LAMBDA_MAX_CONCURRENCY               = "AWS_LAMBDA_MAX_CONCURRENCY"
	AWS_REGION                               = "AWS_REGION"
	AWS_SECRET_ACCESS_KEY                    = "AWS_SECRET_ACCESS_KEY"
	AWS_SESSION_TOKEN                        = "AWS_SESSION_TOKEN"
	_AWS_XRAY_DAEMON_ADDRESS                 = "_AWS_XRAY_DAEMON_ADDRESS"
	_AWS_XRAY_DAEMON_PORT                    = "_AWS_XRAY_DAEMON_PORT"
	_LAMBDA_TELEMETRY_LOG_FD_PROVIDER_SOCKET = "_LAMBDA_TELEMETRY_LOG_FD_PROVIDER_SOCKET"
	AWS_EXECUTION_ENV                        = "AWS_EXECUTION_ENV"
	AWS_LAMBDA_INITIALIZATION_TYPE           = "AWS_LAMBDA_INITIALIZATION_TYPE"
	AWS_LAMBDA_RUNTIME_API                   = "AWS_LAMBDA_RUNTIME_API"
	AWS_XRAY_CONTEXT_MISSING                 = "AWS_XRAY_CONTEXT_MISSING"
	AWS_XRAY_DAEMON_ADDRESS                  = "AWS_XRAY_DAEMON_ADDRESS"
	AWS_XRAY_DAEMON_PORT                     = "AWS_XRAY_DAEMON_PORT"
	AWS_XRAY_TRACE_ID                        = "AWS_XRAY_TRACE_ID"
	HANDLER                                  = "_HANDLER"
	LAMBDA_RUNTIME_DIR                       = "LAMBDA_RUNTIME_DIR"
	LAMBDA_TASK_ROOT                         = "LAMBDA_TASK_ROOT"
	LANG                                     = "LANG"
	LD_LIBRARY_PATH                          = "LD_LIBRARY_PATH"
	PATH                                     = "PATH"
	TZ                                       = "TZ"
)

var Defined = map[string]struct{}{
	AWS_ACCESS_KEY_ID:                        {},
	AWS_DEFAULT_REGION:                       {},
	AWS_LAMBDA_FUNCTION_MEMORY_SIZE:          {},
	AWS_LAMBDA_FUNCTION_NAME:                 {},
	AWS_LAMBDA_FUNCTION_VERSION:              {},
	AWS_LAMBDA_LOG_FORMAT:                    {},
	AWS_LAMBDA_LOG_GROUP_NAME:                {},
	AWS_LAMBDA_LOG_LEVEL:                     {},
	AWS_LAMBDA_LOG_STREAM_NAME:               {},
	AWS_LAMBDA_MAX_CONCURRENCY:               {},
	AWS_REGION:                               {},
	AWS_SECRET_ACCESS_KEY:                    {},
	AWS_SESSION_TOKEN:                        {},
	_AWS_XRAY_DAEMON_ADDRESS:                 {},
	_AWS_XRAY_DAEMON_PORT:                    {},
	_LAMBDA_TELEMETRY_LOG_FD_PROVIDER_SOCKET: {},
	AWS_EXECUTION_ENV:                        {},
	AWS_LAMBDA_INITIALIZATION_TYPE:           {},
	AWS_LAMBDA_RUNTIME_API:                   {},
	AWS_XRAY_CONTEXT_MISSING:                 {},
	AWS_XRAY_DAEMON_ADDRESS:                  {},
	AWS_XRAY_DAEMON_PORT:                     {},
	AWS_XRAY_TRACE_ID:                        {},
	HANDLER:                                  {},
	LAMBDA_RUNTIME_DIR:                       {},
	LAMBDA_TASK_ROOT:                         {},
	LANG:                                     {},
	LD_LIBRARY_PATH:                          {},
	PATH:                                     {},
	TZ:                                       {},
}

var overridable = map[string]struct{}{
	AWS_LAMBDA_LOG_FORMAT:    {},
	AWS_LAMBDA_LOG_LEVEL:     {},
	AWS_XRAY_CONTEXT_MISSING: {},
	AWS_XRAY_DAEMON_ADDRESS:  {},
	LANG:                     {},
	LD_LIBRARY_PATH:          {},
	PATH:                     {},
	TZ:                       {},
}

func SetupEnvironment(config *model.InitRequestMessage, runtimePort, runtimeLoggingSocket string) (runtimeEnv, extensionEnv model.KVMap) {

	commonVars := model.KVMap{
		AWS_ACCESS_KEY_ID:               config.AwsKey,
		AWS_DEFAULT_REGION:              config.AwsRegion,
		AWS_LAMBDA_FUNCTION_MEMORY_SIZE: strconv.Itoa(config.MemorySizeBytes / 1024 / 1024),
		AWS_LAMBDA_FUNCTION_NAME:        config.TaskName,
		AWS_LAMBDA_FUNCTION_VERSION:     config.FunctionVersion,
		AWS_REGION:                      config.AwsRegion,
		AWS_SECRET_ACCESS_KEY:           config.AwsSecret,
		AWS_SESSION_TOKEN:               config.AwsSession,
		AWS_LAMBDA_INITIALIZATION_TYPE:  interop.InitializationType,
		AWS_LAMBDA_RUNTIME_API:          runtimePort,
	}
	if config.ArtefactType == model.ArtefactTypeZIP {
		commonVars[LANG] = "en_US.UTF-8"
		commonVars[LD_LIBRARY_PATH] = "/var/lang/lib:/lib64:/usr/lib64:/var/runtime:/var/runtime/lib:/var/task:/var/task/lib:/opt/lib"
		commonVars[PATH] = "/var/lang/bin:/usr/local/bin:/usr/bin/:/bin:/opt/bin"
		commonVars[TZ] = ":UTC"
	}
	if config.LogFormat != "" {
		commonVars[AWS_LAMBDA_LOG_FORMAT] = config.LogFormat
	}
	if config.LogLevel != "" {
		commonVars[AWS_LAMBDA_LOG_LEVEL] = config.LogLevel
	}

	for k, v := range cloneAndFilterCustomerEnvVars(config.EnvVars) {
		commonVars[k] = v
	}

	return getRuntimeOnlyEnvVars(commonVars, config, runtimeLoggingSocket), commonVars
}

func getRuntimeOnlyEnvVars(common model.KVMap, config *model.InitRequestMessage, runtimeLoggingSocket string) model.KVMap {

	runtimeOnlyVars := model.KVMap{
		AWS_LAMBDA_LOG_GROUP_NAME:  config.LogGroupName,
		AWS_LAMBDA_LOG_STREAM_NAME: config.LogStreamName,
		AWS_LAMBDA_MAX_CONCURRENCY: strconv.Itoa(config.RuntimeWorkerCount),
		_AWS_XRAY_DAEMON_ADDRESS:   config.XRayDaemonAddress,
		_AWS_XRAY_DAEMON_PORT:      "2000",
		AWS_XRAY_CONTEXT_MISSING:   "LOG_ERROR",
		AWS_XRAY_DAEMON_ADDRESS:    config.XRayDaemonAddress,
		LAMBDA_RUNTIME_DIR:         "/var/runtime",
		LAMBDA_TASK_ROOT:           "/var/task",
	}

	if config.ArtefactType == model.ArtefactTypeOCI {
		runtimeOnlyVars[AWS_EXECUTION_ENV] = "AWS_Lambda_Image"
	} else {
		runtimeOnlyVars[HANDLER] = config.Handler
	}

	if runtimeLoggingSocket != "" {
		runtimeOnlyVars[_LAMBDA_TELEMETRY_LOG_FD_PROVIDER_SOCKET] = runtimeLoggingSocket
	}

	merge(runtimeOnlyVars, common)

	return runtimeOnlyVars
}

func cloneAndFilterCustomerEnvVars(envVars model.KVMap) model.KVMap {
	filtered := make(model.KVMap, len(envVars))
	for k, v := range envVars {

		if strings.HasPrefix(string(k), "_") {
			continue
		}

		_, defined := Defined[k]
		_, overridable := overridable[k]
		if defined && !overridable {

			continue
		}

		filtered[k] = v
	}

	return filtered
}

func merge(to, from model.KVMap) {
	for k, v := range from {
		to[k] = v
	}
}
