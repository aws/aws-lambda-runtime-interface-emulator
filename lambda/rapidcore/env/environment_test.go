// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRAPIDInternalConfig(t *testing.T) {
	os.Clearenv()
	os.Setenv("_LAMBDA_SB_ID", "sbid")
	os.Setenv("_LAMBDA_LOG_FD", "1")
	os.Setenv("_LAMBDA_SHARED_MEM_FD", "1")
	os.Setenv("_LAMBDA_CONTROL_SOCKET", "1")
	os.Setenv("_LAMBDA_CONSOLE_SOCKET", "1")
	os.Setenv("_LAMBDA_RUNTIME_LOAD_TIME", "1")
	os.Setenv("LAMBDA_TASK_ROOT", "a")
	os.Setenv("AWS_XRAY_DAEMON_ADDRESS", "a")
	os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "a")
	os.Setenv("_LAMBDA_TELEMETRY_API_PASSPHRASE", "a")
	os.Setenv("_LAMBDA_DIRECT_INVOKE_SOCKET", "1")
	NewEnvironment().RAPIDInternalConfig()
}

func TestEnvironmentParsing(t *testing.T) {
	internalEnvVal, platformEnvVal, credsEnvVal := "rapid", "platform", "creds"
	runtimeEnvVal := "runtime"
	customerEnvVal := "customer=foo=bar"
	runtimeAPIAddress := "host:port"

	os.Clearenv()
	setAll(predefinedInternalEnvVarKeys(), internalEnvVal)
	setAll(predefinedPlatformEnvVarKeys(), platformEnvVal)
	setAll(predefinedRuntimeEnvVarKeys(), runtimeEnvVal)
	setAll(predefinedPlatformUnreservedEnvVarKeys(), customerEnvVal)
	setAll(predefinedCredentialsEnvVarKeys(), credsEnvVal)
	os.Setenv("MY_FOOBAR_ENV_1", customerEnvVal)
	os.Setenv("MY_EMPTY_ENV", "")
	os.Setenv("_UNKNOWN_INTERNAL_ENV", platformEnvVal)

	env := NewEnvironment() // parse environment variables
	customerEnv := CustomerEnvironmentVariables()

	env.StoreRuntimeAPIEnvironmentVariable(runtimeAPIAddress)
	env.StoreEnvironmentVariablesFromInit(customerEnv, runtimeEnvVal, credsEnvVal, credsEnvVal, credsEnvVal, platformEnvVal, platformEnvVal)

	for _, val := range env.RAPID {
		assert.Equal(t, internalEnvVal, val)
	}

	for key, val := range env.Platform {
		if key == runtimeAPIAddressKey {
			assert.Equal(t, runtimeAPIAddress, val)
		} else {
			assert.Equal(t, platformEnvVal, val)
		}
	}

	for _, val := range env.Runtime {
		assert.Equal(t, runtimeEnvVal, val)
	}

	for key, val := range env.Credentials {
		assert.Equal(t, credsEnvVal, val)
		assert.NotContains(t, env.Customer, key)
	}

	for _, val := range env.PlatformUnreserved {
		assert.Equal(t, customerEnvVal, val)
	}

	assert.Equal(t, customerEnvVal, env.Customer["MY_FOOBAR_ENV_1"])
	assert.Equal(t, "", env.Customer["MY_EMPTY_ENV"])
	assert.Equal(t, "", env.Customer["_UNKNOWN_INTERNAL_ENV"])
}

func TestEnvironmentParsingUnsetPlatformAndInternalEnvVarKeysAreDeleted(t *testing.T) {
	// Done to ensure that we can continue to distinguish between unset and empty env vars
	os.Clearenv()
	env := NewEnvironment()

	assert.Len(t, env.RAPID, 0)
	assert.Len(t, env.Platform, 0)
	assert.Len(t, env.PlatformUnreserved, 0)
	assert.Len(t, env.Credentials, 0) // uninitialized
	assert.Len(t, env.Customer, 0)    // uninitialized
}

func TestRuntimeExecEnvironmentVariables(t *testing.T) {
	internalEnvVal, platformEnvVal, credsEnvVal := "rapid", "platform", "creds"
	customerEnvVal, platformUnreservedEnvVal := "customer", "platform-unreserved"
	lcisCLIArgEnvVal := "lcis"
	runtimeAPIAddress := "host:port"
	runtimeEnvVal := "runtime"

	os.Clearenv()
	setAll(predefinedInternalEnvVarKeys(), internalEnvVal)
	setAll(predefinedPlatformEnvVarKeys(), platformEnvVal)
	setAll(predefinedRuntimeEnvVarKeys(), runtimeEnvVal)
	setAll(predefinedPlatformUnreservedEnvVarKeys(), platformUnreservedEnvVal)
	setAll(predefinedCredentialsEnvVarKeys(), credsEnvVal)
	customerEnv := map[string]string{
		"MY_FOOBAR_ENV_1": customerEnvVal,
	}

	cliOptionsEnv := map[string]string{
		"LCIS_ARG1": lcisCLIArgEnvVal,
	}

	env := NewEnvironment()
	env.StoreRuntimeAPIEnvironmentVariable(runtimeAPIAddress)
	env.StoreEnvironmentVariablesFromCLIOptions(cliOptionsEnv)
	env.StoreEnvironmentVariablesFromInit(customerEnv, runtimeEnvVal, credsEnvVal, credsEnvVal, credsEnvVal, platformEnvVal, platformEnvVal)

	rapidEnvVars := env.RuntimeExecEnv()

	var rapidEnvKeys []string
	for _, keyval := range rapidEnvVars {
		key := strings.Split(keyval, "=")[0]
		rapidEnvKeys = append(rapidEnvKeys, key)
	}

	for key := range env.RAPID {
		assert.NotContains(t, rapidEnvKeys, key)
	}

	for key, val := range env.Runtime {
		assert.Contains(t, rapidEnvVars, key+"="+val)
	}

	for key, val := range env.Platform {
		assert.Contains(t, rapidEnvVars, key+"="+val)
	}

	for key, val := range env.PlatformUnreserved {
		assert.Contains(t, rapidEnvVars, key+"="+val)
		assert.NotContains(t, env.Customer, key)
	}

	for key, val := range env.Credentials {
		assert.Contains(t, rapidEnvVars, key+"="+val)
	}

	for key, val := range env.Customer {
		assert.Contains(t, rapidEnvVars, key+"="+val)
		assert.NotContains(t, env.PlatformUnreserved, key)
	}
}

func TestRuntimeExecEnvironmentVariablesPriority(t *testing.T) {
	internalEnvVal, platformEnvVal, credsEnvVal := "rapid", "platform", "creds"
	customerEnvVal, platformUnreservedEnvVal := "customer", "platform-unreserved"
	runtimeEnvVal := "runtime"
	lcisCLIArgEnvVal := "lcis"
	runtimeAPIAddress := "host:port"

	os.Clearenv()
	setAll(predefinedInternalEnvVarKeys(), internalEnvVal)
	setAll(predefinedPlatformEnvVarKeys(), platformEnvVal)
	setAll(predefinedPlatformUnreservedEnvVarKeys(), platformUnreservedEnvVal)
	setAll(predefinedCredentialsEnvVarKeys(), credsEnvVal)
	setAll(predefinedRuntimeEnvVarKeys(), runtimeEnvVal)

	conflictPlatformKeyFromInit := "AWS_REGION"
	conflictPlatformKeyFromCLI := "LAMBDA_TASK_ROOT"

	customerEnv := map[string]string{
		"MY_FOOBAR_ENV_1":           customerEnvVal,
		conflictPlatformKeyFromInit: customerEnvVal,
	}

	cliOptionsEnv := map[string]string{
		"LCIS_ARG1":                lcisCLIArgEnvVal,
		conflictPlatformKeyFromCLI: lcisCLIArgEnvVal,
	}

	env := NewEnvironment()
	env.StoreRuntimeAPIEnvironmentVariable(runtimeAPIAddress)
	env.StoreEnvironmentVariablesFromCLIOptions(cliOptionsEnv)
	env.StoreEnvironmentVariablesFromInit(customerEnv, runtimeEnvVal, credsEnvVal, credsEnvVal, credsEnvVal, platformEnvVal, platformEnvVal)

	assert.Equal(t, len(predefinedPlatformEnvVarKeys()), len(env.Platform))
	assert.Equal(t, len(predefinedCredentialsEnvVarKeys()), len(env.Credentials))
	assert.Equal(t, len(predefinedPlatformUnreservedEnvVarKeys()), len(env.PlatformUnreserved))
	assert.Equal(t, len(predefinedInternalEnvVarKeys()), len(env.RAPID))
	assert.Equal(t, len(predefinedRuntimeEnvVarKeys()), len(env.Runtime))

	rapidEnvVars := env.RuntimeExecEnv()

	// Customer env vars cannot override platform/internal ones
	assert.NotContains(t, rapidEnvVars, conflictPlatformKeyFromInit+"="+customerEnvVal)
	assert.NotContains(t, rapidEnvVars, conflictPlatformKeyFromCLI+"="+lcisCLIArgEnvVal)
	assert.Contains(t, rapidEnvVars, conflictPlatformKeyFromInit+"="+platformEnvVal)
	assert.Contains(t, rapidEnvVars, conflictPlatformKeyFromCLI+"="+runtimeEnvVal)
}

func TestCustomerEnvironmentVariablesFromInitCanOverrideEnvironmentVariablesFromCLIOptions(t *testing.T) {
	platformEnvVal, credsEnvVal, customerEnvVal := "platform", "creds", "customer"
	lcisCLIArgEnvVal := "lcis"
	runtimeAPIAddress := "host:port"
	runtimeEnvVal := "runtime"

	os.Clearenv()
	customerEnv := map[string]string{
		"MY_FOOBAR_ENV_1": customerEnvVal,
	}

	cliOptionsEnv := map[string]string{
		"LCIS_ARG1":       lcisCLIArgEnvVal,
		"MY_FOOBAR_ENV_1": lcisCLIArgEnvVal,
	}

	env := NewEnvironment()
	env.StoreRuntimeAPIEnvironmentVariable(runtimeAPIAddress)
	env.StoreEnvironmentVariablesFromCLIOptions(cliOptionsEnv)
	env.StoreEnvironmentVariablesFromInit(customerEnv, runtimeEnvVal, credsEnvVal, credsEnvVal, credsEnvVal, platformEnvVal, platformEnvVal)

	assert.Equal(t, env.Customer["LCIS_ARG1"], lcisCLIArgEnvVal)
	assert.Equal(t, env.Customer["MY_FOOBAR_ENV_1"], customerEnvVal)

	rapidEnvVars := env.RuntimeExecEnv()

	assert.Contains(t, rapidEnvVars, "LCIS_ARG1="+lcisCLIArgEnvVal)
	assert.Contains(t, rapidEnvVars, "MY_FOOBAR_ENV_1="+customerEnvVal)
}

func TestAgentExecEnvironmentVariables(t *testing.T) {
	internalEnvVal, platformEnvVal, credsEnvVal := "rapid", "platform", "creds"
	customerEnvVal, platformUnreservedEnvVal := "customer", "platform-unreserved"
	runtimeAPIAddress := "host:port"
	runtimeEnvVal := "runtime"

	os.Clearenv()
	setAll(predefinedInternalEnvVarKeys(), internalEnvVal)
	setAll(predefinedPlatformEnvVarKeys(), platformEnvVal)
	setAll(predefinedPlatformUnreservedEnvVarKeys(), platformUnreservedEnvVal)
	setAll(predefinedCredentialsEnvVarKeys(), credsEnvVal)
	customerEnv := map[string]string{"MY_FOOBAR_ENV_1": customerEnvVal}

	env := NewEnvironment()
	env.StoreRuntimeAPIEnvironmentVariable(runtimeAPIAddress)
	env.StoreEnvironmentVariablesFromInit(customerEnv, runtimeEnvVal, credsEnvVal, credsEnvVal, credsEnvVal, platformEnvVal, platformEnvVal)

	agentEnvVars := env.AgentExecEnv()

	var agentEnvKeys []string
	for _, keyval := range agentEnvVars {
		key := strings.Split(keyval, "=")[0]
		agentEnvKeys = append(agentEnvKeys, key)
	}

	for key := range env.RAPID {
		assert.NotContains(t, agentEnvKeys, key)
	}

	for key, val := range env.Runtime {
		assert.NotContains(t, agentEnvKeys, key+"="+val)
	}

	for key := range env.Platform {
		assert.Contains(t, agentEnvKeys, key)
	}

	for key := range env.Customer {
		assert.Contains(t, agentEnvKeys, key)
	}

	for key, val := range env.Credentials {
		assert.Contains(t, agentEnvVars, key+"="+val)
	}

	assert.Contains(t, agentEnvVars, runtimeAPIAddressKey+"="+env.Platform[runtimeAPIAddressKey])
}

func TestStoreEnvironmentVariablesFromInitCaching(t *testing.T) {
	host := "samplehost"
	port := 1234
	handler := "samplehandler"
	funcName := "samplefunctionname"
	funcVer := "samplefunctionver"
	token := "sampletoken"
	env := NewEnvironment()
	customerEnv := CustomerEnvironmentVariables()

	env.StoreEnvironmentVariablesFromInitForInitCaching("samplehost", 1234, customerEnv, handler, funcName, funcVer, token)

	assert.Equal(t, fmt.Sprintf("http://%s:%d/2021-04-23/credentials", host, port), env.Credentials["AWS_CONTAINER_CREDENTIALS_FULL_URI"])
	assert.Equal(t, token, env.Credentials["AWS_CONTAINER_AUTHORIZATION_TOKEN"])
	assert.Equal(t, funcName, env.Platform["AWS_LAMBDA_FUNCTION_NAME"])
	assert.Equal(t, funcVer, env.Platform["AWS_LAMBDA_FUNCTION_VERSION"])
	assert.Equal(t, handler, env.Runtime["_HANDLER"])
}

func setAll(keys map[string]bool, value string) {
	for key := range keys {
		os.Setenv(key, value)
	}
}
