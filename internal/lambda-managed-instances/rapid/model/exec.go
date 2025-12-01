// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils/invariant"
)

type RuntimeExecError struct {
	Type RuntimeExecErrorType
	Err  error
}

type RuntimeExec struct {
	Env        model.KVMap
	Cmd        []string
	WorkingDir string
}

type Runtime struct {
	ExecConfig  RuntimeExec
	ConfigError *RuntimeExecError
}

type ExtensionsExec struct {
	Env        model.KVMap
	WorkingDir string
}

type ExternalAgents struct {
	Bootstraps []string
	ExecConfig ExtensionsExec
}

type RuntimeExecErrorType int

const (
	InvalidTaskConfig RuntimeExecErrorType = iota
	InvalidEntrypoint
	InvalidWorkingDir
)

func (e RuntimeExecErrorType) FatalErrorType() ErrorType {
	switch e {
	case InvalidEntrypoint:
		return ErrorRuntimeInvalidEntryPoint
	case InvalidWorkingDir:
		return ErrorRuntimeInvalidWorkingDir
	}

	invariant.Violatef("invalid runtime exec error value: %d", int(e))
	return ErrorType("")
}
