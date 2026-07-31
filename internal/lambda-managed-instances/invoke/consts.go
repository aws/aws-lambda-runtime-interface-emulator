// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

const (
	СontentTypeHeader        = "content-type"
	FunctionErrorTypeTrailer = "lambda-runtime-function-error-type"
	FunctionErrorBodyTrailer = "lambda-runtime-function-error-body"
	ResponseModeHeader       = "invoke-response-mode"
	TraceIdHeader            = "x-amzn-trace-id"

	RuntimeInvocationIdHeader = "lambda-runtime-invocation-id"
)

type InvokeBodyResponseStatus string

const (
	InvokeBodyResponseComplete  InvokeBodyResponseStatus = "Complete"
	InvokeBodyResponseTruncated InvokeBodyResponseStatus = "Truncated"
	invokeBodyResponseOversized InvokeBodyResponseStatus = "Oversized"
	invokeBodyResponseTimeout   InvokeBodyResponseStatus = "Timeout"
)
