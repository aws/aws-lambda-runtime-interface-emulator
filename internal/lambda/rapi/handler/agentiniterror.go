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

type agentInitErrorHandler struct {
	registrationService core.RegistrationService
}

func (h *agentInitErrorHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	agentID, ok := request.Context().Value(AgentIDCtxKey).(uuid.UUID)
	if !ok {
		rendering.RenderInternalServerError(writer, request)
		return
	}

	var errorType string
	if errorType = request.Header.Get(LambdaAgentFunctionErrorType); errorType == "" {
		log.Warnf("Invalid /extension/init/error: missing %s header, agentID: %s", LambdaAgentFunctionErrorType, agentID)
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentMissingHeader, "%s not found", LambdaAgentFunctionErrorType)
		return
	}

	if externalAgent, found := h.registrationService.FindExternalAgentByID(agentID); found {
		if err := externalAgent.InitError(errorType); err != nil {
			log.Warnf("InitError() failed for %s: %s, state is %s", externalAgent.String(), err, externalAgent.GetState().Name())
			rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, StateTransitionFailedForExtensionMessageFormat,
				externalAgent.GetState().Name(), core.AgentInitErrorStateName, agentID.String(), err)
			return
		}
	} else if internalAgent, found := h.registrationService.FindInternalAgentByID(agentID); found {
		if err := internalAgent.InitError(errorType); err != nil {
			log.Warnf("InitError() failed for %s: %s, state is %s", internalAgent.String(), err, internalAgent.GetState().Name())
			rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, StateTransitionFailedForExtensionMessageFormat,
				internalAgent.GetState().Name(), core.AgentInitErrorStateName, agentID.String(), err)
			return
		}
	} else {
		log.Warnf("Unknown agent %s tried to call /extension/init/error", LambdaAgentIdentifier)
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentIdentifierUnknown, "Unknown "+LambdaAgentIdentifier)
		return
	}

	appctx.StoreFirstFatalError(appctx.FromRequest(request), fatalerror.AgentInitError)
	rendering.RenderAccepted(writer, request)
}

// NewAgentInitErrorHandler returns a new instance of http handler for serving /extension/init/error
func NewAgentInitErrorHandler(registrationService core.RegistrationService) http.Handler {
	return &agentInitErrorHandler{
		registrationService: registrationService,
	}
}
