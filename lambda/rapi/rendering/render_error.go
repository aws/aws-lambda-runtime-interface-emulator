// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rendering

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi/model"
)

// RenderForbiddenWithTypeMsg method for rendering error response
func RenderForbiddenWithTypeMsg(w http.ResponseWriter, r *http.Request, errorType string, format string, args ...interface{}) {
	if err := RenderJSON(http.StatusForbidden, w, r, &model.ErrorResponse{
		ErrorType:    errorType,
		ErrorMessage: fmt.Sprintf(format, args...),
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderInternalServerError method for rendering error response
func RenderInternalServerError(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusInternalServerError, w, r, &model.ErrorResponse{
		ErrorMessage: "Internal Server Error",
		ErrorType:    ErrorTypeInternalServerError,
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderRequestEntityTooLarge method for rendering error response
func RenderRequestEntityTooLarge(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusRequestEntityTooLarge, w, r, &model.ErrorResponse{
		ErrorMessage: fmt.Sprintf("Exceeded maximum allowed payload size (%d bytes).", interop.MaxPayloadSize),
		ErrorType:    ErrorTypeRequestEntityTooLarge,
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderTruncatedHTTPRequestError method for rendering error response
func RenderTruncatedHTTPRequestError(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusBadRequest, w, r, &model.ErrorResponse{
		ErrorMessage: "HTTP request detected as truncated",
		ErrorType:    ErrorTypeTruncatedHTTPRequest,
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderInvalidRequestID renders invalid request ID error response
func RenderInvalidRequestID(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusBadRequest, w, r, &model.ErrorResponse{
		ErrorMessage: "Invalid request ID",
		ErrorType:    "InvalidRequestID",
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderInvalidFunctionResponseMode renders invalid function response mode response
func RenderInvalidFunctionResponseMode(w http.ResponseWriter, r *http.Request) {
	if err := RenderJSON(http.StatusBadRequest, w, r, &model.ErrorResponse{
		ErrorMessage: "Invalid function response mode",
		ErrorType:    "InvalidFunctionResponseMode",
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderInteropError is a convenience method for interpreting interop errors
func RenderInteropError(writer http.ResponseWriter, request *http.Request, err error) {
	if err == interop.ErrInvalidInvokeID || err == interop.ErrResponseSent {
		RenderInvalidRequestID(writer, request)
	} else {
		log.Panic(err)
	}
}
