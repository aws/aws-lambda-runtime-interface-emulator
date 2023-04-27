// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
)

const runtimeAPIAddressKey = "AWS_LAMBDA_RUNTIME_API"
const handlerEnvKey = "_HANDLER"
const executionEnvKey = "AWS_EXECUTION_ENV"

// Environment holds env vars for runtime, agents, and for
// internal use, parsed during startup and from START msg
type Environment struct {
	RAPID              map[string]string // env vars req'd internally by RAPID
	Platform           map[string]string // reserved platform env vars as per Lambda docs
	Runtime            map[string]string // reserved runtime env vars as per Lambda docs
	PlatformUnreserved map[string]string // unreserved platform env vars that customers can override
	Credentials        map[string]string // reserved env vars for credentials, set on INIT
	Customer           map[string]string // customer & unreserved platform env vars, set on INIT

	runtimeAPISet  bool
	initEnvVarsSet bool
}

// RapidConfig holds config req'd for RAPID's internal
// operation, parsed from internal env vars.
type RapidConfig struct {
	SbID                   string
	LogFd                  int
	ShmFd                  int
	CtrlFd                 int
	CnslFd                 int
	DirectInvokeFd         int
	LambdaTaskRoot         string
	XrayDaemonAddress      string
	PreLoadTimeNs          int64
	FunctionName           string
	TelemetryAPIPassphrase string
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
		RAPID:              lookupEnv(predefinedInternalEnvVarKeys()),
		Platform:           lookupEnv(predefinedPlatformEnvVarKeys()),
		Runtime:            lookupEnv(predefinedRuntimeEnvVarKeys()),
		PlatformUnreserved: lookupEnv(predefinedPlatformUnreservedEnvVarKeys()),

		Credentials: map[string]string{},
		Customer:    map[string]string{},

		runtimeAPISet:  false,
		initEnvVarsSet: false,
	}

}

// StoreRuntimeAPIEnvironmentVariable stores value for AWS_LAMBDA_RUNTIME_API
func (e *Environment) StoreRuntimeAPIEnvironmentVariable(runtimeAPIAddress string) {
	e.Platform[runtimeAPIAddressKey] = runtimeAPIAddress
	e.runtimeAPISet = true
}

// GetHandler turns the current setting for handler
func (e *Environment) GetHandler() string {
	return e.Runtime[handlerEnvKey]
}

// SetHandler sets _HANDLER env variable value for Runtime
func (e *Environment) SetHandler(handler string) {
	e.Runtime[handlerEnvKey] = handler
}

// GetExecutionEnv returns the current setting for AWS_EXECUTION_ENV
func (e *Environment) GetExecutionEnv() string {
	return e.Runtime[executionEnvKey]
}

// SetExecutionEnv sets AWS_EXECUTION_ENV variable value for Runtime
func (e *Environment) SetExecutionEnv(executionEnv string) {
	e.Runtime[executionEnvKey] = executionEnv
}

// StoreEnvironmentVariablesFromInit sets the environment variables
// for credentials & _HANDLER which are received in the START message
func (e *Environment) StoreEnvironmentVariablesFromInit(customerEnv map[string]string, handler, awsKey, awsSecret, awsSession, funcName, funcVer string) {

	e.Credentials["AWS_ACCESS_KEY_ID"] = awsKey
	e.Credentials["AWS_SECRET_ACCESS_KEY"] = awsSecret
	e.Credentials["AWS_SESSION_TOKEN"] = awsSession

	e.storeNonCredentialEnvironmentVariablesFromInit(customerEnv, handler, funcName, funcVer)
}

func (e *Environment) StoreEnvironmentVariablesFromInitForInitCaching(host string, port int, customerEnv map[string]string, handler, funcName, funcVer, token string) {
	e.Credentials["AWS_CONTAINER_CREDENTIALS_FULL_URI"] = fmt.Sprintf("http://%s:%d/2021-04-23/credentials", host, port)
	e.Credentials["AWS_CONTAINER_AUTHORIZATION_TOKEN"] = token

	e.storeNonCredentialEnvironmentVariablesFromInit(customerEnv, handler, funcName, funcVer)
}

func (e *Environment) storeNonCredentialEnvironmentVariablesFromInit(customerEnv map[string]string, handler, funcName, funcVer string) {
	if handler != "" {
		e.SetHandler(handler)
	}

	if funcName != "" {
		e.Platform["AWS_LAMBDA_FUNCTION_NAME"] = funcName
	}

	if funcVer != "" {
		e.Platform["AWS_LAMBDA_FUNCTION_VERSION"] = funcVer
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

	return mapUnion(e.Customer, e.PlatformUnreserved, e.Credentials, e.Runtime, e.Platform)
}

// AgentExecEnv returns the key=value strings of all environment variables
// passed to agent process on exec()
func (e *Environment) AgentExecEnv() map[string]string {
	if !e.initEnvVarsSet || !e.runtimeAPISet {
		log.Fatal("credentials, customer and runtime API address must be set")
	}

	excludedKeys := extensionExcludedKeys()
	excludeCondition := func(key string) bool { return excludedKeys[key] || strings.HasPrefix(key, "_") }
	return mapExclude(mapUnion(e.Customer, e.Credentials, e.Platform), excludeCondition)
}

// RAPIDInternalConfig returns the rapid config parsed from environment vars
func (e *Environment) RAPIDInternalConfig() RapidConfig {
	return RapidConfig{
		SbID:                   e.getStrEnvVarOrDie(e.RAPID, "_LAMBDA_SB_ID"),
		LogFd:                  e.getSocketEnvVarOrDie(e.RAPID, "_LAMBDA_LOG_FD"),
		ShmFd:                  e.getSocketEnvVarOrDie(e.RAPID, "_LAMBDA_SHARED_MEM_FD"),
		CtrlFd:                 e.getSocketEnvVarOrDie(e.RAPID, "_LAMBDA_CONTROL_SOCKET"),
		CnslFd:                 e.getSocketEnvVarOrDie(e.RAPID, "_LAMBDA_CONSOLE_SOCKET"),
		DirectInvokeFd:         e.getOptionalSocketEnvVar(e.RAPID, "_LAMBDA_DIRECT_INVOKE_SOCKET"),
		PreLoadTimeNs:          e.getInt64EnvVarOrDie(e.RAPID, "_LAMBDA_RUNTIME_LOAD_TIME"),
		LambdaTaskRoot:         e.getStrEnvVarOrDie(e.Runtime, "LAMBDA_TASK_ROOT"),
		XrayDaemonAddress:      e.getStrEnvVarOrDie(e.PlatformUnreserved, "AWS_XRAY_DAEMON_ADDRESS"),
		FunctionName:           e.getStrEnvVarOrDie(e.Platform, "AWS_LAMBDA_FUNCTION_NAME"),
		TelemetryAPIPassphrase: e.RAPID["_LAMBDA_TELEMETRY_API_PASSPHRASE"], // TODO: Die if not set
	}
}

func (e *Environment) getStrEnvVarOrDie(env map[string]string, name string) string {
	val, ok := env[name]
	if !ok {
		log.WithField("name", name).Fatal("Environment variable is not set")
	}
	return val
}

func (e *Environment) getInt64EnvVarOrDie(env map[string]string, name string) int64 {
	strval := e.getStrEnvVarOrDie(env, name)
	val, err := strconv.ParseInt(strval, 10, 64)
	if err != nil {
		log.WithError(err).WithField("name", name).Fatal("Unable to parse int env var.")
	}
	return val
}

func (e *Environment) getIntEnvVarOrDie(env map[string]string, name string) int {
	return int(e.getInt64EnvVarOrDie(env, name))
}

// getSocketEnvVarOrDie reads and returns an int value of the
// environment variable or dies, when unable to do so.
// It also makes CloseOnExec for this value.
func (e *Environment) getSocketEnvVarOrDie(env map[string]string, name string) int {
	sock := e.getIntEnvVarOrDie(env, name)
	syscall.CloseOnExec(sock)
	return sock
}

// returns -1 if env variable was not set. Exits if it holds unexpected (non-int) value
func (e *Environment) getOptionalSocketEnvVar(env map[string]string, name string) int {
	val, found := env[name]
	if !found {
		return -1
	}

	sock, err := strconv.Atoi(val)
	if err != nil {
		log.WithError(err).WithField("name", name).Fatal("Unable to parse socket env var.")
	}

	if sock < 0 {
		log.WithError(err).WithField("name", name).Fatal("Negative socket descriptor value")
	}

	syscall.CloseOnExec(sock)
	return sock
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
