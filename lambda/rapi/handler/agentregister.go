// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/rapi/model"
	"go.amzn.com/lambda/rapi/rendering"
)

type agentRegisterHandler struct {
	registrationService core.RegistrationService
}

// RegisterRequest represent /extension/register JSON body
type RegisterRequest struct {
	Events []core.Event `json:"events"`
}

func parseRegister(request *http.Request) (*RegisterRequest, error) {
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}

	req := struct {
		RegisterRequest
		ConfigurationKeys []string `json:"configurationKeys"`
	}{}

	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	if len(req.ConfigurationKeys) != 0 {
		return nil, errors.New("configurationKeys are deprecated; use environment variables instead")
	}

	return &req.RegisterRequest, nil
}

func (h *agentRegisterHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	agentName := request.Header.Get(LambdaAgentName)
	if agentName == "" {
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentNameInvalid, "Empty extension name")
		return
	}

	registerRequest, err := parseRegister(request)
	if err != nil {
		rendering.RenderForbiddenWithTypeMsg(writer, request, errInvalidRequestFormat, err.Error())
		return
	}

	agent, found := h.registrationService.FindExternalAgentByName(agentName)

	if found {
		h.registerExternalAgent(agent, registerRequest, writer, request)
	} else {
		h.registerInternalAgent(agentName, registerRequest, writer, request)
	}
}

func (h *agentRegisterHandler) renderResponse(agentID string, writer http.ResponseWriter, request *http.Request) {
	render.Status(request, http.StatusOK)
	writer.Header().Set(LambdaAgentIdentifier, agentID)

	metadata := h.registrationService.GetFunctionMetadata()

	resp := &model.ExtensionRegisterResponse{
		FunctionVersion: metadata.FunctionVersion,
		FunctionName:    metadata.FunctionName,
		Handler:         metadata.Handler,
	}

	render.JSON(writer, request, resp)
}

func (h *agentRegisterHandler) registerExternalAgent(agent *core.ExternalAgent, registerRequest *RegisterRequest, writer http.ResponseWriter, request *http.Request) {
	for _, e := range registerRequest.Events {
		if err := core.ValidateExternalAgentEvent(e); err != nil {
			log.Warnf("Failed to register %s: event %s: %s", agent.Name, e, err)
			rendering.RenderForbiddenWithTypeMsg(writer, request, errInvalidEventType, "%s: %s", e, err)
			return
		}
	}

	if err := agent.Register(registerRequest.Events); err != nil {
		log.Warnf("Failed to register %s: %s", agent.String(), err)
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, StateTransitionFailedForExtensionMessageFormat,
			agent.GetState().Name(), core.AgentRegisteredStateName, agent.Name, err)
		return
	}

	h.renderResponse(agent.ID.String(), writer, request)
	log.Infof("External agent %s registered, subscribed to %v", agent.String(), registerRequest.Events)
}

func (h *agentRegisterHandler) registerInternalAgent(agentName string, registerRequest *RegisterRequest, writer http.ResponseWriter, request *http.Request) {
	for _, e := range registerRequest.Events {
		if err := core.ValidateInternalAgentEvent(e); err != nil {
			log.Warnf("Failed to register %s: event %s: %s", agentName, e, err)
			rendering.RenderForbiddenWithTypeMsg(writer, request, errInvalidEventType, "%s: %s", e, err)
			return
		}
	}

	agent, err := h.registrationService.CreateInternalAgent(agentName)
	if err != nil {
		log.Warnf("Failed to create internal agent %s: %s", agentName, err)

		switch err {
		case core.ErrRegistrationServiceOff:
			rendering.RenderForbiddenWithTypeMsg(writer, request,
				errAgentRegistrationClosed, "Extension registration closed already")
		case core.ErrAgentNameCollision:
			rendering.RenderForbiddenWithTypeMsg(writer, request,
				errAgentInvalidState, "Extension with this name already registered")
		case core.ErrTooManyExtensions:
			rendering.RenderForbiddenWithTypeMsg(writer, request,
				errTooManyExtensions, "Extension limit (%d) reached", core.MaxAgentsAllowed)
		default:
			rendering.RenderInternalServerError(writer, request)
		}

		return
	}

	if err := agent.Register(registerRequest.Events); err != nil {
		log.Warnf("Failed to register %s: %s", agent.String(), err)
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, StateTransitionFailedForExtensionMessageFormat,
			agent.GetState().Name(), core.AgentRegisteredStateName, agent.Name, err)
		return
	}

	h.renderResponse(agent.ID.String(), writer, request)
	log.Infof("Internal agent %s registered, subscribed to %v", agent.String(), registerRequest.Events)
}

// NewAgentRegisterHandler returns a new instance of http handler for serving /extension/register
func NewAgentRegisterHandler(registrationService core.RegistrationService) http.Handler {
	return &agentRegisterHandler{
		registrationService: registrationService,
	}
}
