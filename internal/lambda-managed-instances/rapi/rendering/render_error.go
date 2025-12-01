// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rendering

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
)

func RenderForbiddenWithTypeMsg(w http.ResponseWriter, r *http.Request, errorType string, format string, args ...interface{}) {
	if err := RenderJSON(http.StatusForbidden, w, r, &model.ErrorResponse{
		ErrorType:    errorType,
		ErrorMessage: fmt.Sprintf(format, args...),
	}); err != nil {
		slog.Warn("Error while rendering response", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func RenderInternalServerError(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusInternalServerError, w, r, &model.ErrorResponse{
		ErrorMessage: "Internal Server Error",
		ErrorType:    ErrorTypeInternalServerError,
	}); err != nil {
		slog.Warn("Error while rendering response", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func RenderRequestEntityTooLarge(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusRequestEntityTooLarge, w, r, &model.ErrorResponse{
		ErrorMessage: fmt.Sprintf("Exceeded maximum allowed payload size (%d bytes).", interop.MaxPayloadSize),
		ErrorType:    ErrorTypeRequestEntityTooLarge,
	}); err != nil {
		slog.Warn("Error while rendering response", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func RenderTruncatedHTTPRequestError(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusBadRequest, w, r, &model.ErrorResponse{
		ErrorMessage: "HTTP request detected as truncated",
		ErrorType:    ErrorTypeTruncatedHTTPRequest,
	}); err != nil {
		slog.Warn("Error while rendering response", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func RenderInvalidRequestID(w http.ResponseWriter, r *http.Request) {

	if err := RenderJSON(http.StatusBadRequest, w, r, &model.ErrorResponse{
		ErrorMessage: "Invalid request ID",
		ErrorType:    "InvalidRequestID",
	}); err != nil {
		slog.Warn("Error while rendering response", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func RenderInvokeTimeout(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusGone, w, r, &model.ErrorResponse{
		ErrorMessage: "Invoke timeout",
		ErrorType:    "InvokeTimeout",
	}); err != nil {
		slog.Warn("Error while rendering response", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func RenderInvalidFunctionResponseMode(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusBadRequest, w, r, &model.ErrorResponse{
		ErrorMessage: "Invalid function response mode",
		ErrorType:    "InvalidFunctionResponseMode",
	}); err != nil {
		slog.Warn("Error while rendering response", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func RenderInteropError(writer http.ResponseWriter, request *http.Request, err error) {
	if err == interop.ErrResponseSent {
		RenderInvalidRequestID(writer, request)
	} else {
		slog.Error("Interop error", "err", err)
		panic(err)
	}
}
