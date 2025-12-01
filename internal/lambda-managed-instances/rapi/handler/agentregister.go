// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
)

type agentRegisterHandler struct {
	registrationService core.RegistrationService
}

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
	ctx := logging.WithFields(request.Context(), "agentName", agentName)
	if agentName == "" {
		logging.Warn(ctx, "Empty extension name")
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
		logging.Warn(ctx, "Invalid Register request format", "err", err)
		rendering.RenderForbiddenWithTypeMsg(writer, request, errInvalidRequestFormat, "%s", err.Error())
		return
	}

	agent, found := h.registrationService.FindExternalAgentByName(agentName)
	if found {
		h.registerExternalAgent(ctx, agent, registerRequest, writer, request, responseModifiers...)
	} else {
		h.registerInternalAgent(ctx, agentName, registerRequest, writer, request, responseModifiers...)
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
	writer.Header().Set(model.LambdaAgentIdentifier, agentID)

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
		slog.Warn("Error while rendering response", "err", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func (h *agentRegisterHandler) registerExternalAgent(
	ctx context.Context,
	agent *core.ExternalAgent,
	registerRequest *RegisterRequest,
	writer http.ResponseWriter,
	request *http.Request,
	respModifiers ...responseModifier,
) {
	ctx = logging.WithFields(ctx, "agent", agent.String())
	for _, e := range registerRequest.Events {
		if err := core.ValidateExternalAgentEvent(e); err != nil {
			logging.Warn(ctx, "Failed to register agent event", "event", e, "err", err)
			rendering.RenderForbiddenWithTypeMsg(writer, request, errInvalidEventType, "%s: %s", e, err)
			return
		}
	}

	if err := agent.Register(registerRequest.Events); err != nil {
		logging.Warn(ctx, "Failed to register agent", "err", err)
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, StateTransitionFailedForExtensionMessageFormat,
			agent.GetState().Name(), core.AgentRegisteredStateName, agent.Name(), err)
		return
	}

	h.renderResponse(agent.ID().String(), writer, request, respModifiers...)
	logging.Debug(ctx, "External agent registered", "events", registerRequest.Events)
}

func (h *agentRegisterHandler) registerInternalAgent(
	ctx context.Context,
	agentName string,
	registerRequest *RegisterRequest,
	writer http.ResponseWriter,
	request *http.Request,
	respModifiers ...responseModifier,
) {
	if len(registerRequest.Events) != 0 {
		logging.Warn(ctx, "No events allowed for internal extensions")
		rendering.RenderForbiddenWithTypeMsg(writer, request, errInvalidEventType, "No events allowed for internal extensions")
		return
	}

	agent, err := h.registrationService.CreateInternalAgent(agentName)
	if err != nil {
		logging.Warn(ctx, "Failed to create internal agent", "err", err)

		switch err {
		case core.ErrRegistrationServiceOff:
			logging.Warn(ctx, "Extension registration closed already")
			rendering.RenderForbiddenWithTypeMsg(writer, request,
				errAgentRegistrationClosed, "Extension registration closed already")
		case core.ErrAgentNameCollision:
			logging.Warn(ctx, "Extension with this name already registered")
			rendering.RenderForbiddenWithTypeMsg(writer, request,
				errAgentInvalidState, "Extension with this name already registered")
		case core.ErrTooManyExtensions:
			logging.Warn(ctx, "Extension limit reached", "limit", core.MaxAgentsAllowed)
			rendering.RenderForbiddenWithTypeMsg(writer, request,
				errTooManyExtensions, "Extension limit (%d) reached", core.MaxAgentsAllowed)
		default:
			rendering.RenderInternalServerError(writer, request)
		}

		return
	}

	ctx = logging.WithFields(ctx, "agent", agent.String())

	if err := agent.Register(registerRequest.Events); err != nil {
		logging.Warn(ctx, "Failed to register agent", "err", err)
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, StateTransitionFailedForExtensionMessageFormat,
			agent.GetState().Name(), core.AgentRegisteredStateName, agent.Name(), err)
		return
	}

	h.renderResponse(agent.ID().String(), writer, request, respModifiers...)
	logging.Info(ctx, "Internal agent registered")
}

func NewAgentRegisterHandler(registrationService core.RegistrationService) http.Handler {
	return &agentRegisterHandler{
		registrationService: registrationService,
	}
}
