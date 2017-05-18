// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"net/http"

	"go.amzn.com/lambda/rapi/model"

	"github.com/go-chi/render"
)

const (
	telemetryAPIDisabledErrorType = "Logs.NotSupported"
)

type runtimeLogsStubHandler struct{}

func (h *runtimeLogsStubHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	render.Status(request, http.StatusAccepted)
	render.JSON(writer, request, &model.ErrorResponse{
		ErrorType:    telemetryAPIDisabledErrorType,
		ErrorMessage: "Logs API is not supported",
	})
}

// NewRuntimeLogsStubHandler returns a new instance of http handler
// for serving /runtime/logs when a telemetry service implementation is absent
func NewRuntimeLogsStubHandler() http.Handler {
	return &runtimeLogsStubHandler{}
}
