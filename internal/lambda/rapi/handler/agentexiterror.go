// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/fatalerror"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapi/rendering"

	log "github.com/sirupsen/logrus"
)

type agentExitErrorHandler struct {
	registrationService core.RegistrationService
}

func (h *agentExitErrorHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	agentID, ok := request.Context().Value(AgentIDCtxKey).(uuid.UUID)
	if !ok {
		rendering.RenderInternalServerError(writer, request)
		return
	}

	var errorType string
	if errorType = request.Header.Get(LambdaAgentFunctionErrorType); errorType == "" {
		log.Warnf("Invalid /extension/exit/error: missing %s header, agentID: %s", LambdaAgentFunctionErrorType, agentID)
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentMissingHeader, "%s not found", LambdaAgentFunctionErrorType)
		return
	}

	if externalAgent, found := h.registrationService.FindExternalAgentByID(agentID); found {
		if err := externalAgent.ExitError(errorType); err != nil {
			log.Warnf("Failed to transition agent %s to ExitError state: %s, current state %s", externalAgent.String(), err, externalAgent.GetState().Name())
			rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, StateTransitionFailedForExtensionMessageFormat,
				externalAgent.GetState().Name(), core.AgentExitedStateName, agentID.String(), err)
			return
		}
	} else if internalAgent, found := h.registrationService.FindInternalAgentByID(agentID); found {
		if err := internalAgent.ExitError(errorType); err != nil {
			log.Warnf("Failed to transition agent %s to ExitError state: %s, current state %s", internalAgent.String(), err, internalAgent.GetState().Name())
			rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, StateTransitionFailedForExtensionMessageFormat,
				internalAgent.GetState().Name(), core.AgentExitedStateName, agentID.String(), err)
			return
		}
	} else {
		log.Warnf("Unknown agent %s tried to call /extension/exit/error", agentID)
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentIdentifierUnknown, "Unknown "+LambdaAgentIdentifier)
		return
	}

	appctx.StoreFirstFatalError(appctx.FromRequest(request), fatalerror.AgentExitError)
	rendering.RenderAccepted(writer, request)
}

// NewAgentExitErrorHandler returns a new instance of http handler for serving /extension/exit/error
func NewAgentExitErrorHandler(registrationService core.RegistrationService) http.Handler {
	return &agentExitErrorHandler{
		registrationService: registrationService,
	}
}
