// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package fatalerror

import (
	"regexp"
	"strings"
)

// This package defines constant error types returned to slicer with DONE(failure), and also sandbox errors
// Separate package for namespacing

// ErrorType is returned to slicer inside DONE
type ErrorType string

// TODO: Find another name than "fatalerror"
// TODO: Rename all const so that they always begin with Agent/Runtime/Sandbox/Function
// TODO: Add filtering for extensions as well
const (
	// Extension errors
	AgentInitError   ErrorType = "Extension.InitError"   // agent exited after calling /extension/init/error
	AgentExitError   ErrorType = "Extension.ExitError"   // agent exited after calling /extension/exit/error
	AgentCrash       ErrorType = "Extension.Crash"       // agent crashed unexpectedly
	AgentLaunchError ErrorType = "Extension.LaunchError" // agent could not be launched

	// Runtime errors
	RuntimeExit                      ErrorType = "Runtime.ExitError"
	InvalidEntrypoint                ErrorType = "Runtime.InvalidEntrypoint"
	InvalidWorkingDir                ErrorType = "Runtime.InvalidWorkingDir"
	InvalidTaskConfig                ErrorType = "Runtime.InvalidTaskConfig"
	TruncatedResponse                ErrorType = "Runtime.TruncatedResponse"
	RuntimeInvalidResponseModeHeader ErrorType = "Runtime.InvalidResponseModeHeader"
	RuntimeUnknown                   ErrorType = "Runtime.Unknown"

	// Function errors
	FunctionOversizedResponse ErrorType = "Function.ResponseSizeTooLarge"
	FunctionUnknown           ErrorType = "Function.Unknown"

	// Sandbox errors
	SandboxFailure ErrorType = "Sandbox.Failure"
	SandboxTimeout ErrorType = "Sandbox.Timeout"
)

var validRuntimeAndFunctionErrors = map[ErrorType]struct{}{
	// Runtime errors
	RuntimeExit:                      {},
	InvalidEntrypoint:                {},
	InvalidWorkingDir:                {},
	InvalidTaskConfig:                {},
	TruncatedResponse:                {},
	RuntimeInvalidResponseModeHeader: {},
	RuntimeUnknown:                   {},

	// Function errors
	FunctionOversizedResponse: {},
	FunctionUnknown:           {},
}

func GetValidRuntimeOrFunctionErrorType(errorType string) ErrorType {
	match, _ := regexp.MatchString("(Runtime|Function)\\.[A-Z][a-zA-Z]+", errorType)
	if match {
		return ErrorType(errorType)
	}

	if strings.HasPrefix(errorType, "Function.") {
		return FunctionUnknown
	}

	return RuntimeUnknown
}
