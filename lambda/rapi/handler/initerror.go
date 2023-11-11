// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"

	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/rapi/rendering"

	log "github.com/sirupsen/logrus"
)

type initErrorHandler struct {
	registrationService core.RegistrationService
}

func (h *initErrorHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	appCtx := appctx.FromRequest(request)
	interopServer := appctx.LoadInteropServer(appCtx)
	if interopServer == nil {
		log.Panic("Invalid state, cannot access interop server")
	}

	errorType := fatalerror.GetValidRuntimeOrFunctionErrorType(request.Header.Get("Lambda-Runtime-Function-Error-Type"))
	fnError := interop.FunctionError{Type: errorType}
	errorBody, err := io.ReadAll(request.Body)
	if err != nil {
		log.WithError(err).Warn("Failed to read error body")
	}
	headers := interop.InvokeResponseHeaders{ContentType: determineJSONContentType(errorBody)}
	response := &interop.ErrorInvokeResponse{Headers: headers, FunctionError: fnError, Payload: errorBody}

	runtime := h.registrationService.GetRuntime()

	// remove once Languages team change the endpoint to /restore/error
	// when an exception is throw while executing the restore hooks
	if runtime.GetState() == runtime.RuntimeRestoringState {
		if err := runtime.RestoreError(fnError); err != nil {
			log.Warn(err)
			rendering.RenderForbiddenWithTypeMsg(writer, request, rendering.ErrorTypeInvalidStateTransition, StateTransitionFailedForRuntimeMessageFormat,
				runtime.GetState().Name(), core.RuntimeRestoreErrorStateName, err)
			return
		}

		appctx.StoreInvokeErrorTraceData(appCtx, &interop.InvokeErrorTraceData{})
		rendering.RenderAccepted(writer, request)
		return
	}

	if err := runtime.InitError(); err != nil {
		log.Warn(err)
		rendering.RenderForbiddenWithTypeMsg(writer, request, rendering.ErrorTypeInvalidStateTransition, StateTransitionFailedForRuntimeMessageFormat,
			runtime.GetState().Name(), core.RuntimeInitErrorStateName, err)
		return
	}

	if err := interopServer.SendInitErrorResponse(response); err != nil {
		rendering.RenderInteropError(writer, request, err)
		return
	}

	appctx.StoreInvokeErrorTraceData(appCtx, &interop.InvokeErrorTraceData{})
	rendering.RenderAccepted(writer, request)
}

// NewInitErrorHandler returns a new instance of http handler
// for serving /runtime/init/error.
func NewInitErrorHandler(registrationService core.RegistrationService) http.Handler {
	return &initErrorHandler{registrationService: registrationService}
}

func determineJSONContentType(body []byte) string {
	if json.Valid(body) {
		return "application/json"
	}
	return "application/octet-stream"
}
