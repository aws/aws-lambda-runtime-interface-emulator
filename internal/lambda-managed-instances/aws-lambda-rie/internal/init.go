// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	intmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapidcore/env"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils"
)

type InitRequestMessageFactory func(fileUtil utils.FileUtil, args []string) (intmodel.InitRequestMessage, model.AppError)

func GetInitRequestMessage(fileUtil utils.FileUtil, args []string) (intmodel.InitRequestMessage, model.AppError) {
	accountID := getEnvOrDefault("AWS_ACCOUNT_ID", "123456789012")
	invokeTimeout := intmodel.DurationMS(getEnvOrDefaultInt("AWS_LAMBDA_FUNCTION_TIMEOUT", 300) * int(time.Second))
	functionName := getEnvOrDefault(env.AWS_LAMBDA_FUNCTION_NAME, "test_function")
	region := getEnvOrDefault(env.AWS_REGION, "us-east-1")
	cwd := getCwd()

	cmd, err := getBootstrap(fileUtil, args, cwd)
	if err != nil {
		return intmodel.InitRequestMessage{}, err
	}

	return intmodel.InitRequestMessage{
		AccountID:            accountID,
		AwsKey:               os.Getenv(env.AWS_ACCESS_KEY_ID),
		AwsSecret:            os.Getenv(env.AWS_SECRET_ACCESS_KEY),
		AwsSession:           os.Getenv(env.AWS_SESSION_TOKEN),
		AwsRegion:            region,
		EnvVars:              env.KVPairStringsToMap(os.Environ()),
		MemorySizeBytes:      getEnvOrDefaultInt(env.AWS_LAMBDA_FUNCTION_MEMORY_SIZE, 3008) * 1024 * 1024,
		FunctionARN:          fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s", region, accountID, functionName),
		FunctionVersion:      getEnvOrDefault(env.AWS_LAMBDA_FUNCTION_VERSION, "$LATEST"),
		FunctionVersionID:    "",
		ArtefactType:         intmodel.ArtefactTypeZIP,
		TaskName:             functionName,
		Handler:              getHandler(args),
		InvokeTimeout:        invokeTimeout,
		InitTimeout:          invokeTimeout,
		RuntimeVersion:       "",
		RuntimeArn:           "",
		RuntimeWorkerCount:   getEnvOrDefaultInt(env.AWS_LAMBDA_MAX_CONCURRENCY, 1),
		LogFormat:            getEnvOrDefault(env.AWS_LAMBDA_LOG_FORMAT, "json"),
		LogLevel:             os.Getenv(env.AWS_LAMBDA_LOG_LEVEL),
		LogGroupName:         getEnvOrDefault(env.AWS_LAMBDA_LOG_GROUP_NAME, "/aws/lambda/Functions"),
		LogStreamName:        getEnvOrDefault(env.AWS_LAMBDA_LOG_STREAM_NAME, "$LATEST"),
		TelemetryAPIAddress:  intmodel.TelemetryAddr(netip.MustParseAddrPort("127.0.0.1:0")),
		TelemetryPassphrase:  "",
		XRayDaemonAddress:    "",
		XrayTracingMode:      intmodel.XRayTracingModePassThrough,
		CurrentWorkingDir:    cwd,
		RuntimeBinaryCommand: cmd,
		AvailabilityZoneId:   "",
		AmiId:                "",
	}, nil
}

func getHandler(args []string) string {
	handler := getEnvOrDefault("AWS_LAMBDA_FUNCTION_HANDLER", os.Getenv(env.HANDLER))
	if handler != "" {
		return handler
	}

	if len(args) > 2 {
		return args[len(args)-1]
	}
	return ""
}

func getEnvOrDefault(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func getEnvOrDefaultInt(key string, defaultVal int) int {
	val, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal
	}

	valInt, err := strconv.Atoi(val)
	if err != nil {
		slog.Warn("Failed to convert environment variable to integer",
			"key", key,
			"value", val,
			"err", err)
		return defaultVal
	}
	return valInt
}

func getBootstrap(fileUtil utils.FileUtil, args []string, cwd string) (cmd []string, err model.AppError) {

	if len(args) > 1 {
		slog.Info("executing bootstrap", "command", args[1])
		return args[1:], nil
	}

	candidates := []string{
		cwd + "/bootstrap",
		"/var/runtime/bootstrap",
		"/var/task/bootstrap",
		"/opt/bootstrap",
	}
	for _, c := range candidates {
		stat, err := fileUtil.Stat(c)
		if fileUtil.IsNotExist(err) {
			slog.Warn("could not find bootstrap file", "path", c)
			continue
		}
		if stat.IsDir() {
			slog.Warn("bootstrap file is a directory", "path", c)
			continue
		}

		return []string{c}, nil
	}

	err = model.WrapErrorIntoCustomerInvalidError(
		fmt.Errorf("could not find runtime entrypoint in CLI args and in predefined locations: %s", strings.Join(candidates, ", ")),
		model.ErrorRuntimeInvalidEntryPoint,
	)
	return nil, err
}

func getCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		slog.Warn("could not find current working directory. Using default /var/task instead", "err", err)
		return "/var/task"
	}

	return cwd
}
