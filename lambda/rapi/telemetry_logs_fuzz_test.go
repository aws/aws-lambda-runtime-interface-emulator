// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapi

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.amzn.com/lambda/extensions"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi/handler"
	"go.amzn.com/lambda/telemetry"
	"go.amzn.com/lambda/testdata"
)

const (
	logsHandlerPath      = "/logs"
	telemetryHandlerPath = "/telemetry"

	samplePayload = `{"foo" : "bar"}`
)

func FuzzTelemetryLogRouters(f *testing.F) {
	extensions.Enable()
	defer extensions.Disable()

	f.Add(makeTargetURL(logsHandlerPath, version20200815), []byte(samplePayload))
	f.Add(makeTargetURL(telemetryHandlerPath, version20220701), []byte(samplePayload))

	logsPath := fmt.Sprintf("%s%s", version20200815, logsHandlerPath)
	telemetryPath := fmt.Sprintf("%s%s", version20220701, telemetryHandlerPath)

	f.Fuzz(func(t *testing.T, rawPath string, payload []byte) {
		u, err := parseToURLStruct(rawPath)
		if err != nil {
			t.Skipf("error parsing url: %v. Skipping test.", err)
		}

		flowTest := testdata.NewFlowTest()

		rapiServer := makeRapiServerWithMockSubscriptionAPI(flowTest, newMockSubscriptionAPI(true), newMockSubscriptionAPI(true))

		request := httptest.NewRequest("PUT", rawPath, bytes.NewReader(payload))
		responseRecorder := serveTestRequest(rapiServer, request)

		if u.Path == logsPath || u.Path == telemetryPath {
			assertExpectedPathResponseCode(t, responseRecorder.Code, rawPath)
		} else {
			assertUnexpectedPathResponseCode(t, responseRecorder.Code, rawPath)
		}
	})
}

func FuzzLogsHandler(f *testing.F) {
	extensions.Enable()
	defer extensions.Disable()

	fuzzSubscriptionAPIHandler(f, logsHandlerPath, version20200815)
}

func FuzzTelemetryHandler(f *testing.F) {
	extensions.Enable()
	defer extensions.Disable()

	fuzzSubscriptionAPIHandler(f, telemetryHandlerPath, version20220701)
}

func fuzzSubscriptionAPIHandler(f *testing.F, handlerPath string, apiVersion string) {
	flowTest := testdata.NewFlowTest()
	agent := makeExternalAgent(flowTest.RegistrationService)
	f.Add([]byte(samplePayload), agent.ID.String(), true)
	f.Add([]byte(samplePayload), agent.ID.String(), false)

	f.Fuzz(func(t *testing.T, payload []byte, agentIdentifierHeader string, serviceOn bool) {
		telemetrySubscriptionAPI := newMockSubscriptionAPI(serviceOn)
		logsSubscriptionAPI := newMockSubscriptionAPI(serviceOn)
		rapiServer := makeRapiServerWithMockSubscriptionAPI(flowTest, logsSubscriptionAPI, telemetrySubscriptionAPI)

		apiUnderTest := telemetrySubscriptionAPI
		if handlerPath == logsHandlerPath {
			apiUnderTest = logsSubscriptionAPI
		}

		target := makeTargetURL(handlerPath, apiVersion)
		request := httptest.NewRequest("PUT", target, bytes.NewReader(payload))
		request.Header.Set(handler.LambdaAgentIdentifier, agentIdentifierHeader)

		responseRecorder := serveTestRequest(rapiServer, request)

		if agentIdentifierHeader == "" {
			assertForbiddenErrorType(t, responseRecorder, handler.ErrAgentIdentifierMissing)
			return
		}

		if _, err := uuid.Parse(agentIdentifierHeader); err != nil {
			assertForbiddenErrorType(t, responseRecorder, handler.ErrAgentIdentifierInvalid)
			return
		}

		if agentIdentifierHeader != agent.ID.String() {
			assertForbiddenErrorType(t, responseRecorder, "Extension.UnknownExtensionIdentifier")
			return
		}

		if !serviceOn {
			assertForbiddenErrorType(t, responseRecorder, apiUnderTest.GetServiceClosedErrorType())
			return
		}

		// assert that payload has been stored in the mock subscription api after the handler calls Subscribe()
		assert.Equal(t, payload, apiUnderTest.receivedPayload)
	})
}

func makeRapiServerWithMockSubscriptionAPI(
	flowTest *testdata.FlowTest,
	logsSubscription telemetry.SubscriptionAPI,
	telemetrySubscription telemetry.SubscriptionAPI) *Server {
	return NewServer(
		"127.0.0.1",
		0,
		flowTest.AppCtx,
		flowTest.RegistrationService,
		flowTest.RenderingService,
		true,
		logsSubscription,
		telemetrySubscription,
		flowTest.CredentialsService,
	)
}

type mockSubscriptionAPI struct {
	serviceOn       bool
	receivedPayload []byte
}

func newMockSubscriptionAPI(serviceOn bool) *mockSubscriptionAPI {
	return &mockSubscriptionAPI{
		serviceOn: serviceOn,
	}
}

func (m *mockSubscriptionAPI) Subscribe(agentName string, body io.Reader, headers map[string][]string, remoteAddr string) ([]byte, int, map[string][]string, error) {
	if !m.serviceOn {
		return nil, 0, map[string][]string{}, telemetry.ErrTelemetryServiceOff
	}

	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, 0, map[string][]string{}, fmt.Errorf("error Reading the body of subscription request: %s", err)
	}

	m.receivedPayload = bodyBytes

	return []byte("OK"), http.StatusOK, map[string][]string{}, nil
}

func (m *mockSubscriptionAPI) RecordCounterMetric(metricName string, count int) {}

func (m *mockSubscriptionAPI) FlushMetrics() interop.TelemetrySubscriptionMetrics {
	return nil
}

func (m *mockSubscriptionAPI) Clear() {}

func (m *mockSubscriptionAPI) TurnOff() {}

func (m *mockSubscriptionAPI) GetEndpointURL() string {
	return "/subscribe"
}

func (m *mockSubscriptionAPI) GetServiceClosedErrorMessage() string {
	return "Subscription API is closed"
}

func (m *mockSubscriptionAPI) GetServiceClosedErrorType() string {
	return "SubscriptionClosed"
}
