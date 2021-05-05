// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

func isInternalEnvVar(envKey string) bool {
	// the rule is no '_' prefixed env. variables will be propagated to the runtime but the ones explicitly exempted
	allowedKeys := map[string]bool{
		"_HANDLER":                 true,
		"_AWS_XRAY_DAEMON_ADDRESS": true,
		"_AWS_XRAY_DAEMON_PORT":    true,
		"_LAMBDA_TELEMETRY_LOG_FD": true,
	}
	return strings.HasPrefix(envKey, "_") && !allowedKeys[envKey]
}

// CustomerEnvironmentVariables parses all environment variables that are
// not internal/credential/platform, and must be called before agent bootstrap.
func CustomerEnvironmentVariables() map[string]string {
	internalKeys := predefinedInternalEnvVarKeys()
	platformKeys := predefinedPlatformEnvVarKeys()
	runtimeKeys := predefinedRuntimeEnvVarKeys()
	credentialKeys := predefinedCredentialsEnvVarKeys()
	platformUnreservedKeys := predefinedPlatformUnreservedEnvVarKeys()
	isCustomer := func(key string) bool {
		return !internalKeys[key] &&
			!runtimeKeys[key] &&
			!platformKeys[key] &&
			!credentialKeys[key] &&
			!platformUnreservedKeys[key] &&
			!isInternalEnvVar(key)
	}

	customerEnv := map[string]string{}
	for _, keyval := range os.Environ() {
		key, val, err := SplitEnvironmentVariable(keyval)
		if err != nil {
			log.Warnf("Customer environment variable with invalid format: %s", err)
			continue
		}

		if isCustomer(key) {
			customerEnv[key] = val
		}
	}

	return customerEnv
}
