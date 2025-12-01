// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"net/http"
)

const (
	ErrorAgentPermissionDenied  ErrorType = "PermissionDenied"
	ErrorAgentExtensionLaunch   ErrorType = "Extension.LaunchError"
	ErrorAgentTooManyExtensions ErrorType = "TooManyExtensions"
	ErrorAgentInit              ErrorType = "Extension.InitError"
	ErrorAgentExit              ErrorType = "Extension.ExitError"
	ErrorAgentCrash             ErrorType = "Extension.Crash"
	ErrorAgentUnknown           ErrorType = "Unknown"

	ErrorRuntimeInit                      ErrorType = "Runtime.InitError"
	ErrorRuntimeInvalidWorkingDir         ErrorType = "Runtime.InvalidWorkingDir"
	ErrorRuntimeInvalidEntryPoint         ErrorType = "Runtime.InvalidEntrypoint"
	ErrorRuntimeExit                      ErrorType = "Runtime.ExitError"
	ErrorRuntimeUnknown                   ErrorType = "Runtime.Unknown"
	ErrorRuntimeOutOfMemory               ErrorType = "Runtime.OutOfMemory"
	ErrorRuntimeTruncatedResponse         ErrorType = "Runtime.TruncatedResponse"
	ErrorRuntimeInvalidResponseModeHeader ErrorType = "Runtime.InvalidResponseModeHeader"

	ErrorRuntimeInvokeResponseInProgress ErrorType = "Runtime.InvokeResponseInProgress"
	ErrorRuntimeInvokeErrorInProgress    ErrorType = "Runtime.ErrorResponseInProgress"
	ErrorRuntimeInvokeResponseWasSent    ErrorType = "Runtime.InvokeResponseWasSent"
	ErrorRuntimeInvalidInvokeId          ErrorType = "Runtime.InvalidInvokeId"
	ErrorRuntimeInvokeTimeout            ErrorType = "Runtime.InvokeTimeout"
	ErrorRuntimeTooManyIdleRuntimes      ErrorType = "Runtime.TooManyIdleRuntimes"

	ErrorSandboxTimedout               ErrorType = "Sandbox.Timedout"
	ErrorSandboxFailure                ErrorType = "Sandbox.Failure"
	ErrorSandboxTimeoutResponseTrailer ErrorType = "Sandbox.TimeoutResponseTrailer"

	ErrorFunctionOversizedResponse ErrorType = "Function.ResponseSizeTooLarge"
	ErrorFunctionUnknown           ErrorType = "Function.Unknown"
)

type CustomerError struct {
	*appError
}

type ErrorOption func(err *appError)

func WithErrorMessage(msg string) ErrorOption {
	return func(err *appError) {
		err.errorMessage = msg
	}
}

func WithSeverity(sev ErrorSeverity) ErrorOption {
	return func(err *appError) {
		err.severity = sev
	}
}

func WithCause(cause error) ErrorOption {
	return func(err *appError) {
		err.cause = cause
	}
}

func NewCustomerError(errorType ErrorType, opts ...ErrorOption) CustomerError {
	err := appError{
		source:    ErrorSourceRuntime,
		severity:  ErrorSeverityError,
		errorType: errorType,
		code:      http.StatusOK,
	}
	for _, option := range opts {
		option(&err)
	}
	return CustomerError{&err}
}

func WrapErrorIntoCustomerInvalidError(e error, errorType ErrorType) CustomerError {

	if err, ok := e.(CustomerError); ok {
		return err
	}
	return NewCustomerError(errorType, WithCause(e), WithSeverity(ErrorSeverityInvalid))
}

func WrapErrorIntoCustomerFatalError(e error, errorType ErrorType) CustomerError {

	if err, ok := e.(CustomerError); ok {
		return err
	}
	return NewCustomerError(errorType, WithCause(e), WithSeverity(ErrorSeverityFatal))
}
