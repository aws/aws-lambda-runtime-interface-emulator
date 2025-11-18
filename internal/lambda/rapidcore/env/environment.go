// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

const runtimeAPIAddressKey = "AWS_LAMBDA_RUNTIME_API"
const handlerEnvKey = "_HANDLER"
const executionEnvKey = "AWS_EXECUTION_ENV"
const taskRootEnvKey = "LAMBDA_TASK_ROOT"
const runtimeDirEnvKey = "LAMBDA_RUNTIME_DIR"

// Environment holds env vars for runtime, agents, and for
// internal use, parsed during startup and from START msg
type Environment struct {
	Customer map[string]string // customer & unreserved platform env vars, set on INIT

	rapid              map[string]string // env vars req'd internally by RAPID
	platform           map[string]string // reserved platform env vars as per Lambda docs
	runtime            map[string]string // reserved runtime env vars as per Lambda docs
	platformUnreserved map[string]string // unreserved platform env vars that customers can override
	credentials        map[string]string // reserved env vars for credentials, set on INIT

	runtimeAPISet  bool
	initEnvVarsSet bool
}

func lookupEnv(keys map[string]bool) map[string]string {
	res := map[string]string{}
	for key := range keys {
		val, ok := os.LookupEnv(key)
		if ok {
			res[key] = val
		}
	}
	return res
}

// NewEnvironment parses environment variables into an Environment object
func NewEnvironment() *Environment {
	return &Environment{
		rapid:              lookupEnv(predefinedInternalEnvVarKeys()),
		platform:           lookupEnv(predefinedPlatformEnvVarKeys()),
		runtime:            lookupEnv(predefinedRuntimeEnvVarKeys()),
		platformUnreserved: lookupEnv(predefinedPlatformUnreservedEnvVarKeys()),

		Customer:    map[string]string{},
		credentials: map[string]string{},

		runtimeAPISet:  false,
		initEnvVarsSet: false,
	}

}

// StoreRuntimeAPIEnvironmentVariable stores value for AWS_LAMBDA_RUNTIME_API
func (e *Environment) StoreRuntimeAPIEnvironmentVariable(runtimeAPIAddress string) {
	e.platform[runtimeAPIAddressKey] = runtimeAPIAddress
	e.runtimeAPISet = true
}

// SetHandler sets _HANDLER env variable value for Runtime
func (e *Environment) SetHandler(handler string) {
	e.runtime[handlerEnvKey] = handler
}

// GetExecutionEnv returns the current setting for AWS_EXECUTION_ENV
func (e *Environment) GetExecutionEnv() string {
	return e.runtime[executionEnvKey]
}

// SetExecutionEnv sets AWS_EXECUTION_ENV variable value for Runtime
func (e *Environment) SetExecutionEnv(executionEnv string) {
	e.runtime[executionEnvKey] = executionEnv
}

// SetTaskRoot sets the LAMBDA_TASK_ROOT environment variable for Runtime
func (e *Environment) SetTaskRoot(taskRoot string) {
	e.runtime[taskRootEnvKey] = taskRoot
}

// SetRuntimeDir sets the LAMBDA_RUNTIME_DIR environment variable for Runtime
func (e *Environment) SetRuntimeDir(runtimeDir string) {
	e.runtime[runtimeDirEnvKey] = runtimeDir
}

// StoreEnvironmentVariablesFromInit sets the environment variables
// for credentials & _HANDLER which are received in the START message
func (e *Environment) StoreEnvironmentVariablesFromInit(customerEnv map[string]string, handler, awsKey, awsSecret, awsSession, funcName, funcVer string) {

	e.credentials["AWS_ACCESS_KEY_ID"] = awsKey
	e.credentials["AWS_SECRET_ACCESS_KEY"] = awsSecret
	e.credentials["AWS_SESSION_TOKEN"] = awsSession

	e.storeNonCredentialEnvironmentVariablesFromInit(customerEnv, handler, funcName, funcVer)
}

func (e *Environment) StoreEnvironmentVariablesFromInitForInitCaching(host string, port int, customerEnv map[string]string, handler, funcName, funcVer, token string) {
	e.credentials["AWS_CONTAINER_CREDENTIALS_FULL_URI"] = fmt.Sprintf("http://%s:%d/2021-04-23/credentials", host, port)
	e.credentials["AWS_CONTAINER_AUTHORIZATION_TOKEN"] = token

	e.storeNonCredentialEnvironmentVariablesFromInit(customerEnv, handler, funcName, funcVer)
}

func (e *Environment) storeNonCredentialEnvironmentVariablesFromInit(customerEnv map[string]string, handler, funcName, funcVer string) {
	if handler != "" {
		e.SetHandler(handler)
	}

	if funcName != "" {
		e.platform["AWS_LAMBDA_FUNCTION_NAME"] = funcName
	}

	if funcVer != "" {
		e.platform["AWS_LAMBDA_FUNCTION_VERSION"] = funcVer
	}

	e.mergeCustomerEnvironmentVariables(customerEnv) // overrides env vars from CLI options
	e.initEnvVarsSet = true
}

// StoreEnvironmentVariablesFromCLIOptions sets the environment
// variables received via a CLI flag, for example LCIS config
func (e *Environment) StoreEnvironmentVariablesFromCLIOptions(envVars map[string]string) {
	e.mergeCustomerEnvironmentVariables(envVars)
}

// mergeCustomerEnvironmentVariables appends to customer env vars, overwriting entries if they exist
func (e *Environment) mergeCustomerEnvironmentVariables(envVars map[string]string) {
	e.Customer = mapUnion(e.Customer, envVars)
}

// RuntimeExecEnv returns the key=value strings of all environment variables
// passed to runtime process on exec()
func (e *Environment) RuntimeExecEnv() map[string]string {
	if !e.initEnvVarsSet || !e.runtimeAPISet {
		log.Fatal("credentials, customer and runtime API address must be set")
	}

	return mapUnion(e.Customer, e.platformUnreserved, e.credentials, e.runtime, e.platform)
}

// AgentExecEnv returns the key=value strings of all environment variables
// passed to agent process on exec()
func (e *Environment) AgentExecEnv() map[string]string {
	if !e.initEnvVarsSet || !e.runtimeAPISet {
		log.Fatal("credentials, customer and runtime API address must be set")
	}

	excludedKeys := extensionExcludedKeys()
	excludeCondition := func(key string) bool { return excludedKeys[key] || strings.HasPrefix(key, "_") }
	return mapExclude(mapUnion(e.Customer, e.credentials, e.platform), excludeCondition)
}

func mapUnion(maps ...map[string]string) map[string]string {
	// last maps in argument overwrite values of ones before
	union := map[string]string{}
	for _, m := range maps {
		for key, val := range m {
			union[key] = val
		}
	}
	return union
}

func mapExclude(m map[string]string, excludeCondition func(string) bool) map[string]string {
	res := map[string]string{}
	for key, val := range m {
		if !excludeCondition(key) {
			res[key] = val
		}
	}
	return res
}
