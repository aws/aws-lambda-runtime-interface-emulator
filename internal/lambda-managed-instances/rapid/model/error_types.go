// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils/invariant"
)

type ErrorSeverity string

const (
	ErrorSeverityError   ErrorSeverity = "Error"
	ErrorSeverityFatal   ErrorSeverity = "Fatal"
	ErrorSeverityInvalid ErrorSeverity = "Invalid"
)

type ErrorSource string

const (
	ErrorSourceClient  ErrorSource = "Client"
	ErrorSourceSandbox ErrorSource = "Sandbox"
	ErrorSourceRuntime ErrorSource = "Runtime"
)

type (
	ErrorType     string
	ErrorCategory string
)

const (
	ErrorReasonUnknownError    ErrorType     = "UnknownError"
	ErrorCategoryReasonUnknown ErrorCategory = "Fatal.Sandbox"
	ErrorCategoryClientCaused  ErrorCategory = "Invalid.Client"
)

func (e ErrorType) String() string {
	return string(e)
}

func (e ErrorCategory) String() string {
	return string(e)
}

type AppError interface {
	error

	Severity() ErrorSeverity

	Source() ErrorSource

	ErrorCategory() ErrorCategory

	ErrorType() ErrorType

	Unwrap() error

	ReturnCode() int

	ErrorDetails() string
}

type appError struct {
	cause     error
	severity  ErrorSeverity
	source    ErrorSource
	errorType ErrorType
	code      int

	errorMessage string
}

func (e *appError) Severity() ErrorSeverity {
	return e.severity
}

func (e *appError) Source() ErrorSource {
	return e.source
}

func (e *appError) ErrorType() ErrorType {
	return e.errorType
}

func (e *appError) ErrorDetails() string {
	errorDetails, err := json.Marshal(FunctionError{
		Type:    e.errorType,
		Message: e.errorMessage,
	})
	invariant.Checkf(err == nil, "could not json marshal error details: %s", err)
	return string(errorDetails)
}

func (e *appError) Unwrap() error {
	return e.cause
}

func (e *appError) Error() string {
	if e.cause == nil {
		return string(e.errorType)
	}
	return string(e.errorType) + ": " + e.cause.Error()
}

func (e *appError) ErrorCategory() ErrorCategory {
	return ErrorCategory(fmt.Sprintf("%s.%s", e.Severity(), e.Source()))
}

func (e *appError) ReturnCode() int {
	return e.code
}

func GetValidRuntimeOrFunctionErrorType(errorType string) ErrorType {
	match, _ := regexp.MatchString("(Runtime|Function)\\.[A-Z][a-zA-Z]+", errorType)
	if match {
		return ErrorType(errorType)
	}

	if strings.HasPrefix(errorType, "Function.") {
		return ErrorFunctionUnknown
	}

	return ErrorRuntimeUnknown
}

func GetValidExtensionErrorType(errorType string, defaultErrorType ErrorType) ErrorType {
	match, _ := regexp.MatchString("Extension\\.[A-Z][a-zA-Z]+", errorType)
	if match {
		return ErrorType(errorType)
	}

	return defaultErrorType
}

type FunctionError struct {
	Type ErrorType `json:"errorType,omitempty"`

	Message string `json:"errorMessage,omitempty"`
}
