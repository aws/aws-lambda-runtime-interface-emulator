// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

const (
	RuntimeErrorTypeHeader     = "Lambda-Runtime-Function-Error-Type"
	RuntimeErrorCategory       = "Error.Runtime"
	LambdaXRayErrorCauseHeader = "Lambda-Runtime-Function-XRay-Error-Cause"
)

var runtimeDefaultSeverity = model.ErrorSeverityError

type runtimeError struct {
	request *http.Request

	contentType   string
	invokeID      interop.InvokeID
	errorType     model.ErrorType
	errorCategory model.ErrorCategory

	errorDetails   string
	xrayErrorCause json.RawMessage
}

func NewRuntimeError(ctx context.Context, request *http.Request, invokeID interop.InvokeID, errorDetails string) runtimeError {

	return runtimeError{
		request:        request,
		contentType:    request.Header.Get(RuntimeContentTypeHeader),
		invokeID:       invokeID,
		errorType:      model.GetValidRuntimeOrFunctionErrorType(request.Header.Get(RuntimeErrorTypeHeader)),
		errorCategory:  RuntimeErrorCategory,
		errorDetails:   errorDetails,
		xrayErrorCause: getValidatedErrorCause(ctx, request.Header.Get(LambdaXRayErrorCauseHeader)),
	}
}

func (r *runtimeError) InvokeID() interop.InvokeID {
	return r.invokeID
}

func (r *runtimeError) ContentType() string {
	return r.contentType
}

func (r *runtimeError) ErrorType() model.ErrorType {
	return r.errorType
}

func (r *runtimeError) ErrorCategory() model.ErrorCategory {
	return r.errorCategory
}

func (r *runtimeError) GetError() model.AppError {

	return model.NewCustomerError(r.errorType)
}

func (r *runtimeError) IsRuntimeError(err model.AppError) bool {

	if r.errorType != err.ErrorType() {
		return false
	}

	if err.Severity() != runtimeDefaultSeverity {
		return false
	}

	return true
}

func (r *runtimeError) ErrorDetails() string {
	return r.errorDetails
}

func (r *runtimeError) GetXrayErrorCause() json.RawMessage {
	return r.xrayErrorCause
}

func (r *runtimeError) ReturnCode() int {
	return http.StatusOK
}

func getValidatedErrorCause(ctx context.Context, errorCauseHeader string) json.RawMessage {
	if len(errorCauseHeader) == 0 {
		logging.Debug(ctx, "errorCause has not been set")
		return nil
	}
	errorCauseJSON := json.RawMessage(errorCauseHeader)

	validErrorCauseJSON, err := rapidmodel.ValidatedErrorCauseJSON(errorCauseJSON)
	if err != nil {
		logging.Warn(ctx, "errorCause JSON validation failed", "err", err)
		return nil
	}

	return validErrorCauseJSON
}
