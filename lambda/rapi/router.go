// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapi

import (
	"net/http"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/rapi/handler"
	"go.amzn.com/lambda/rapi/middleware"
	"go.amzn.com/lambda/telemetry"

	"github.com/go-chi/chi"

	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/rapi/rendering"
)

// NewRouter returns a new instance of chi router implementing
// Runtime API specification.
func NewRouter(appCtx appctx.ApplicationContext, registrationService core.RegistrationService, renderingService *rendering.EventRenderingService) http.Handler {

	router := chi.NewRouter()
	router.Use(middleware.AppCtxMiddleware(appCtx))
	router.Use(middleware.AccessLogMiddleware())
	router.Use(middleware.RuntimeReleaseMiddleware())

	// To respect Hyrum's Law, keeping /ping API even though
	// we no longer use it ourselves.
	// http://www.hyrumslaw.com/
	router.Get("/ping", handler.NewPingHandler().ServeHTTP)

	router.Get("/runtime/invocation/next",
		handler.NewInvocationNextHandler(registrationService, renderingService).ServeHTTP)

	// Note, request validation must happen before state
	// transition. State machine transitions are irreversible
	// at the moment.
	router.Post("/runtime/invocation/{awsrequestid}/response",
		middleware.AwsRequestIDValidator(
			handler.NewInvocationResponseHandler(registrationService)).ServeHTTP)

	router.Post("/runtime/invocation/{awsrequestid}/error",
		middleware.AwsRequestIDValidator(
			handler.NewInvocationErrorHandler(registrationService)).ServeHTTP)

	router.Post("/runtime/init/error", handler.NewInitErrorHandler(registrationService).ServeHTTP)

	if appctx.LoadInitType(appCtx) == appctx.InitCaching {
		router.Get("/runtime/restore/next", handler.NewRestoreNextHandler(registrationService, renderingService).ServeHTTP)
		router.Post("/runtime/restore/error", handler.NewRestoreErrorHandler(registrationService).ServeHTTP)
	}

	return router
}

// ExtensionsRouter returns a new instance of chi router implementing
// Extensions Runtime API specification.
func ExtensionsRouter(appCtx appctx.ApplicationContext, registrationService core.RegistrationService, renderingService *rendering.EventRenderingService) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.AccessLogMiddleware())
	router.Use(middleware.AllowIfExtensionsEnabled)
	router.Use(middleware.AppCtxMiddleware(appCtx))

	registerHandler := handler.NewAgentRegisterHandler(registrationService)
	router.Post("/extension/register",
		registerHandler.ServeHTTP)

	router.Get("/extension/event/next",
		middleware.AgentUniqueIdentifierHeaderValidator(
			handler.NewAgentNextHandler(registrationService, renderingService)).ServeHTTP)

	router.Post("/extension/init/error",
		middleware.AgentUniqueIdentifierHeaderValidator(
			handler.NewAgentInitErrorHandler(registrationService)).ServeHTTP)

	router.Post("/extension/exit/error",
		middleware.AgentUniqueIdentifierHeaderValidator(
			handler.NewAgentExitErrorHandler(registrationService)).ServeHTTP)

	return router
}

// LogsAPIRouter returns a new instance of chi router implementing
// Logs API specification.
func LogsAPIRouter(registrationService core.RegistrationService, logsSubscriptionAPI telemetry.SubscriptionAPI) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.AccessLogMiddleware())
	router.Use(middleware.AllowIfExtensionsEnabled)

	router.Put("/logs",
		middleware.AgentUniqueIdentifierHeaderValidator(
			handler.NewRuntimeTelemetrySubscriptionHandler(registrationService, logsSubscriptionAPI)).ServeHTTP)

	return router
}

// LogsAPIStubRouter returns a new instance of chi router implementing
// a stub of Logs API that always returns a non-committal response to
// prevent customer code from crashing when Logs API is disabled locally
func LogsAPIStubRouter() http.Handler {
	router := chi.NewRouter()

	router.Put("/logs", handler.NewRuntimeLogsAPIStubHandler().ServeHTTP)

	return router
}

// TelemetryRouter returns a new instance of chi router implementing
// Telemetry API specification.
func TelemetryAPIRouter(registrationService core.RegistrationService, telemetrySubscriptionAPI telemetry.SubscriptionAPI) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.AccessLogMiddleware())
	router.Use(middleware.AllowIfExtensionsEnabled)

	router.Put("/telemetry",
		middleware.AgentUniqueIdentifierHeaderValidator(
			handler.NewRuntimeTelemetrySubscriptionHandler(registrationService, telemetrySubscriptionAPI)).ServeHTTP)

	return router
}

// TelemetryStubRouter returns a new instance of chi router implementing
// a stub of Telemetry API that always returns a non-committal response to
// prevent customer code from crashing when Telemetry API is disabled locally
func TelemetryAPIStubRouter() http.Handler {
	router := chi.NewRouter()

	router.Put("/telemetry", handler.NewRuntimeTelemetryAPIStubHandler().ServeHTTP)

	return router
}

func CredentialsAPIRouter(credentialsService core.CredentialsService) http.Handler {
	router := chi.NewRouter()

	router.Get("/credentials", handler.NewCredentialsHandler(credentialsService).ServeHTTP)

	return router
}
