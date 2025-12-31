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

type agentExitErrorHandler struct {
	registrationService core.RegistrationService
}

func (h *agentExitErrorHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	agentID, ok := request.Context().Value(model.AgentIDCtxKey).(uuid.UUID)
	if !ok {
		rendering.RenderInternalServerError(writer, request)
		return
	}

	ctx := logging.WithFields(request.Context(), "agentID", agentID.String())

	var rawErrorType string
	if rawErrorType = request.Header.Get(LambdaAgentFunctionErrorType); rawErrorType == "" {
		logging.Warn(ctx, "Extension exit error missing header", "header", LambdaAgentFunctionErrorType)
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentMissingHeader, "%s not found", LambdaAgentFunctionErrorType)
		return
	}

	errorType := rapidmodel.GetValidExtensionErrorType(rawErrorType, rapidmodel.ErrorAgentExit)
	logging.Warn(ctx, "Received Extension exit error request", "errorType", errorType)

	if externalAgent, found := h.registrationService.FindExternalAgentByID(agentID); found {
		ctx = logging.WithFields(ctx, "extension", externalAgent.Name())
		if err := externalAgent.ExitError(errorType); err != nil {
			logging.Warn(ctx, "Extension exit error transition failed", "err", err, "currentState", externalAgent.GetState().Name())
			rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, StateTransitionFailedForExtensionMessageFormat,
				externalAgent.GetState().Name(), core.AgentExitedStateName, agentID.String(), err)
			return
		}
	} else if internalAgent, found := h.registrationService.FindInternalAgentByID(agentID); found {
		ctx = logging.WithFields(ctx, "extension", internalAgent.Name())
		if err := internalAgent.ExitError(errorType); err != nil {
			logging.Warn(ctx, "Extension exit error transition failed", "err", err, "currentState", internalAgent.GetState().Name())
			rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentInvalidState, StateTransitionFailedForExtensionMessageFormat,
				internalAgent.GetState().Name(), core.AgentExitedStateName, agentID.String(), err)
			return
		}
	} else {
		logging.Warn(ctx, "Unknown extension exit error request")
		rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentIdentifierUnknown, "Unknown "+model.LambdaAgentIdentifier)
		return
	}

	appctx.StoreFirstFatalError(appctx.FromRequest(request), rapidmodel.WrapErrorIntoCustomerFatalError(nil, errorType))
	rendering.RenderAccepted(writer, request)
}

func NewAgentExitErrorHandler(registrationService core.RegistrationService) http.Handler {
	return &agentExitErrorHandler{
		registrationService: registrationService,
	}
}
