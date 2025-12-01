// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"net/http"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type initErrorHandler struct {
	registrationService core.RegistrationService
}

func (h *initErrorHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	appCtx := appctx.FromRequest(request)
	ctx := request.Context()

	errorType := model.GetValidRuntimeOrFunctionErrorType(request.Header.Get("Lambda-Runtime-Function-Error-Type"))
	ctx = logging.WithFields(ctx, "errType", errorType)

	logging.Warn(ctx, "Received Runtime Init Error")

	runtime := h.registrationService.GetRuntime()

	if err := runtime.InitError(); err != nil {
		logging.Warn(ctx, "Runtime init error", "err", err)
		rendering.RenderForbiddenWithTypeMsg(
			writer,
			request,
			rendering.ErrorTypeInvalidStateTransition,
			StateTransitionFailedForRuntimeMessageFormat,
			runtime.GetState().Name(),
			core.RuntimeInitErrorStateName,
			err,
		)
		return
	}

	appctx.StoreFirstFatalError(appCtx, model.WrapErrorIntoCustomerFatalError(nil, errorType))

	appctx.StoreInvokeErrorTraceData(appCtx, &interop.InvokeErrorTraceData{})
	rendering.RenderAccepted(writer, request)
}

func NewInitErrorHandler(registrationService core.RegistrationService) http.Handler {
	return &initErrorHandler{registrationService: registrationService}
}
