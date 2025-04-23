// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"net/http"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/rapi/rendering"
)

// A CtxKey type is used as a key for storing values in the request context.
type CtxKey int

// AgentIDCtxKey is the context key for fetching agent's UUID
const (
	AgentIDCtxKey CtxKey = iota
)

type agentNextHandler struct {
	registrationService core.RegistrationService
	renderingService    *rendering.EventRenderingService
}

func (h *agentNextHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	agentID, ok := request.Context().Value(AgentIDCtxKey).(uuid.UUID)
	if !ok {
		rendering.RenderInternalServerError(writer, request)
		return
	}

	if externalAgent, found := h.registrationService.FindExternalAgentByID(agentID); found {
		if err := externalAgent.Ready(); err != nil {
			log.Warnf("Ready() failed for %s: %s, state is %s", externalAgent.String(), err, externalAgent.GetState().Name())
			rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, "State transition from %s to %s failed for extension %s. Error: %s",
				externalAgent.GetState().Name(), core.AgentReadyStateName, agentID.String(), err)
			return
		}
	} else if internalAgent, found := h.registrationService.FindInternalAgentByID(agentID); found {
		if err := internalAgent.Ready(); err != nil {
			log.Warnf("Ready() failed for %s: %s, state is %s", internalAgent.String(), err, internalAgent.GetState().Name())
			rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, "State transition from %s to %s failed for extension %s. Error: %s",
				internalAgent.GetState().Name(), core.AgentReadyStateName, agentID.String(), err)
			return
		}
	} else {
		log.Warnf("Unknown agent %s tried to call /next", agentID.String())
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentIdentifierUnknown, "Unknown extension %s", agentID.String())
		return
	}

	if err := h.renderingService.RenderAgentEvent(writer, request); err != nil {
		log.Error(err)
		rendering.RenderInternalServerError(writer, request)
		return
	}
}

// NewAgentNextHandler returns a new instance of http handler for serving /extension/event/next
func NewAgentNextHandler(registrationService core.RegistrationService, renderingService *rendering.EventRenderingService) http.Handler {
	return &agentNextHandler{
		registrationService: registrationService,
		renderingService:    renderingService,
	}
}
