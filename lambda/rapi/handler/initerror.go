// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/telemetry"

	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/rapi/rendering"

	log "github.com/sirupsen/logrus"
)

type initErrorHandler struct {
	registrationService core.RegistrationService
	eventsAPI           telemetry.EventsAPI
}

func (h *initErrorHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	appCtx := appctx.FromRequest(request)

	server := appctx.LoadInteropServer(appCtx)
	if server == nil {
		log.Panic("Invalid state, cannot access interop server")
	}

	runtime := h.registrationService.GetRuntime()

	// the previousStateName is needed to define if the init/error is called for INIT or RESTORE
	previousStateName := runtime.GetState().Name()

	if err := runtime.InitError(); err != nil {
		log.Warn(err)
		rendering.RenderForbiddenWithTypeMsg(writer, request, rendering.ErrorTypeInvalidStateTransition, StateTransitionFailedForRuntimeMessageFormat,
			runtime.GetState().Name(), core.RuntimeInitErrorStateName, err)
		return
	}

	errorType := request.Header.Get("Lambda-Runtime-Function-Error-Type")

	errorBody, err := io.ReadAll(request.Body)
	if err != nil {
		log.WithError(err).Warn("Failed to read error body")
	}

	if previousStateName == core.RuntimeRestoringStateName {
		h.sendRestoreRuntimeDoneLogEvent()
	} else {
		h.sendInitRuntimeDoneLogEvent(appCtx)
	}

	response := &interop.ErrorResponse{
		ErrorType:   errorType,
		Payload:     errorBody,
		ContentType: determineJSONContentType(errorBody),
	}

	if err := server.SendInitErrorResponse(server.GetCurrentInvokeID(), response); err != nil {
		rendering.RenderInteropError(writer, request, err)
		return
	}

	appctx.StoreErrorResponse(appCtx, response)

	rendering.RenderAccepted(writer, request)
}

// NewInitErrorHandler returns a new instance of http handler
// for serving /runtime/init/error.
func NewInitErrorHandler(registrationService core.RegistrationService, eventsAPI telemetry.EventsAPI) http.Handler {
	return &initErrorHandler{
		registrationService: registrationService,
		eventsAPI:           eventsAPI,
	}
}

func determineJSONContentType(body []byte) string {
	if json.Valid(body) {
		return "application/json"
	}
	return "application/octet-stream"
}

func (h *initErrorHandler) sendInitRuntimeDoneLogEvent(appCtx appctx.ApplicationContext) {
	// ToDo: Convert this to an enum for the whole package to increase readability.
	initCachingEnabled := appctx.LoadInitType(appCtx) == appctx.InitCaching

	initSource := interop.InferTelemetryInitSource(initCachingEnabled, appctx.LoadSandboxType(appCtx))
	runtimeDoneData := &telemetry.InitRuntimeDoneData{
		InitSource: initSource,
		Status:     telemetry.RuntimeDoneFailure,
	}

	if err := h.eventsAPI.SendInitRuntimeDone(runtimeDoneData); err != nil {
		log.Errorf("Failed to send INITRD: %s", err)
	}
}

func (h *initErrorHandler) sendRestoreRuntimeDoneLogEvent() {
	if err := h.eventsAPI.SendRestoreRuntimeDone(telemetry.RuntimeDoneFailure); err != nil {
		log.Errorf("Failed to send RESTRD: %s", err)
	}
}
