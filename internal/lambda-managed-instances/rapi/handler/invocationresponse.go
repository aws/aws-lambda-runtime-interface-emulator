// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/invoke"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type RuntimeResponseHandler interface {
	RuntimeResponse(ctx context.Context, runtimeRespReq invoke.RuntimeResponseRequest) model.AppError
}

type invocationResponseHandler struct {
	runtimeRespHandler RuntimeResponseHandler
}

func (h *invocationResponseHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	invokeID := chi.URLParam(request, "awsrequestid")
	ctx := logging.WithInvokeID(request.Context(), invokeID)

	logging.Debug(ctx, "Received Runtime Response")
	resp := invoke.NewRuntimeResponse(ctx, request, invokeID)

	err := h.runtimeRespHandler.RuntimeResponse(ctx, &resp)
	if err == nil {
		rendering.RenderAccepted(writer, request)
		return
	}

	logging.Warn(ctx, "Runtime Response Error", "err", err)

	switch err.ErrorType() {
	case model.ErrorRuntimeInvalidInvokeId, model.ErrorRuntimeInvokeResponseInProgress:

		rendering.RenderInvalidRequestID(writer, request)
	case model.ErrorRuntimeInvokeTimeout, model.ErrorSandboxTimedout:
		rendering.RenderInvokeTimeout(writer, request)
	case model.ErrorRuntimeInvalidResponseModeHeader:
		rendering.RenderInvalidFunctionResponseMode(writer, request)
	case model.ErrorFunctionOversizedResponse:
		rendering.RenderRequestEntityTooLarge(writer, request)
	case model.ErrorRuntimeTruncatedResponse:
		rendering.RenderTruncatedHTTPRequestError(writer, request)
	default:

		if trailerError := resp.TrailerError(); trailerError.ErrorType() != "" {
			rendering.RenderAccepted(writer, request)
			return
		}
		logging.Error(ctx, "unexpected error in runtime response", "err", err)
		rendering.RenderInternalServerError(writer, request)
	}
}

func NewInvocationResponseHandler(runtimeRespHandler RuntimeResponseHandler) http.Handler {
	return &invocationResponseHandler{
		runtimeRespHandler: runtimeRespHandler,
	}
}
