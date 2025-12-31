// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type agentInitErrorHandler struct {
	registrationService core.RegistrationService
}

func (h *agentInitErrorHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	agentID, ok := request.Context().Value(model.AgentIDCtxKey).(uuid.UUID)
	if !ok {
		rendering.RenderInternalServerError(writer, request)
		return
	}
	ctx := logging.WithFields(request.Context(), "agentID", agentID.String())

	var rawErrorType string
	if rawErrorType = request.Header.Get(LambdaAgentFunctionErrorType); rawErrorType == "" {
		logging.Warn(ctx, "Invalid /extension/init/error: missing header", "header", LambdaAgentFunctionErrorType)
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentMissingHeader, "%s not found", LambdaAgentFunctionErrorType)
		return
	}

	errorType := rapidmodel.GetValidExtensionErrorType(rawErrorType, rapidmodel.ErrorAgentInit)
	logging.Warn(ctx, "Received extension Init error", "errorType", errorType)

	if externalAgent, found := h.registrationService.FindExternalAgentByID(agentID); found {
		if err := externalAgent.InitError(errorType); err != nil {
			logging.Warn(ctx, "InitError() failed for external agent", "agent", externalAgent.String(), "err", err, "state", externalAgent.GetState().Name())
			rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, StateTransitionFailedForExtensionMessageFormat,
				externalAgent.GetState().Name(), core.AgentInitErrorStateName, agentID.String(), err)
			return
		}
	} else if internalAgent, found := h.registrationService.FindInternalAgentByID(agentID); found {
		if err := internalAgent.InitError(errorType); err != nil {
			logging.Warn(ctx, "InitError() failed for internal agent", "agent", internalAgent.String(), "err", err, "state", internalAgent.GetState().Name())
			rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, StateTransitionFailedForExtensionMessageFormat,
				internalAgent.GetState().Name(), core.AgentInitErrorStateName, agentID.String(), err)
			return
		}
	} else {
		logging.Warn(ctx, "Unknown agent tried to call /extension/init/error")
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentIdentifierUnknown, "Unknown "+model.LambdaAgentIdentifier)
		return
	}

	appctx.StoreFirstFatalError(appctx.FromRequest(request), rapidmodel.WrapErrorIntoCustomerFatalError(nil, errorType))
	rendering.RenderAccepted(writer, request)
}

func NewAgentInitErrorHandler(registrationService core.RegistrationService) http.Handler {
	return &agentInitErrorHandler{
		registrationService: registrationService,
	}
}
