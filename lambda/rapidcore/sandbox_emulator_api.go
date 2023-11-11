// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"go.amzn.com/lambda/interop"

	"net/http"
)

// LambdaInvokeAPI are the methods used by the Runtime Interface Emulator
type LambdaInvokeAPI interface {
	Init(i *interop.Init, invokeTimeoutMs int64)
	Invoke(responseWriter http.ResponseWriter, invoke *interop.Invoke) error
}

// EmulatorAPI wraps the standalone interop server to provide a convenient interface
// for Rapid Standalone
type EmulatorAPI struct {
	server *Server
}

// Validate interface compliance
var _ LambdaInvokeAPI = (*EmulatorAPI)(nil)

func NewEmulatorAPI(s *Server) *EmulatorAPI {
	return &EmulatorAPI{s}
}

// Init method is only used by the Runtime interface emulator
func (l *EmulatorAPI) Init(i *interop.Init, timeoutMs int64) {
	l.server.Init(&interop.Init{
		AccountID:                    i.AccountID,
		Handler:                      i.Handler,
		AwsKey:                       i.AwsKey,
		AwsSecret:                    i.AwsSecret,
		AwsSession:                   i.AwsSession,
		XRayDaemonAddress:            i.XRayDaemonAddress,
		FunctionName:                 i.FunctionName,
		FunctionVersion:              i.FunctionVersion,
		CustomerEnvironmentVariables: i.CustomerEnvironmentVariables,
		RuntimeInfo:                  i.RuntimeInfo,
		SandboxType:                  i.SandboxType,
		Bootstrap:                    i.Bootstrap,
		EnvironmentVariables:         i.EnvironmentVariables,
	}, timeoutMs)
}

// Invoke method is only used by the Runtime interface emulator
func (l *EmulatorAPI) Invoke(w http.ResponseWriter, i *interop.Invoke) error {
	return l.server.Invoke(w, i)
}
