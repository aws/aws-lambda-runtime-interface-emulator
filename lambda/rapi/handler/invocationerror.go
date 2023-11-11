// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi/model"

	"go.amzn.com/lambda/appctx"

	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/rapi/rendering"

	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"
)

const errorWithCauseContentType = "application/vnd.aws.lambda.error.cause+json"
const xrayErrorCauseHeaderName = "Lambda-Runtime-Function-XRay-Error-Cause"
const invalidErrorBodyMessage = "Invalid error body"

const (
	contentTypeHeader          = "Content-Type"
	functionResponseModeHeader = "Lambda-Runtime-Function-Response-Mode"
)

type invocationErrorHandler struct {
	registrationService core.RegistrationService
}

func (h *invocationErrorHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	appCtx := appctx.FromRequest(request)

	server := appctx.LoadResponseSender(appCtx)
	if server == nil {
		log.Panic("Invalid state, cannot access interop server")
	}

	runtime := h.registrationService.GetRuntime()
	if err := runtime.InvocationErrorResponse(); err != nil {
		log.Warn(err)
		rendering.RenderForbiddenWithTypeMsg(writer, request, rendering.ErrorTypeInvalidStateTransition, StateTransitionFailedForRuntimeMessageFormat,
			runtime.GetState().Name(), core.RuntimeInvocationErrorResponseStateName, err)
		return
	}

	errorType := fatalerror.GetValidRuntimeOrFunctionErrorType(h.getErrorType(request.Header))

	var errorCause json.RawMessage
	var errorBody []byte
	var contentType string
	var err error

	switch request.Header.Get(contentTypeHeader) {
	case errorWithCauseContentType:
		errorBody, errorCause, err = h.getErrorBodyForErrorCauseContentType(request)
		contentType = "application/json"
		if err != nil {
			contentType = "application/octet-stream"
		}
	default:
		errorBody, err = h.getErrorBody(request)
		errorCause = h.getValidatedErrorCause(request.Header)
		contentType = request.Header.Get(contentTypeHeader)
	}
	functionResponseMode := request.Header.Get(functionResponseModeHeader)

	if err != nil {
		log.WithError(err).Warn("Failed to parse error body")
	}

	headers := interop.InvokeResponseHeaders{
		ContentType:          contentType,
		FunctionResponseMode: functionResponseMode,
	}

	response := &interop.ErrorInvokeResponse{
		Headers:       headers,
		FunctionError: interop.FunctionError{Type: errorType},
		Payload:       errorBody,
	}

	if err := server.SendErrorResponse(chi.URLParam(request, "awsrequestid"), response); err != nil {
		rendering.RenderInteropError(writer, request, err)
		return
	}

	appctx.StoreInvokeErrorTraceData(appCtx, &interop.InvokeErrorTraceData{ErrorCause: errorCause})

	if err := runtime.ResponseSent(); err != nil {
		log.Panic(err)
	}

	rendering.RenderAccepted(writer, request)
}

func (h *invocationErrorHandler) getErrorType(headers http.Header) string {
	return headers.Get("Lambda-Runtime-Function-Error-Type")
}

func (h *invocationErrorHandler) getErrorBody(request *http.Request) ([]byte, error) {
	errorBody, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading request body: %s", err)
	}
	return errorBody, nil
}

func (h *invocationErrorHandler) getValidatedErrorCause(headers http.Header) json.RawMessage {
	errorCauseHeader := headers.Get(xrayErrorCauseHeaderName)
	if len(errorCauseHeader) == 0 {
		return nil
	}

	errorCauseJSON := json.RawMessage(errorCauseHeader)

	validErrorCauseJSON, err := model.ValidatedErrorCauseJSON(errorCauseJSON)
	if err != nil {
		log.WithError(err).Error("errorCause validation error")
		return nil
	}

	return validErrorCauseJSON
}

func (h *invocationErrorHandler) getErrorBodyForErrorCauseContentType(request *http.Request) ([]byte, json.RawMessage, error) {
	errorBody, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading request body: %s", err)
	}

	parsedError, err := newErrorWithCauseRequest(errorBody)
	if err != nil {
		errResponse, _ := json.Marshal(invalidErrorBodyMessage)
		return errResponse, nil, fmt.Errorf("error parsing request body: %s, request.Body: %s", err, errorBody)
	}

	filteredError, err := parsedError.getInvokeErrorResponse()

	return filteredError, parsedError.getValidatedXRayCause(), err
}

// NewInvocationErrorHandler returns a new instance of http handler
// for serving /runtime/invocation/{awsrequestid}/error.
func NewInvocationErrorHandler(registrationService core.RegistrationService) http.Handler {
	return &invocationErrorHandler{
		registrationService: registrationService,
	}
}
