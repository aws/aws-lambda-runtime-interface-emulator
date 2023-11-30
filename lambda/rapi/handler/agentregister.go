// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

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

const featuresHeader = "Lambda-Extension-Accept-Feature"

type registrationFeature int

const (
	accountFeature registrationFeature = iota + 1
)

var allowedFeatures = map[string]registrationFeature{
	"accountId": accountFeature,
}

type responseModifier func(*model.ExtensionRegisterResponse)

func parseRegister(request *http.Request) (*RegisterRequest, error) {
	body, err := io.ReadAll(request.Body)
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

	var responseModifiers []responseModifier
	for _, f := range parseRegistrationFeatures(request) {
		if f == accountFeature {
			responseModifiers = append(responseModifiers, h.respondWithAccountID())
		}
	}

	registerRequest, err := parseRegister(request)
	if err != nil {
		rendering.RenderForbiddenWithTypeMsg(writer, request, errInvalidRequestFormat, err.Error())
		return
	}

	agent, found := h.registrationService.FindExternalAgentByName(agentName)
	if found {
		h.registerExternalAgent(agent, registerRequest, writer, request, responseModifiers...)
	} else {
		h.registerInternalAgent(agentName, registerRequest, writer, request, responseModifiers...)
	}
}

func (h *agentRegisterHandler) respondWithAccountID() responseModifier {
	return func(resp *model.ExtensionRegisterResponse) {
		resp.AccountID = h.registrationService.GetFunctionMetadata().AccountID
	}
}

func parseRegistrationFeatures(request *http.Request) []registrationFeature {
	rawFeatures := strings.Split(request.Header.Get(featuresHeader), ",")

	var features []registrationFeature
	for _, feature := range rawFeatures {
		feature = strings.TrimSpace(feature)
		if v, found := allowedFeatures[feature]; found {
			features = append(features, v)
		}
	}

	return features
}

func (h *agentRegisterHandler) renderResponse(
	agentID string,
	writer http.ResponseWriter,
	request *http.Request,
	respModifiers ...responseModifier,
) {
	writer.Header().Set(LambdaAgentIdentifier, agentID)

	metadata := h.registrationService.GetFunctionMetadata()
	resp := &model.ExtensionRegisterResponse{
		FunctionVersion: metadata.FunctionVersion,
		FunctionName:    metadata.FunctionName,
		Handler:         metadata.Handler,
	}

	for _, mod := range respModifiers {
		mod(resp)
	}

	if err := rendering.RenderJSON(http.StatusOK, writer, request, resp); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func (h *agentRegisterHandler) registerExternalAgent(
	agent *core.ExternalAgent,
	registerRequest *RegisterRequest,
	writer http.ResponseWriter,
	request *http.Request,
	respModifiers ...responseModifier,
) {
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

	h.renderResponse(agent.ID.String(), writer, request, respModifiers...)
	log.Infof("External agent %s registered, subscribed to %v", agent.String(), registerRequest.Events)
}

func (h *agentRegisterHandler) registerInternalAgent(
	agentName string,
	registerRequest *RegisterRequest,
	writer http.ResponseWriter,
	request *http.Request,
	respModifiers ...responseModifier,
) {
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

	h.renderResponse(agent.ID.String(), writer, request, respModifiers...)
	log.Infof("Internal agent %s registered, subscribed to %v", agent.String(), registerRequest.Events)
}

// NewAgentRegisterHandler returns a new instance of http handler for serving /extension/register
func NewAgentRegisterHandler(registrationService core.RegistrationService) http.Handler {
	return &agentRegisterHandler{
		registrationService: registrationService,
	}
}
