// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"strconv"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// RapidConfig holds config req'd for RAPID's internal
// operation, parsed from internal env vars.
// It should be build using `NewRapidConfig` to make sure that all the
// internal invariants are respected.
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

// Build the `RapidConfig` struct checking all the internal invariants
func NewRapidConfig(e *Environment) RapidConfig {
	return RapidConfig{
		SbID:                   getStrEnvVarOrDie(e.rapid, "_LAMBDA_SB_ID"),
		LogFd:                  getSocketEnvVarOrDie(e.rapid, "_LAMBDA_LOG_FD"),
		ShmFd:                  getSocketEnvVarOrDie(e.rapid, "_LAMBDA_SHARED_MEM_FD"),
		CtrlFd:                 getSocketEnvVarOrDie(e.rapid, "_LAMBDA_CONTROL_SOCKET"),
		CnslFd:                 getSocketEnvVarOrDie(e.rapid, "_LAMBDA_CONSOLE_SOCKET"),
		DirectInvokeFd:         getOptionalSocketEnvVar(e.rapid, "_LAMBDA_DIRECT_INVOKE_SOCKET"),
		PreLoadTimeNs:          getInt64EnvVarOrDie(e.rapid, "_LAMBDA_RUNTIME_LOAD_TIME"),
		LambdaTaskRoot:         getStrEnvVarOrDie(e.runtime, "LAMBDA_TASK_ROOT"),
		XrayDaemonAddress:      getStrEnvVarOrDie(e.platformUnreserved, "AWS_XRAY_DAEMON_ADDRESS"),
		FunctionName:           getStrEnvVarOrDie(e.platform, "AWS_LAMBDA_FUNCTION_NAME"),
		TelemetryAPIPassphrase: e.rapid["_LAMBDA_TELEMETRY_API_PASSPHRASE"], // TODO: Die if not set
	}
}

func getStrEnvVarOrDie(env map[string]string, name string) string {
	val, ok := env[name]
	if !ok {
		log.WithField("name", name).Fatal("Environment variable is not set")
	}
	return val
}

func getInt64EnvVarOrDie(env map[string]string, name string) int64 {
	strval := getStrEnvVarOrDie(env, name)
	val, err := strconv.ParseInt(strval, 10, 64)
	if err != nil {
		log.WithError(err).WithField("name", name).Fatal("Unable to parse int env var.")
	}
	return val
}

func getIntEnvVarOrDie(env map[string]string, name string) int {
	return int(getInt64EnvVarOrDie(env, name))
}

// getSocketEnvVarOrDie reads and returns an int value of the
// environment variable or dies, when unable to do so.
// It also makes CloseOnExec for this value.
func getSocketEnvVarOrDie(env map[string]string, name string) int {
	sock := getIntEnvVarOrDie(env, name)
	syscall.CloseOnExec(sock)
	return sock
}

// returns -1 if env variable was not set. Exits if it holds unexpected (non-int) value
func getOptionalSocketEnvVar(env map[string]string, name string) int {
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
