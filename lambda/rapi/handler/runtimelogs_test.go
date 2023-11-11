// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/telemetry"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockSubscriptionAPI struct{ mock.Mock }

func (s *mockSubscriptionAPI) Subscribe(agentName string, body io.Reader, headers map[string][]string, remoteAddr string) ([]byte, int, map[string][]string, error) {
	args := s.Called(agentName, body, headers, remoteAddr)
	return args.Get(0).([]byte), args.Int(1), args.Get(2).(map[string][]string), args.Error(3)
}

func (s *mockSubscriptionAPI) RecordCounterMetric(metricName string, count int) {
	s.Called(metricName, count)
}

func (s *mockSubscriptionAPI) FlushMetrics() interop.TelemetrySubscriptionMetrics {
	args := s.Called()
	return args.Get(0).(interop.TelemetrySubscriptionMetrics)
}

func (s *mockSubscriptionAPI) Clear() {
	s.Called()
}

func (s *mockSubscriptionAPI) TurnOff() {
	s.Called()
}

func (s *mockSubscriptionAPI) GetEndpointURL() string {
	args := s.Called()
	return args.Get(0).(string)
}

func (s *mockSubscriptionAPI) GetServiceClosedErrorMessage() string {
	args := s.Called()
	return args.Get(0).(string)
}

func (s *mockSubscriptionAPI) GetServiceClosedErrorType() string {
	args := s.Called()
	return args.Get(0).(string)
}

func validIPPort(addr string) bool {
	ip, _, err := net.SplitHostPort(addr)
	return err == nil && net.ParseIP(ip) != nil
}

func TestSuccessfulRuntimeLogsResponseProxy(t *testing.T) {
	agentName, reqBody, reqHeaders := "dummyName", []byte(`foobar`), map[string][]string{"Key": []string{"VAL1", "VAL2"}}
	respBody, respStatus, respHeaders := []byte(`barbaz`), http.StatusNotFound, map[string][]string{"K": []string{"V1", "V2"}}
	clientErrMetric := telemetry.SubscribeClientErr

	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	agent, err := registrationService.CreateExternalAgent(agentName)
	assert.NoError(t, err)

	telemetrySubscription := &mockSubscriptionAPI{}
	telemetrySubscription.On("Subscribe", agentName, bytes.NewReader(reqBody), reqHeaders, mock.MatchedBy(validIPPort)).Return(respBody, respStatus, respHeaders, nil)
	telemetrySubscription.On("RecordCounterMetric", clientErrMetric, 1)

	handler := NewRuntimeTelemetrySubscriptionHandler(registrationService, telemetrySubscription)
	request := httptest.NewRequest("PUT", "/", bytes.NewBuffer(reqBody))
	for k, vals := range reqHeaders {
		for _, v := range vals {
			request.Header.Add(k, v)
		}
	}

	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)

	telemetrySubscription.AssertCalled(t, "Subscribe", agentName, bytes.NewReader(reqBody), reqHeaders, mock.MatchedBy(validIPPort))
	telemetrySubscription.AssertCalled(t, "RecordCounterMetric", clientErrMetric, 1)

	recordedBody, err := io.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)

	assert.Equal(t, respStatus, responseRecorder.Code)
	assert.Equal(t, respBody, recordedBody)
	assert.Equal(t, http.Header(respHeaders), responseRecorder.Header())
}

func TestSuccessfulTelemetryAPIPutRequest(t *testing.T) {
	agentName, reqBody, reqHeaders := "extensionName", []byte(`foobar`), map[string][]string{"Key": []string{"VAL1", "VAL2"}}
	respBody, respStatus, respHeaders := []byte(`"OK"`), http.StatusOK, map[string][]string{"K": []string{"V1", "V2"}}
	numSubscribersMetric := telemetry.NumSubscribers
	subscribeSuccessMetric := telemetry.SubscribeSuccess

	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	agent, err := registrationService.CreateExternalAgent(agentName)
	assert.NoError(t, err)

	telemetrySubscription := &mockSubscriptionAPI{}
	telemetrySubscription.On("Subscribe", agentName, bytes.NewReader(reqBody), reqHeaders, mock.MatchedBy(validIPPort)).Return(respBody, respStatus, respHeaders, nil)
	telemetrySubscription.On("RecordCounterMetric", numSubscribersMetric, 1)
	telemetrySubscription.On("RecordCounterMetric", subscribeSuccessMetric, 1)

	handler := NewRuntimeTelemetrySubscriptionHandler(registrationService, telemetrySubscription)
	request := httptest.NewRequest("PUT", "/", bytes.NewBuffer(reqBody))
	for k, vals := range reqHeaders {
		for _, v := range vals {
			request.Header.Add(k, v)
		}
	}

	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)

	telemetrySubscription.AssertCalled(t, "Subscribe", agentName, bytes.NewReader(reqBody), reqHeaders, mock.MatchedBy(validIPPort))
	telemetrySubscription.AssertCalled(t, "RecordCounterMetric", numSubscribersMetric, 1)
	telemetrySubscription.AssertCalled(t, "RecordCounterMetric", subscribeSuccessMetric, 1)

	recordedBody, err := io.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)

	assert.Equal(t, respStatus, responseRecorder.Code)
	assert.Equal(t, respBody, recordedBody)
	assert.Equal(t, http.Header(respHeaders), responseRecorder.Header())
}

func TestNumberOfSubscribersWhenAnExtensionIsAlreadySubscribed(t *testing.T) {
	agentName, reqBody, reqHeaders := "extensionName", []byte(`foobar`), map[string][]string{"Key": []string{"VAL1", "VAL2"}}
	respBody, respStatus, respHeaders := []byte(`"AlreadySubcribed"`), http.StatusOK, map[string][]string{"K": []string{"V1", "V2"}}
	numSubscribersMetric := telemetry.NumSubscribers
	subscribeSuccessMetric := telemetry.SubscribeSuccess

	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	agent, err := registrationService.CreateExternalAgent(agentName)
	assert.NoError(t, err)

	telemetrySubscription := &mockSubscriptionAPI{}
	telemetrySubscription.On("Subscribe", agentName, bytes.NewReader(reqBody), reqHeaders, mock.MatchedBy(validIPPort)).Return(respBody, respStatus, respHeaders, nil)
	telemetrySubscription.On("RecordCounterMetric", subscribeSuccessMetric, 1)

	handler := NewRuntimeTelemetrySubscriptionHandler(registrationService, telemetrySubscription)
	request := httptest.NewRequest("PUT", "/", bytes.NewBuffer(reqBody))
	for k, vals := range reqHeaders {
		for _, v := range vals {
			request.Header.Add(k, v)
		}
	}

	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)

	telemetrySubscription.AssertCalled(t, "Subscribe", agentName, bytes.NewReader(reqBody), reqHeaders, mock.MatchedBy(validIPPort))
	telemetrySubscription.AssertCalled(t, "RecordCounterMetric", subscribeSuccessMetric, 1)
	telemetrySubscription.AssertNotCalled(t, "RecordCounterMetric", numSubscribersMetric, mock.Anything)

	recordedBody, err := io.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)

	assert.Equal(t, respStatus, responseRecorder.Code)
	assert.Equal(t, respBody, recordedBody)
	assert.Equal(t, http.Header(respHeaders), responseRecorder.Header())
}

func TestErrorUnregisteredAgentID(t *testing.T) {
	invalidAgentID := uuid.New()
	reqBody, reqHeaders := []byte(`foobar`), map[string][]string{"Key": []string{"VAL1", "VAL2"}}
	clientErrMetric := telemetry.SubscribeClientErr

	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	telemetrySubscription := &mockSubscriptionAPI{}
	telemetrySubscription.On("RecordCounterMetric", clientErrMetric, 1)

	handler := NewRuntimeTelemetrySubscriptionHandler(registrationService, telemetrySubscription)
	request := httptest.NewRequest("PUT", "/", bytes.NewBuffer(reqBody))
	for k, vals := range reqHeaders {
		for _, v := range vals {
			request.Header.Add(k, v)
		}
	}

	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, invalidAgentID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)

	recordedBody, err := io.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)

	expectedErrorBody := fmt.Sprintf(`{"errorMessage":"Unknown extension %s","errorType":"Extension.UnknownExtensionIdentifier"}`+"\n", invalidAgentID.String())
	expectedHeaders := http.Header(map[string][]string{"Content-Type": []string{"application/json"}})

	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assert.Equal(t, expectedErrorBody, string(recordedBody))
	assert.Equal(t, expectedHeaders, responseRecorder.Header())
	telemetrySubscription.AssertCalled(t, "RecordCounterMetric", clientErrMetric, 1)
}

func TestErrorTelemetryAPICallFailure(t *testing.T) {
	agentName, reqBody, reqHeaders := "dummyName", []byte(`foobar`), map[string][]string{"Key": []string{"VAL1", "VAL2"}}
	apiError := errors.New("Error calling Telemetry API: connection refused")
	serverErrMetric := telemetry.SubscribeServerErr

	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	agent, err := registrationService.CreateExternalAgent(agentName)
	assert.NoError(t, err)

	telemetrySubscription := &mockSubscriptionAPI{}
	telemetrySubscription.On("Subscribe", agentName, bytes.NewReader(reqBody), reqHeaders, mock.MatchedBy(validIPPort)).Return([]byte(``), http.StatusOK, map[string][]string{}, apiError)
	telemetrySubscription.On("RecordCounterMetric", serverErrMetric, 1)

	handler := NewRuntimeTelemetrySubscriptionHandler(registrationService, telemetrySubscription)
	request := httptest.NewRequest("PUT", "/", bytes.NewBuffer(reqBody))
	for k, vals := range reqHeaders {
		for _, v := range vals {
			request.Header.Add(k, v)
		}
	}

	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)

	recordedBody, err := io.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)

	expectedErrorBody := `{"errorMessage":"Internal Server Error","errorType":"InternalServerError"}` + "\n"
	expectedHeaders := http.Header(map[string][]string{"Content-Type": []string{"application/json"}})

	assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	assert.Equal(t, expectedErrorBody, string(recordedBody))
	assert.Equal(t, expectedHeaders, responseRecorder.Header())
	telemetrySubscription.AssertCalled(t, "RecordCounterMetric", serverErrMetric, 1)
}

func TestRenderLogsSubscriptionClosed(t *testing.T) {
	agentName, reqBody, reqHeaders := "dummyName", []byte(`foobar`), map[string][]string{"Key": []string{"VAL1", "VAL2"}}
	apiError := telemetry.ErrTelemetryServiceOff
	clientErrMetric := telemetry.SubscribeClientErr

	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	agent, err := registrationService.CreateExternalAgent(agentName)
	assert.NoError(t, err)

	telemetrySubscription := &mockSubscriptionAPI{}
	telemetrySubscription.On("Subscribe", agentName, bytes.NewReader(reqBody), reqHeaders, mock.MatchedBy(validIPPort)).Return([]byte(``), http.StatusOK, map[string][]string{}, apiError)
	telemetrySubscription.On("RecordCounterMetric", clientErrMetric, 1)
	telemetrySubscription.On("GetServiceClosedErrorMessage").Return("Logs API subscription is closed already")
	telemetrySubscription.On("GetServiceClosedErrorType").Return("Logs.SubscriptionClosed")

	handler := NewRuntimeTelemetrySubscriptionHandler(registrationService, telemetrySubscription)
	request := httptest.NewRequest("PUT", "/", bytes.NewBuffer(reqBody))
	for k, vals := range reqHeaders {
		for _, v := range vals {
			request.Header.Add(k, v)
		}
	}

	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)

	recordedBody, err := io.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)

	expectedErrorBody := `{"errorMessage":"Logs API subscription is closed already","errorType":"Logs.SubscriptionClosed"}` + "\n"
	expectedHeaders := http.Header(map[string][]string{"Content-Type": []string{"application/json"}})

	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assert.Equal(t, expectedErrorBody, string(recordedBody))
	assert.Equal(t, expectedHeaders, responseRecorder.Header())
	telemetrySubscription.AssertCalled(t, "RecordCounterMetric", clientErrMetric, 1)
}

func TestRenderTelemetrySubscriptionClosed(t *testing.T) {
	agentName, reqBody, reqHeaders := "dummyName", []byte(`foobar`), map[string][]string{"Key": []string{"VAL1", "VAL2"}}
	apiError := telemetry.ErrTelemetryServiceOff
	clientErrMetric := telemetry.SubscribeClientErr

	registrationService := core.NewRegistrationService(
		core.NewInitFlowSynchronization(),
		core.NewInvokeFlowSynchronization(),
	)

	agent, err := registrationService.CreateExternalAgent(agentName)
	assert.NoError(t, err)

	telemetrySubscription := &mockSubscriptionAPI{}
	telemetrySubscription.On("Subscribe", agentName, bytes.NewReader(reqBody), reqHeaders, mock.MatchedBy(validIPPort)).Return([]byte(``), http.StatusOK, map[string][]string{}, apiError)
	telemetrySubscription.On("RecordCounterMetric", clientErrMetric, 1)
	telemetrySubscription.On("GetServiceClosedErrorMessage").Return("Telemetry API subscription is closed already")
	telemetrySubscription.On("GetServiceClosedErrorType").Return("Telemetry.SubscriptionClosed")

	handler := NewRuntimeTelemetrySubscriptionHandler(registrationService, telemetrySubscription)
	request := httptest.NewRequest("PUT", "/", bytes.NewBuffer(reqBody))
	for k, vals := range reqHeaders {
		for _, v := range vals {
			request.Header.Add(k, v)
		}
	}

	request = request.WithContext(context.WithValue(context.Background(), AgentIDCtxKey, agent.ID))
	responseRecorder := httptest.NewRecorder()

	handler.ServeHTTP(responseRecorder, request)

	recordedBody, err := io.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)

	expectedErrorBody := `{"errorMessage":"Telemetry API subscription is closed already","errorType":"Telemetry.SubscriptionClosed"}` + "\n"
	expectedHeaders := http.Header(map[string][]string{"Content-Type": []string{"application/json"}})

	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
	assert.Equal(t, expectedErrorBody, string(recordedBody))
	assert.Equal(t, expectedHeaders, responseRecorder.Header())
	telemetrySubscription.AssertCalled(t, "RecordCounterMetric", clientErrMetric, 1)
}
