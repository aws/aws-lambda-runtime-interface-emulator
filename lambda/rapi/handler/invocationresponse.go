// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"net/http"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi/rendering"

	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"
)

type invocationResponseHandler struct {
	registrationService core.RegistrationService
}

func (h *invocationResponseHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	appCtx := appctx.FromRequest(request)

	server := appctx.LoadInteropServer(appCtx)
	if server == nil {
		log.Panic("Invalid state, cannot access interop server")
	}

	runtime := h.registrationService.GetRuntime()
	if err := runtime.InvocationResponse(); err != nil {
		log.Warn(err)
		rendering.RenderForbiddenWithTypeMsg(writer, request, rendering.ErrorTypeInvalidStateTransition, StateTransitionFailedForRuntimeMessageFormat,
			runtime.GetState().Name(), core.RuntimeInvocationResponseStateName, err)
		return
	}

	invokeID := chi.URLParam(request, "awsrequestid")

	if err := server.SendResponse(invokeID, request.Body); err != nil {
		switch err := err.(type) {
		case *interop.ErrorResponseTooLarge:
			if server.SendErrorResponse(invokeID, err.AsInteropError()) != nil {
				rendering.RenderInteropError(writer, request, err)
				return
			}

			appctx.StoreErrorResponse(appCtx, err.AsInteropError())

			if err := runtime.ResponseSent(); err != nil {
				log.Panic(err)
			}

			rendering.RenderRequestEntityTooLarge(writer, request)
			return
		default:
			rendering.RenderInteropError(writer, request, err)
			return
		}
	}

	if err := runtime.ResponseSent(); err != nil {
		log.Panic(err)
	}

	rendering.RenderAccepted(writer, request)
}

// NewInvocationResponseHandler returns a new instance of http handler
// for serving /runtime/invocation/{awsrequestid}/response.
func NewInvocationResponseHandler(registrationService core.RegistrationService) http.Handler {
	return &invocationResponseHandler{
		registrationService: registrationService,
	}
}
