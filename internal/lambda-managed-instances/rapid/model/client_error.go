// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import "net/http"

const (
	ErrorInvalidRequest                    ErrorType = "InvalidRequest"
	ErrorMalformedRequest                  ErrorType = "ErrMalformedRequest"
	ErrorInitIncomplete                    ErrorType = "Client.InitIncomplete"
	ErrorEnvironmentUnhealthy              ErrorType = "Client.ExecutionEnvironmentUnhealthy"
	ErrorRuntimeUnavailable                ErrorType = "Runtime.Unavailable"
	ErrorDublicatedInvokeId                ErrorType = "Client.DuplicatedInvokeId"
	ErrorInvalidFunctionVersion            ErrorType = "ErrInvalidFunctionVersion"
	ErrorInvalidMaxPayloadSize             ErrorType = "ErrInvalidMaxPayloadSize"
	ErrorInvalidResponseBandwidthRate      ErrorType = "ErrInvalidResponseBandwidthRate"
	ErrorInvalidResponseBandwidthBurstSize ErrorType = "ErrInvalidResponseBandwidthBurstSize"
	ErrorExecutionEnvironmentShutdown      ErrorType = "Client.ExecutionEnvironmentShutDown"
)

type ClientError struct {
	*appError
}

func NewClientError(cause error, severity ErrorSeverity, errorType ErrorType) ClientError {
	return ClientError{
		appError: &appError{
			cause:     cause,
			severity:  severity,
			source:    ErrorSourceClient,
			errorType: errorType,
			code:      http.StatusBadRequest,
		},
	}
}
