// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

type EnvironmentVariables interface {
	AgentExecEnv() map[string]string
	RuntimeExecEnv() map[string]string
	SetHandler(handler string)
	StoreRuntimeAPIEnvironmentVariable(runtimeAPIAddress string)
	StoreEnvironmentVariablesFromInit(customerEnv map[string]string,
		handler, awsKey, awsSecret, awsSession, funcName, funcVer string)
	StoreEnvironmentVariablesFromInitForInitCaching(host string, port int, customerEnv map[string]string, handler, funcName, funcVer, token string)
}
