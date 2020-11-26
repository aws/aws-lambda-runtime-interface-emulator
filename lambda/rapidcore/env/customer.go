// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

func logUnfilteredInternalEnvVars(envKey string) {
	// We would like to filter out all internal environment variables, but we
	// log this for now to get data to ensure customers aren't depending on it.
	if strings.HasPrefix(envKey, "_") {
		log.Warn("Internal environment variable not filtered")
	}
}

// CustomerEnvironmentVariables parses all environment variables that are
// not internal/credential/platform, and must be called before agent bootstrap.
func CustomerEnvironmentVariables() map[string]string {
	isInternal := predefinedInternalEnvVarKeys()
	isPlatform := predefinedPlatformEnvVarKeys()
	isRuntime := predefinedRuntimeEnvVarKeys()
	isCredential := predefinedCredentialsEnvVarKeys()
	isPlatformUnreserved := predefinedPlatformUnreservedEnvVarKeys()
	isCustomer := func(key string) bool {
		return !isInternal[key] && !isRuntime[key] && !isPlatform[key] && !isCredential[key] && !isPlatformUnreserved[key]
	}

	customerEnv := map[string]string{}
	for _, keyval := range os.Environ() {
		key, val, err := SplitEnvironmentVariable(keyval)
		if err != nil {
			log.Warnf("Customer environment variable with invalid format: %s", err)
			continue
		}

		if isCustomer(key) {
			logUnfilteredInternalEnvVars(key)
			customerEnv[key] = val
		}
	}

	return customerEnv
}
