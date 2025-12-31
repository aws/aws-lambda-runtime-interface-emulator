// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

const (
	RuntimeResponseModeHeader    = "Lambda-Runtime-Function-Response-Mode"
	runtimeResponseModeStreaming = "streaming"
	runtimeResponseModeBuffered  = "buffered"
)

type runtimeResponse struct {
	request *http.Request

	parsingErr model.AppError

	contentType  string
	invokeID     interop.InvokeID
	responseMode string
}

func NewRuntimeResponse(ctx context.Context, request *http.Request, invokeID interop.InvokeID) runtimeResponse {
	contentType := request.Header.Get(RuntimeContentTypeHeader)
	if contentType == "" {

		contentType = "application/octet-stream"
	}
	resp := runtimeResponse{
		request:     request,
		contentType: contentType,
		invokeID:    invokeID,
	}

	switch mode := request.Header.Get(RuntimeResponseModeHeader); mode {
	case runtimeResponseModeStreaming:
		resp.responseMode = runtimeResponseModeStreaming
	case "":
		resp.responseMode = runtimeResponseModeBuffered
	default:
		logging.Error(ctx, "invalid response mode from runtime", "mode", mode)
		resp.responseMode = ""

		resp.parsingErr = model.NewCustomerError(model.ErrorRuntimeInvalidResponseModeHeader)
	}

	return resp
}

func (r *runtimeResponse) ParsingError() model.AppError {
	return r.parsingErr
}

func (r *runtimeResponse) InvokeID() interop.InvokeID {
	return r.invokeID
}

func (r *runtimeResponse) ContentType() string {
	return r.contentType
}

func (r *runtimeResponse) BodyReader() io.Reader {
	return r.request.Body
}

func (r *runtimeResponse) ResponseMode() string {
	return r.responseMode
}

func (r *runtimeResponse) TrailerError() ErrorForInvoker {
	typ := r.request.Trailer.Get(FunctionErrorTypeTrailer)
	if typ == "" {
		return nil
	}

	te := trailerError{
		typ:     model.GetValidRuntimeOrFunctionErrorType(typ),
		details: "",
	}

	base64EncodedBody := r.request.Trailer.Get(FunctionErrorBodyTrailer)
	decoded, err := base64.StdEncoding.DecodeString(base64EncodedBody)
	if err != nil {
		slog.Warn("could not base64 decode lambda-runtime-function-error-body trailer", "err", err)
		return te
	}

	te.details = string(decoded)
	return te
}

type trailerError struct {
	typ     model.ErrorType
	details string
}

func (t trailerError) ReturnCode() int {
	return http.StatusOK
}

func (t trailerError) ErrorCategory() model.ErrorCategory {
	return RuntimeErrorCategory
}

func (t trailerError) ErrorType() model.ErrorType {
	return t.typ
}

func (t trailerError) ErrorDetails() string {
	return t.details
}
