// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
)

type agentNextHandler struct {
	registrationService core.RegistrationService
	renderingService    *rendering.EventRenderingService
}

func (h *agentNextHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	agentID, ok := request.Context().Value(model.AgentIDCtxKey).(uuid.UUID)
	if !ok {
		rendering.RenderInternalServerError(writer, request)
		return
	}

	ctx := logging.WithFields(request.Context(), "agentID", agentID.String())
	logging.Debug(ctx, "Received Extension /next")

	if externalAgent, found := h.registrationService.FindExternalAgentByID(agentID); found {
		ctx = logging.WithFields(ctx, "agent", externalAgent.Name())
		if err := externalAgent.Ready(); err != nil {
			logging.Warn(ctx, "Extension ready failed", "err", err, "state", externalAgent.GetState().Name())
			rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, StateTransitionFailedForExtensionMessageFormat,
				externalAgent.GetState().Name(), core.AgentReadyStateName, agentID.String(), err)
			return
		}
	} else if internalAgent, found := h.registrationService.FindInternalAgentByID(agentID); found {
		ctx = logging.WithFields(ctx, "agent", internalAgent.Name())
		if err := internalAgent.Ready(); err != nil {
			logging.Warn(ctx, "Extension ready failed", "err", err, "state", internalAgent.GetState().Name())
			rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, StateTransitionFailedForExtensionMessageFormat,
				internalAgent.GetState().Name(), core.AgentReadyStateName, agentID.String(), err)
			return
		}
	} else {
		logging.Warn(ctx, "Unknown extension /next request")
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentIdentifierUnknown, "Unknown extension %s", agentID.String())
		return
	}

	if err := h.renderingService.RenderAgentEvent(writer, request); err != nil {
		logging.Error(ctx, "Render agent event failed", "err", err)
		rendering.RenderInternalServerError(writer, request)
		return
	}
}

func NewAgentNextHandler(registrationService core.RegistrationService, renderingService *rendering.EventRenderingService) http.Handler {
	return &agentNextHandler{
		registrationService: registrationService,
		renderingService:    renderingService,
	}
}
