// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"io/ioutil"
	"net/http"

	"go.amzn.com/lambda/appctx"
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

	server := appctx.LoadInteropServer(appCtx)
	if server == nil {
		log.Panic("Invalid state, cannot access interop server")
	}

	runtime := h.registrationService.GetRuntime()
	if err := runtime.InitError(); err != nil {
		log.Warn(err)
		rendering.RenderForbiddenWithTypeMsg(writer, request, rendering.ErrorTypeInvalidStateTransition, StateTransitionFailedForRuntimeMessageFormat,
			runtime.GetState().Name(), core.RuntimeInitErrorStateName, err)
		return
	}

	errorType := request.Header.Get("Lambda-Runtime-Function-Error-Type")

	errorBody, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.WithError(err).Warn("Failed to read error body")
	}

	response := &interop.ErrorResponse{
		ErrorType: errorType,
		Payload:   errorBody,
	}

	if err := server.SendErrorResponse(server.GetCurrentInvokeID(), response); err != nil {
		rendering.RenderInteropError(writer, request, err)
		return
	}

	appctx.StoreErrorResponse(appCtx, response)

	rendering.RenderAccepted(writer, request)
}

// NewInitErrorHandler returns a new instance of http handler
// for serving /runtime/init/error.
func NewInitErrorHandler(registrationService core.RegistrationService) http.Handler {
	return &initErrorHandler{
		registrationService: registrationService,
	}
}
