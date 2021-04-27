// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package fatalerror

// This package defines constant error types returned to slicer with DONE(failure)
// Separate package for namespacing

// ErrorType is returned to slicer inside DONE
type ErrorType string

const (
	AgentInitError    ErrorType = "Extension.InitError"   // agent exited after calling /extension/init/error
	AgentExitError    ErrorType = "Extension.ExitError"   // agent exited after calling /extension/exit/error
	AgentCrash        ErrorType = "Extension.Crash"       // agent crashed unexpectedly
	AgentLaunchError  ErrorType = "Extension.LaunchError" // agent could not be launched
	RuntimeExit       ErrorType = "Runtime.ExitError"
	InvalidEntrypoint ErrorType = "Runtime.InvalidEntrypoint"
	InvalidWorkingDir ErrorType = "Runtime.InvalidWorkingDir"
	InvalidTaskConfig ErrorType = "Runtime.InvalidTaskConfig"
	Unknown           ErrorType = "Unknown"
)
