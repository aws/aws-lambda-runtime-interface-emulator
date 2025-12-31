// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"context"
	"net/http"

	"github.com/go-chi/chi"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/invoke"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type RuntimeErrorHandler interface {
	RuntimeError(ctx context.Context, runtimeErrReq invoke.RuntimeErrorRequest) model.AppError
}

type invocationErrorHandler struct {
	runtimeErrHandler RuntimeErrorHandler
}

func (h *invocationErrorHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	invokeID := chi.URLParam(request, "awsrequestid")
	ctx := logging.WithInvokeID(request.Context(), invokeID)

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(request.Body); err != nil {
		logging.Warn(ctx, "Failed to parse error body", "err", err)
		rendering.RenderRequestEntityTooLarge(writer, request)
		return
	}

	resp := invoke.NewRuntimeError(ctx, request, invokeID, buf.String())
	err := h.runtimeErrHandler.RuntimeError(ctx, &resp)
	logging.Warn(ctx, "Received Runtime error", "err", err)
	if err == nil {
		rendering.RenderAccepted(writer, request)
		return
	}

	logging.Warn(ctx, "Runtime response error", "err", err)

	switch err.ErrorType() {
	case model.ErrorRuntimeInvalidInvokeId, model.ErrorRuntimeInvokeErrorInProgress:

		rendering.RenderInvalidRequestID(writer, request)
	case model.ErrorRuntimeInvokeTimeout:
		rendering.RenderInvokeTimeout(writer, request)
	case model.ErrorRuntimeInvokeResponseWasSent:

		rendering.RenderInvalidRequestID(writer, request)
	default:

		logging.Error(ctx, "Received unexpected runtime error", "err", err)
		rendering.RenderInternalServerError(writer, request)
	}
}

func NewInvocationErrorHandler(runtimeErrHandler RuntimeErrorHandler) http.Handler {
	return &invocationErrorHandler{
		runtimeErrHandler: runtimeErrHandler,
	}
}
