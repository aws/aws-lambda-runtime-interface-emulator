// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"log/slog"
	"net/http"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
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
		slog.Warn("Error while rendering response", "err", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func NewRuntimeLogsAPIStubHandler() http.Handler {
	return &runtimeLogsStubAPIHandler{}
}

type runtimeTelemetryAPIStubHandler struct{}

func (h *runtimeTelemetryAPIStubHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if err := rendering.RenderJSON(http.StatusAccepted, writer, request, &model.ErrorResponse{
		ErrorType:    telemetryAPIDisabledErrorType,
		ErrorMessage: "Telemetry API is not supported",
	}); err != nil {
		slog.Warn("Error while rendering response", "err", err)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func NewRuntimeTelemetryAPIStubHandler() http.Handler {
	return &runtimeTelemetryAPIStubHandler{}
}
