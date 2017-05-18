// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/rapi/rendering"
	"go.amzn.com/lambda/rapidcore/telemetry/logsapi"
	"go.amzn.com/lambda/telemetry"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type runtimeLogsHandler struct {
	registrationService core.RegistrationService
	telemetryService    telemetry.LogsAPIService
}

func (h *runtimeLogsHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	agentName, err := h.verifyAgentID(writer, request)
	if err != nil {
		log.Errorf("Agent Verification Error: %s", err)
		switch err := err.(type) {
		case *ErrAgentIdentifierUnknown:
			rendering.RenderForbiddenWithTypeMsg(writer, request, errAgentIdentifierUnknown, "Unknown extension "+err.agentID.String())
			h.telemetryService.RecordCounterMetric(logsapi.SubscribeClientErr, 1)
		default:
			rendering.RenderInternalServerError(writer, request)
			h.telemetryService.RecordCounterMetric(logsapi.SubscribeServerErr, 1)
		}
		return
	}

	delete(request.Header, LambdaAgentIdentifier)

	body, err := h.getBody(writer, request)
	if err != nil {
		log.Error(err)
		rendering.RenderInternalServerError(writer, request)
		h.telemetryService.RecordCounterMetric(logsapi.SubscribeServerErr, 1)
		return
	}

	respBody, status, headers, err := h.telemetryService.Subscribe(agentName, bytes.NewReader(body), request.Header)
	if err != nil {
		log.Errorf("Telemetry API error: %s", err)
		switch err {
		case logsapi.ErrTelemetryServiceOff:
			rendering.RenderForbiddenWithTypeMsg(writer, request,
				errLogsSubscriptionClosed, "Logs API subscription is closed already")
			h.telemetryService.RecordCounterMetric(logsapi.SubscribeClientErr, 1)
		default:
			rendering.RenderInternalServerError(writer, request)
			h.telemetryService.RecordCounterMetric(logsapi.SubscribeServerErr, 1)
		}
		return
	}

	rendering.RenderRuntimeLogsResponse(writer, respBody, status, headers)
	switch status / 100 {
	case 2: // 2xx
		h.telemetryService.RecordCounterMetric(logsapi.SubscribeSuccess, 1)
	case 4: // 4xx
		h.telemetryService.RecordCounterMetric(logsapi.SubscribeClientErr, 1)
	case 5: // 5xx
		h.telemetryService.RecordCounterMetric(logsapi.SubscribeServerErr, 1)
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
	agentID, ok := request.Context().Value(AgentIDCtxKey).(uuid.UUID)
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
		return agent.Name, true
	} else if agent, found := h.registrationService.FindInternalAgentByID(agentID); found {
		return agent.Name, true
	} else {
		return "", false
	}
}

func (h *runtimeLogsHandler) getBody(writer http.ResponseWriter, request *http.Request) ([]byte, error) {
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read error body: %s", err)
	}

	return body, nil
}

// NewRuntimeLogsHandler returns a new instance of http handler
// for serving /runtime/logs
func NewRuntimeLogsHandler(registrationService core.RegistrationService, telemetryService telemetry.LogsAPIService) http.Handler {
	return &runtimeLogsHandler{
		registrationService: registrationService,
		telemetryService:    telemetryService,
	}
}
