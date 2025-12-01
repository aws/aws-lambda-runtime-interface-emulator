// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapi/rendering"
)

const (
	logsAPIDisabledErrorType      = "Logs.NotSupported"
	telemetryAPIDisabledErrorType = "Telemetry.NotSupported"
)

type runtimeLogsStubAPIHandler struct{}

func (h *runtimeLogsStubAPIHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if err := rendering.RenderJSON(http.StatusAccepted, writer, request, &model.ErrorResponse{
		ErrorType:    logsAPIDisabledErrorType,
		ErrorMessage: "Logs API is not supported",
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// NewRuntimeLogsAPIStubHandler returns a new instance of http handler
// for serving /runtime/logs when a telemetry service implementation is absent
func NewRuntimeLogsAPIStubHandler() http.Handler {
	return &runtimeLogsStubAPIHandler{}
}

type runtimeTelemetryAPIStubHandler struct{}

func (h *runtimeTelemetryAPIStubHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if err := rendering.RenderJSON(http.StatusAccepted, writer, request, &model.ErrorResponse{
		ErrorType:    telemetryAPIDisabledErrorType,
		ErrorMessage: "Telemetry API is not supported",
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// NewRuntimeTelemetryAPIStubHandler returns a new instance of http handler
// for serving /runtime/logs when a telemetry service implementation is absent
func NewRuntimeTelemetryAPIStubHandler() http.Handler {
	return &runtimeTelemetryAPIStubHandler{}
}
