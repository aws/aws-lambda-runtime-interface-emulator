// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package env

func predefinedInternalEnvVarKeys() map[string]bool {
	return map[string]bool{
		"_LAMBDA_SB_ID":                true,
		"_LAMBDA_LOG_FD":               true,
		"_LAMBDA_SHARED_MEM_FD":        true,
		"_LAMBDA_CONTROL_SOCKET":       true,
		"_LAMBDA_DIRECT_INVOKE_SOCKET": true,
		"_LAMBDA_RUNTIME_LOAD_TIME":    true,
		"_LAMBDA_CONSOLE_SOCKET":       true,
		// _X_AMZN_TRACE_ID is set by stock runtimes. Provided
		// runtimes should set and mutate it on each invoke.
		"_X_AMZN_TRACE_ID":                 true,
		"_LAMBDA_TELEMETRY_API_PASSPHRASE": true,
	}
}

func predefinedPlatformEnvVarKeys() map[string]bool {
	return map[string]bool{
		"AWS_REGION":                      true,
		"AWS_DEFAULT_REGION":              true,
		"AWS_LAMBDA_FUNCTION_NAME":        true,
		"AWS_LAMBDA_FUNCTION_MEMORY_SIZE": true,
		"AWS_LAMBDA_FUNCTION_VERSION":     true,
		"AWS_LAMBDA_RUNTIME_API":          true,
		"TZ":                              true,
	}
}

func predefinedRuntimeEnvVarKeys() map[string]bool {
	return map[string]bool{
		"_HANDLER":                   true,
		"AWS_EXECUTION_ENV":          true,
		"AWS_LAMBDA_LOG_GROUP_NAME":  true,
		"AWS_LAMBDA_LOG_STREAM_NAME": true,
		"LAMBDA_TASK_ROOT":           true,
		"LAMBDA_RUNTIME_DIR":         true,
	}
}

func predefinedPlatformUnreservedEnvVarKeys() map[string]bool {
	return map[string]bool{
		// AWS_XRAY_DAEMON_ADDRESS is unreserved but RAPID boot depends on it
		"AWS_XRAY_DAEMON_ADDRESS": true,
	}
}

func predefinedCredentialsEnvVarKeys() map[string]bool {
	return map[string]bool{
		"AWS_ACCESS_KEY_ID":     true,
		"AWS_SECRET_ACCESS_KEY": true,
		"AWS_SESSION_TOKEN":     true,
	}
}

func extensionExcludedKeys() map[string]bool {
	return map[string]bool{
		"AWS_XRAY_CONTEXT_MISSING": true,
		"_AWS_XRAY_DAEMON_ADDRESS": true,
		"_AWS_XRAY_DAEMON_PORT":    true,
		"_LAMBDA_TELEMETRY_LOG_FD": true,
	}
}
