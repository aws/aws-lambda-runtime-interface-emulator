// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapi

import (
	"net/http"

	"github.com/go-chi/chi"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/handler"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/middleware"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
)

type runtimeRequestHandler interface {
	handler.RuntimeNextHandler
	handler.RuntimeResponseHandler
	handler.RuntimeErrorHandler
}

func NewRouter(appCtx appctx.ApplicationContext, registrationService core.RegistrationService, renderingService *rendering.EventRenderingService, runtimeReqHandler runtimeRequestHandler) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.AppCtxMiddleware(appCtx))
	router.Use(middleware.AccessLogMiddleware())
	router.Use(middleware.RuntimeReleaseMiddleware())

	router.Get("/ping", http.MaxBytesHandler(handler.NewPingHandler(), 0).ServeHTTP)

	router.Get("/runtime/invocation/next",
		http.MaxBytesHandler(handler.NewInvocationNextHandler(registrationService, runtimeReqHandler), 0).ServeHTTP)

	router.Post("/runtime/invocation/{awsrequestid}/response",
		handler.NewInvocationResponseHandler(runtimeReqHandler).ServeHTTP)

	router.Post("/runtime/invocation/{awsrequestid}/error",
		http.MaxBytesHandler(handler.NewInvocationErrorHandler(runtimeReqHandler), requestBodyLimitBytes).ServeHTTP)

	router.Post("/runtime/init/error", http.MaxBytesHandler(handler.NewInitErrorHandler(registrationService), requestBodyLimitBytes).ServeHTTP)
	return router
}

func ExtensionsRouter(appCtx appctx.ApplicationContext, registrationService core.RegistrationService, renderingService *rendering.EventRenderingService) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.AccessLogMiddleware())
	router.Use(middleware.AppCtxMiddleware(appCtx))

	registerHandler := handler.NewAgentRegisterHandler(registrationService)
	router.Post("/extension/register",
		registerHandler.ServeHTTP)

	router.Get("/extension/event/next",
		middleware.AgentUniqueIdentifierHeaderValidator(
			http.MaxBytesHandler(handler.NewAgentNextHandler(registrationService, renderingService), 0)).ServeHTTP)

	router.Post("/extension/init/error",
		middleware.AgentUniqueIdentifierHeaderValidator(
			handler.NewAgentInitErrorHandler(registrationService)).ServeHTTP)

	router.Post("/extension/exit/error",
		middleware.AgentUniqueIdentifierHeaderValidator(
			handler.NewAgentExitErrorHandler(registrationService)).ServeHTTP)

	return router
}

func TelemetryAPIRouter(registrationService core.RegistrationService, telemetrySubscriptionAPI telemetry.SubscriptionAPI) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.AccessLogMiddleware())

	router.Put("/telemetry",
		middleware.AgentUniqueIdentifierHeaderValidator(
			handler.NewRuntimeTelemetrySubscriptionHandler(registrationService, telemetrySubscriptionAPI)).ServeHTTP)

	return router
}
