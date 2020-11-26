// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"net/http"

	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/rapi/rendering"

	log "github.com/sirupsen/logrus"
)

type invocationNextHandler struct {
	registrationService core.RegistrationService
	renderingService    *rendering.EventRenderingService
}

func (h *invocationNextHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	runtime := h.registrationService.GetRuntime()
	err := runtime.Ready()
	if err != nil {
		log.Warn(err)
		rendering.RenderForbiddenWithTypeMsg(writer, request, rendering.ErrorTypeInvalidStateTransition, StateTransitionFailedForRuntimeMessageFormat,
			runtime.GetState().Name(), core.RuntimeReadyStateName, err)
		return
	}
	err = h.renderingService.RenderRuntimeEvent(writer, request)
	if err != nil {
		log.Error(err)
		rendering.RenderInternalServerError(writer, request)
		return
	}
}

// NewInvocationNextHandler returns a new instance of http handler
// for serving /runtime/invocation/next.
func NewInvocationNextHandler(registrationService core.RegistrationService, renderingService *rendering.EventRenderingService) http.Handler {
	return &invocationNextHandler{
		registrationService: registrationService,
		renderingService:    renderingService,
	}
}
