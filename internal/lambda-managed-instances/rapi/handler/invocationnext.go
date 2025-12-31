// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"net/http"
	"sync"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type RuntimeNextHandler interface {
	RuntimeNext(ctx context.Context, runtimeReq http.ResponseWriter) (model.RuntimeNextWaiter, model.AppError)
}

type invocationNextHandler struct {
	registrationService core.RegistrationService
	nextHandler         RuntimeNextHandler
	runtimeReadyOnce    sync.Once
}

func (h *invocationNextHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	logging.Debug(ctx, "Received Runtime /next")

	waiter, err := h.nextHandler.RuntimeNext(ctx, writer)
	if err != nil {
		logging.Error(ctx, "Runtime Next Error", "err", err)
		rendering.RenderInternalServerError(writer, request)
		return
	}

	h.runtimeReadyOnce.Do(func() {
		if err := h.registrationService.InitFlow().RuntimeReady(); err != nil {
			logging.Warn(ctx, "Could not register runtime", "err", err)
			rendering.RenderForbiddenWithTypeMsg(writer, request, rendering.ErrorTypeInvalidStateTransition, StateTransitionFailedForRuntimeMessageFormat,
				h.registrationService.GetRuntime().GetState().Name(), core.RuntimeReadyStateName, err)
			return
		}
	})

	if err := waiter.RuntimeNextWait(ctx); err != nil {
		logging.Warn(ctx, "Cancelled /next", "err", err)
		rendering.RenderInternalServerError(writer, request)
	}
}

func NewInvocationNextHandler(registrationService core.RegistrationService, nextHandler RuntimeNextHandler) http.Handler {
	return &invocationNextHandler{
		registrationService: registrationService,
		nextHandler:         nextHandler,
	}
}
