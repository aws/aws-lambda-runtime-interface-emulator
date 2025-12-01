// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import "net/http"

const (
	ErrorReasonExtensionExecFailed    ErrorType = "ExtensionExecFailure"
	ErrorAgentCountRegistrationFailed ErrorType = "Extension.CountRegistrationFailed"
	ErrorAgentExtensionCreationFailed ErrorType = "Extension.CreationFailed"
	ErrorAgentRegistrationFailed      ErrorType = "Extension.RegistrationFailed"
	ErrorAgentReadyFailed             ErrorType = "Extension.ReadyFailed"
	ErrorAgentGateCreationFailed      ErrorType = "Extension.GateCreationFailed"

	ErrorReasonRuntimeExecFailed   ErrorType = "RuntimeExecFailure"
	ErrorRuntimeReadyFailed        ErrorType = "Runtime.ReadyFailed"
	ErrorRuntimeRegistrationFailed ErrorType = "Runtime.RegistrationFailed"

	ErrSandboxLogSocketsUnavailable ErrorType = "Sandbox.LogSocketsUnavailable"
	ErrSandboxEventSetupFailure     ErrorType = "Sandbox.EventSetupFailure"

	ErrSandboxShutdownFailed ErrorType = "Sandbox.ShutdownFailed"
)

type PlatformError struct {
	*appError
}

func NewPlatformError(cause error, errorType ErrorType) PlatformError {
	return PlatformError{
		appError: &appError{
			cause:     cause,
			severity:  ErrorSeverityFatal,
			source:    ErrorSourceSandbox,
			errorType: errorType,
			code:      http.StatusInternalServerError,
		},
	}
}

func WrapErrorIntoPlatformFatalError(e error, errorType ErrorType) PlatformError {

	if platformErr, ok := e.(PlatformError); ok {
		return platformErr
	}
	return NewPlatformError(
		e,
		errorType,
	)
}
