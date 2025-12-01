// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapi/rendering"
)

type restoreNextHandler struct {
	registrationService core.RegistrationService
	renderingService    *rendering.EventRenderingService
}

func (h *restoreNextHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	runtime := h.registrationService.GetRuntime()
	err := runtime.RestoreReady()
	if err != nil {
		log.Warn(err)
		rendering.RenderForbiddenWithTypeMsg(writer, request, rendering.ErrorTypeInvalidStateTransition, StateTransitionFailedForRuntimeMessageFormat, runtime.GetState().Name(), core.RuntimeReadyStateName, err)
		return
	}
	err = h.renderingService.RenderRuntimeEvent(writer, request)
	if err != nil {
		log.Error(err)
		rendering.RenderInternalServerError(writer, request)
		return
	}
}

func NewRestoreNextHandler(registrationService core.RegistrationService, renderingService *rendering.EventRenderingService) http.Handler {
	return &restoreNextHandler{
		registrationService: registrationService,
		renderingService:    renderingService,
	}
}
