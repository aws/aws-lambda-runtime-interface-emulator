// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
)

type runtimeLogsHandler struct {
	registrationService   core.RegistrationService
	telemetrySubscription telemetry.SubscriptionAPI
}

func (h *runtimeLogsHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	agentName, err := h.verifyAgentID(writer, request)
	if err != nil {
		slog.Warn("Agent Verification Error", "err", err)
		switch err := err.(type) {
		case *ErrAgentIdentifierUnknown:
			rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentIdentifierUnknown, "Unknown extension %s", err.agentID.String())
			h.telemetrySubscription.RecordCounterMetric(telemetry.SubscribeClientErr, 1)
		default:
			rendering.RenderInternalServerError(writer, request)
			h.telemetrySubscription.RecordCounterMetric(telemetry.SubscribeServerErr, 1)
		}
		return
	}

	delete(request.Header, model.LambdaAgentIdentifier)

	body, err := h.getBody(writer, request)
	if err != nil {
		slog.Warn("Failed to get request body", "err", err)
		rendering.RenderInternalServerError(writer, request)
		h.telemetrySubscription.RecordCounterMetric(telemetry.SubscribeServerErr, 1)
		return
	}

	respBody, status, headers, err := h.telemetrySubscription.Subscribe(agentName, bytes.NewReader(body), request.Header, request.RemoteAddr)
	if err != nil {
		slog.Warn("Telemetry API error", "err", err)
		switch err {
		case telemetry.ErrTelemetryServiceOff:
			rendering.RenderForbiddenWithTypeMsg(writer, request,
				h.telemetrySubscription.GetServiceClosedErrorType(), "%s", h.telemetrySubscription.GetServiceClosedErrorMessage())
			h.telemetrySubscription.RecordCounterMetric(telemetry.SubscribeClientErr, 1)
		default:
			rendering.RenderInternalServerError(writer, request)
			h.telemetrySubscription.RecordCounterMetric(telemetry.SubscribeServerErr, 1)
		}
		return
	}

	if err := rendering.RenderRuntimeLogsResponse(writer, respBody, status, headers); err != nil {
		slog.Warn("Failed to render runtime logs response", "err", err)
	}
	switch status / 100 {
	case 2:
		if strings.Contains(string(respBody), "OK") {
			h.telemetrySubscription.RecordCounterMetric(telemetry.NumSubscribers, 1)
		}
		h.telemetrySubscription.RecordCounterMetric(telemetry.SubscribeSuccess, 1)
	case 4:
		h.telemetrySubscription.RecordCounterMetric(telemetry.SubscribeClientErr, 1)
		slog.Warn("rendered telemetry api subscription client error response", "body", respBody)
	case 5:
		h.telemetrySubscription.RecordCounterMetric(telemetry.SubscribeServerErr, 1)
		slog.Error("rendered telemetry api subscription server error response", "body", respBody)
	}
}

type ErrAgentIdentifierUnknown struct {
	agentID uuid.UUID
}

func NewErrAgentIdentifierUnknown(agentID uuid.UUID) *ErrAgentIdentifierUnknown {
	return &ErrAgentIdentifierUnknown{
		agentID: agentID,
	}
}

func (e *ErrAgentIdentifierUnknown) Error() string {
	return fmt.Sprintf("Unknown agent %s tried to call /runtime/logs", e.agentID.String())
}

func (h *runtimeLogsHandler) verifyAgentID(writer http.ResponseWriter, request *http.Request) (string, error) {
	agentID, ok := request.Context().Value(model.AgentIDCtxKey).(uuid.UUID)
	if !ok {
		return "", errors.New("internal error: agent ID not set in context")
	}

	agentName, found := h.getAgentName(agentID)
	if !found {
		return "", NewErrAgentIdentifierUnknown(agentID)
	}

	return agentName, nil
}

func (h *runtimeLogsHandler) getAgentName(agentID uuid.UUID) (string, bool) {
	if agent, found := h.registrationService.FindExternalAgentByID(agentID); found {
		return agent.Name(), true
	} else if agent, found := h.registrationService.FindInternalAgentByID(agentID); found {
		return agent.Name(), true
	} else {
		return "", false
	}
}

func (h *runtimeLogsHandler) getBody(writer http.ResponseWriter, request *http.Request) ([]byte, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read error body: %s", err)
	}

	return body, nil
}

func NewRuntimeTelemetrySubscriptionHandler(registrationService core.RegistrationService, telemetrySubscription telemetry.SubscriptionAPI) http.Handler {
	return &runtimeLogsHandler{
		registrationService:   registrationService,
		telemetrySubscription: telemetrySubscription,
	}
}
