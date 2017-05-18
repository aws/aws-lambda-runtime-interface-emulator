// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapidcore/telemetry/logsapi"
)

type mockTelemetryService struct{ mock.Mock }

func (s *mockTelemetryService) Subscribe(agentName string, body io.Reader, headers map[string][]string) ([]byte, int, map[string][]string, error) {
	args := s.Called(agentName, body, headers)
	return args.Get(0).([]byte), args.Int(1), args.Get(2).(map[string][]string), args.Error(3)
}

func (s *mockTelemetryService) RecordCounterMetric(metricName string, count int) {
	s.Called(metricName, count)
}

func (s *mockTelemetryService) FlushMetrics() interop.LogsAPIMetrics {
	args := s.Called()
	return args.Get(0).(interop.LogsAPIMetrics)
}

func (s *mockTelemetryService) Clear() {
	s.Called()
}

func (s *mockTelemetryService) TurnOff() {
	s.Called()
}

func TestSuccessfulRuntimeLogsResponseProxy(t *testing.T) {
	agentName, reqBody, reqHeaders := "dummyName", []byte(`foobar`), map[string][]string{"Key": []string{"VAL1", "VAL2"}}
	respBody, respStatus, respHeaders := []byte(`barbaz`), http.StatusNotFound, map[string][]string{"K": []string{"V1", "V2"}}
	clientErrMetric := logsapi.SubscribeClientErr

	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	agent, err := registrationService.CreateExternalAgent(agentName)
	assert.NoError(t, err)

	telemetryService := &mockTelemetryService{}
	telemetryService.On("Subscribe", agentName, bytes.NewReader(reqBody), reqHeaders).Return(respBody, respStatus, respHeaders, nil)
	telemetryService.On("RecordCounterMetric", clientErrMetric, 1)

	handler := NewRuntimeLogsHandler(registrationService, telemetryService)
	request := httptest.NewRequest("PUT", "/", bytes.NewBuffer(reqBody))
	for k, vals := range reqHeaders {
		for _, v := range vals {
			request.Header.Add(k, v)
		}
	}

	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)

	telemetryService.AssertCalled(t, "Subscribe", agentName, bytes.NewReader(reqBody), reqHeaders)
	telemetryService.AssertCalled(t, "RecordCounterMetric", clientErrMetric, 1)

	recordedBody, err := ioutil.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)

	assert.Equal(t, respStatus, responseRecorder.Code)
	assert.Equal(t, respBody, recordedBody)
	assert.Equal(t, http.Header(respHeaders), responseRecorder.Header())
}

func TestErrorUnregisteredAgentID(t *testing.T) {
	invalidAgentID := uuid.New()
	reqBody, reqHeaders := []byte(`foobar`), map[string][]string{"Key": []string{"VAL1", "VAL2"}}
	clientErrMetric := logsapi.SubscribeClientErr

	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	telemetryService := &mockTelemetryService{}
	telemetryService.On("RecordCounterMetric", clientErrMetric, 1)

	handler := NewRuntimeLogsHandler(registrationService, telemetryService)
	request := httptest.NewRequest("PUT", "/", bytes.NewBuffer(reqBody))
	for k, vals := range reqHeaders {
		for _, v := range vals {
			request.Header.Add(k, v)
		}
	}

	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, invalidAgentID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)

	recordedBody, err := ioutil.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)

	expectedErrorBody := fmt.Sprintf(`{"errorMessage":"Unknown extension %s","errorType":"Extension.UnknownExtensionIdentifier"}`+"\n", invalidAgentID.String())
	expectedHeaders := http.Header(map[string][]string{"Content-Type": []string{"application/json; charset=utf-8"}})

	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assert.Equal(t, expectedErrorBody, string(recordedBody))
	assert.Equal(t, expectedHeaders, responseRecorder.Header())
	telemetryService.AssertCalled(t, "RecordCounterMetric", clientErrMetric, 1)
}

func TestErrorTelemetryAPICallFailure(t *testing.T) {
	agentName, reqBody, reqHeaders := "dummyName", []byte(`foobar`), map[string][]string{"Key": []string{"VAL1", "VAL2"}}
	apiError := errors.New("Error calling Telemetry API: connection refused")
	serverErrMetric := logsapi.SubscribeServerErr

	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	agent, err := registrationService.CreateExternalAgent(agentName)
	assert.NoError(t, err)

	telemetryService := &mockTelemetryService{}
	telemetryService.On("Subscribe", agentName, bytes.NewReader(reqBody), reqHeaders).Return([]byte(``), http.StatusOK, map[string][]string{}, apiError)
	telemetryService.On("RecordCounterMetric", serverErrMetric, 1)

	handler := NewRuntimeLogsHandler(registrationService, telemetryService)
	request := httptest.NewRequest("PUT", "/", bytes.NewBuffer(reqBody))
	for k, vals := range reqHeaders {
		for _, v := range vals {
			request.Header.Add(k, v)
		}
	}

	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)

	recordedBody, err := ioutil.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)

	expectedErrorBody := `{"errorMessage":"Internal Server Error","errorType":"InternalServerError"}` + "\n"
	expectedHeaders := http.Header(map[string][]string{"Content-Type": []string{"application/json; charset=utf-8"}})

	assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	assert.Equal(t, expectedErrorBody, string(recordedBody))
	assert.Equal(t, expectedHeaders, responseRecorder.Header())
	telemetryService.AssertCalled(t, "RecordCounterMetric", serverErrMetric, 1)
}

func TestRenderLogsSubscriptionClosed(t *testing.T) {
	agentName, reqBody, reqHeaders := "dummyName", []byte(`foobar`), map[string][]string{"Key": []string{"VAL1", "VAL2"}}
	apiError := logsapi.ErrTelemetryServiceOff
	clientErrMetric := logsapi.SubscribeClientErr

	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	agent, err := registrationService.CreateExternalAgent(agentName)
	assert.NoError(t, err)

	telemetryService := &mockTelemetryService{}
	telemetryService.On("Subscribe", agentName, bytes.NewReader(reqBody), reqHeaders).Return([]byte(``), http.StatusOK, map[string][]string{}, apiError)
	telemetryService.On("RecordCounterMetric", clientErrMetric, 1)

	handler := NewRuntimeLogsHandler(registrationService, telemetryService)
	request := httptest.NewRequest("PUT", "/", bytes.NewBuffer(reqBody))
	for k, vals := range reqHeaders {
		for _, v := range vals {
			request.Header.Add(k, v)
		}
	}

	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)

	recordedBody, err := ioutil.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)

	expectedErrorBody := `{"errorMessage":"Logs API subscription is closed already","errorType":"Logs.SubscriptionClosed"}` + "\n"
	expectedHeaders := http.Header(map[string][]string{"Content-Type": []string{"application/json; charset=utf-8"}})

	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assert.Equal(t, expectedErrorBody, string(recordedBody))
	assert.Equal(t, expectedHeaders, responseRecorder.Header())
	telemetryService.AssertCalled(t, "RecordCounterMetric", clientErrMetric, 1)
}
