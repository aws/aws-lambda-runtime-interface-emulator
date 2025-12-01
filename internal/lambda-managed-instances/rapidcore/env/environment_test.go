// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"testing"

	"github.com/stretchr/testify/assert"

	intmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/testutils"
)

func TestSetupEnvironment(t *testing.T) {
	defaultRuntimeEnv := intmodel.KVMap{

		AWS_ACCESS_KEY_ID:               "AKIAIOSFODNN7EXAMPLE",
		AWS_DEFAULT_REGION:              "us-west-2",
		AWS_LAMBDA_FUNCTION_MEMORY_SIZE: "3008",
		AWS_LAMBDA_FUNCTION_NAME:        "test_function",
		AWS_LAMBDA_FUNCTION_VERSION:     "$LATEST",
		AWS_REGION:                      "us-west-2",
		AWS_SECRET_ACCESS_KEY:           "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		AWS_SESSION_TOKEN:               "FwoGZXIvYXdzEMj//////////wEaDM1Qz0oN8BNwV9GqyyLVAebxhwq9ZGqojXZe1UTJkzK6F9V+VZHhT5JSWYzJUKEwOqOkQyQXJpfJsYHfkJEXtR6Kh9mXnEbqKi",
		AWS_LAMBDA_INITIALIZATION_TYPE:  "lambda-managed-instances",
		AWS_LAMBDA_RUNTIME_API:          "127.0.0.1:9001",
		HANDLER:                         "lambda_function.lambda_handler",
		LANG:                            "en_US.UTF-8",
		LD_LIBRARY_PATH:                 "/var/lang/lib:/lib64:/usr/lib64:/var/runtime:/var/runtime/lib:/var/task:/var/task/lib:/opt/lib",
		PATH:                            "/var/lang/bin:/usr/local/bin:/usr/bin/:/bin:/opt/bin",
		TZ:                              ":UTC",

		AWS_LAMBDA_LOG_FORMAT:      "json",
		AWS_LAMBDA_LOG_GROUP_NAME:  "/aws/lambda/test_function",
		AWS_LAMBDA_LOG_LEVEL:       "info",
		AWS_LAMBDA_LOG_STREAM_NAME: "$LATEST",
		AWS_LAMBDA_MAX_CONCURRENCY: "1",
		_AWS_XRAY_DAEMON_ADDRESS:   "2.2.2.2:2345",
		_AWS_XRAY_DAEMON_PORT:      "2000",
		AWS_XRAY_CONTEXT_MISSING:   "LOG_ERROR",
		AWS_XRAY_DAEMON_ADDRESS:    "2.2.2.2:2345",
		LAMBDA_RUNTIME_DIR:         "/var/runtime",
		LAMBDA_TASK_ROOT:           "/var/task",

		"CUSTOMER_ENV_VAR_1": "customer_env_value_1",
	}
	defaultExtensionEnv := intmodel.KVMap{

		AWS_ACCESS_KEY_ID:               "AKIAIOSFODNN7EXAMPLE",
		AWS_DEFAULT_REGION:              "us-west-2",
		AWS_LAMBDA_FUNCTION_MEMORY_SIZE: "3008",
		AWS_LAMBDA_FUNCTION_NAME:        "test_function",
		AWS_LAMBDA_FUNCTION_VERSION:     "$LATEST",
		AWS_LAMBDA_LOG_FORMAT:           "json",
		AWS_LAMBDA_LOG_LEVEL:            "info",
		AWS_REGION:                      "us-west-2",
		AWS_SECRET_ACCESS_KEY:           "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		AWS_SESSION_TOKEN:               "FwoGZXIvYXdzEMj//////////wEaDM1Qz0oN8BNwV9GqyyLVAebxhwq9ZGqojXZe1UTJkzK6F9V+VZHhT5JSWYzJUKEwOqOkQyQXJpfJsYHfkJEXtR6Kh9mXnEbqKi",
		AWS_LAMBDA_INITIALIZATION_TYPE:  "lambda-managed-instances",
		AWS_LAMBDA_RUNTIME_API:          "127.0.0.1:9001",
		LANG:                            "en_US.UTF-8",
		LD_LIBRARY_PATH:                 "/var/lang/lib:/lib64:/usr/lib64:/var/runtime:/var/runtime/lib:/var/task:/var/task/lib:/opt/lib",
		PATH:                            "/var/lang/bin:/usr/local/bin:/usr/bin/:/bin:/opt/bin",
		TZ:                              ":UTC",

		"CUSTOMER_ENV_VAR_1": "customer_env_value_1",
	}

	tests := []struct {
		name                 string
		initMsg              intmodel.InitRequestMessage
		runtimeLoggingSocket string
		wantRuntimeEnv       func(runtimeEnv intmodel.KVMap) intmodel.KVMap
		wantExtensionEnv     func(extensionEnv intmodel.KVMap) intmodel.KVMap
	}{
		{
			name:    "zip",
			initMsg: testutils.MakeInitPayload(),
		},
		{
			name:                 "runtimeLoggingSocket",
			initMsg:              testutils.MakeInitPayload(),
			runtimeLoggingSocket: "/path/to/runtimeLoggingSocket",
			wantRuntimeEnv: func(env intmodel.KVMap) intmodel.KVMap {
				env[_LAMBDA_TELEMETRY_LOG_FD_PROVIDER_SOCKET] = "/path/to/runtimeLoggingSocket"
				return env
			},
		},
		{
			name:    "oci",
			initMsg: testutils.MakeInitPayload(testutils.WithArtefactType(intmodel.ArtefactTypeOCI)),
			wantRuntimeEnv: func(env intmodel.KVMap) intmodel.KVMap {
				env[AWS_EXECUTION_ENV] = "AWS_Lambda_Image"
				delete(env, HANDLER)
				delete(env, LANG)
				delete(env, LD_LIBRARY_PATH)
				delete(env, PATH)
				delete(env, TZ)
				return env
			},
			wantExtensionEnv: func(env intmodel.KVMap) intmodel.KVMap {
				delete(env, HANDLER)
				delete(env, LANG)
				delete(env, LD_LIBRARY_PATH)
				delete(env, PATH)
				delete(env, TZ)
				return env
			},
		},
		{
			name:    "empty_AWS_LAMBDA_LOG_FORMAT",
			initMsg: testutils.MakeInitPayload(testutils.WithLogFormat("")),
			wantRuntimeEnv: func(env intmodel.KVMap) intmodel.KVMap {
				delete(env, AWS_LAMBDA_LOG_FORMAT)
				return env
			},
			wantExtensionEnv: func(env intmodel.KVMap) intmodel.KVMap {
				delete(env, AWS_LAMBDA_LOG_FORMAT)
				return env
			},
		},
		{
			name:    "empty_AWS_LAMBDA_LOG_LEVEL",
			initMsg: testutils.MakeInitPayload(testutils.WithLogLevel("")),
			wantRuntimeEnv: func(env intmodel.KVMap) intmodel.KVMap {
				delete(env, AWS_LAMBDA_LOG_LEVEL)
				return env
			},
			wantExtensionEnv: func(env intmodel.KVMap) intmodel.KVMap {
				delete(env, AWS_LAMBDA_LOG_LEVEL)
				return env
			},
		},
		{
			name: "customer_sets_underscore_env_var",
			initMsg: testutils.MakeInitPayload(testutils.WithEnvVars(intmodel.KVMap{
				"CUSTOMER_ENV_VAR_1": "customer_env_value_1",
				"_ENV_1":             "val_1",
				"_ENV_2":             "val_2",
			})),
		},
		{
			name: "customer_overwrites_all_defined_env_vars_zip",
			initMsg: testutils.MakeInitPayload(testutils.WithEnvVars(func() intmodel.KVMap {
				customerEnvVars := make(intmodel.KVMap, len(Defined))
				for k := range Defined {
					customerEnvVars[k] = "customer_" + k
				}
				customerEnvVars["CUSTOMER_ENV_VAR_1"] = "customer_env_value_1"
				return customerEnvVars
			}())),
			wantRuntimeEnv: func(env intmodel.KVMap) intmodel.KVMap {
				env[AWS_LAMBDA_LOG_FORMAT] = "customer_AWS_LAMBDA_LOG_FORMAT"
				env[AWS_LAMBDA_LOG_LEVEL] = "customer_AWS_LAMBDA_LOG_LEVEL"
				env[AWS_XRAY_CONTEXT_MISSING] = "customer_AWS_XRAY_CONTEXT_MISSING"
				env[AWS_XRAY_DAEMON_ADDRESS] = "customer_AWS_XRAY_DAEMON_ADDRESS"
				env[LANG] = "customer_LANG"
				env[LD_LIBRARY_PATH] = "customer_LD_LIBRARY_PATH"
				env[PATH] = "customer_PATH"
				env[TZ] = "customer_TZ"
				return env
			},
			wantExtensionEnv: func(env intmodel.KVMap) intmodel.KVMap {
				env[AWS_LAMBDA_LOG_FORMAT] = "customer_AWS_LAMBDA_LOG_FORMAT"
				env[AWS_LAMBDA_LOG_LEVEL] = "customer_AWS_LAMBDA_LOG_LEVEL"
				env[AWS_XRAY_CONTEXT_MISSING] = "customer_AWS_XRAY_CONTEXT_MISSING"
				env[AWS_XRAY_DAEMON_ADDRESS] = "customer_AWS_XRAY_DAEMON_ADDRESS"
				env[LANG] = "customer_LANG"
				env[LD_LIBRARY_PATH] = "customer_LD_LIBRARY_PATH"
				env[PATH] = "customer_PATH"
				env[TZ] = "customer_TZ"
				return env
			},
		},
		{
			name: "customer_overwrites_all_defined_env_vars_oci",
			initMsg: testutils.MakeInitPayload(
				testutils.WithArtefactType(intmodel.ArtefactTypeOCI),
				testutils.WithEnvVars(func() intmodel.KVMap {
					customerEnvVars := make(intmodel.KVMap, len(Defined))
					for k := range Defined {
						customerEnvVars[k] = "customer_" + k
					}
					customerEnvVars["CUSTOMER_ENV_VAR_1"] = "customer_env_value_1"
					return customerEnvVars
				}()),
			),
			wantRuntimeEnv: func(env intmodel.KVMap) intmodel.KVMap {
				delete(env, HANDLER)
				env[AWS_EXECUTION_ENV] = "AWS_Lambda_Image"

				env[AWS_LAMBDA_LOG_FORMAT] = "customer_AWS_LAMBDA_LOG_FORMAT"
				env[AWS_LAMBDA_LOG_LEVEL] = "customer_AWS_LAMBDA_LOG_LEVEL"
				env[AWS_XRAY_CONTEXT_MISSING] = "customer_AWS_XRAY_CONTEXT_MISSING"
				env[AWS_XRAY_DAEMON_ADDRESS] = "customer_AWS_XRAY_DAEMON_ADDRESS"
				env[LANG] = "customer_LANG"
				env[LD_LIBRARY_PATH] = "customer_LD_LIBRARY_PATH"
				env[PATH] = "customer_PATH"
				env[TZ] = "customer_TZ"
				return env
			},
			wantExtensionEnv: func(env intmodel.KVMap) intmodel.KVMap {
				env[AWS_LAMBDA_LOG_FORMAT] = "customer_AWS_LAMBDA_LOG_FORMAT"
				env[AWS_LAMBDA_LOG_LEVEL] = "customer_AWS_LAMBDA_LOG_LEVEL"
				env[AWS_XRAY_CONTEXT_MISSING] = "customer_AWS_XRAY_CONTEXT_MISSING"
				env[AWS_XRAY_DAEMON_ADDRESS] = "customer_AWS_XRAY_DAEMON_ADDRESS"
				env[LANG] = "customer_LANG"
				env[LD_LIBRARY_PATH] = "customer_LD_LIBRARY_PATH"
				env[PATH] = "customer_PATH"
				env[TZ] = "customer_TZ"
				return env
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantRuntimeEnv == nil {
				tt.wantRuntimeEnv = func(runtimeEnv intmodel.KVMap) intmodel.KVMap { return runtimeEnv }
			}
			if tt.wantExtensionEnv == nil {
				tt.wantExtensionEnv = func(extensionEnv intmodel.KVMap) intmodel.KVMap { return extensionEnv }
			}

			gotRuntimeEnv, gotExtensionEnv := SetupEnvironment(&tt.initMsg, "127.0.0.1:9001", tt.runtimeLoggingSocket)
			assert.Equal(t, tt.wantRuntimeEnv(clone(defaultRuntimeEnv)), gotRuntimeEnv)
			assert.Equal(t, tt.wantExtensionEnv(clone(defaultExtensionEnv)), gotExtensionEnv)
		})
	}
}

func clone(m intmodel.KVMap) intmodel.KVMap {
	cloned := make(intmodel.KVMap, len(m))
	for k, v := range m {
		cloned[k] = v
	}
	return cloned
}
