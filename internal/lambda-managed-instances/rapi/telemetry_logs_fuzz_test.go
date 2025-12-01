// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapi

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/testdata"
)

const (
	telemetryHandlerPath = "/telemetry"

	samplePayload = `{"foo" : "bar"}`
)

func FuzzTelemetryLogRouters(f *testing.F) {
	f.Add(makeTargetURL(telemetryHandlerPath, version20220701), []byte(samplePayload))
	f.Add(makeTargetURL("/telemetry#", version20220701), []byte(samplePayload))
	f.Add(makeTargetURL("/telemetry#fragment", version20220701), []byte(samplePayload))

	telemetryPath := fmt.Sprintf("%s%s", version20220701, telemetryHandlerPath)

	f.Fuzz(func(t *testing.T, rawPath string, payload []byte) {
		u, err := parseToURLStruct(rawPath)
		if err != nil {
			t.Skipf("error parsing url: %v. Skipping test.", err)
		}

		flowTest := testdata.NewFlowTest()

		rapiServer := makeRapiServerWithMockSubscriptionAPI(flowTest, newMockSubscriptionAPI(true))

		request := httptest.NewRequest("PUT", rawPath, bytes.NewReader(payload))
		responseRecorder := serveTestRequest(rapiServer, request)

		if u.Path == telemetryPath && u.Fragment == "" {
			assertExpectedPathResponseCode(t, responseRecorder.Code, rawPath)
		} else {
			assertUnexpectedPathResponseCode(t, responseRecorder.Code, rawPath)
		}
	})
}

func FuzzTelemetryHandler(f *testing.F) {
	fuzzSubscriptionAPIHandler(f, telemetryHandlerPath, version20220701)
}

func fuzzSubscriptionAPIHandler(f *testing.F, handlerPath string, apiVersion string) {
	flowTest := testdata.NewFlowTest()
	agent := makeExternalAgent(flowTest.RegistrationService)
	f.Add([]byte(samplePayload), agent.ID().String(), true)
	f.Add([]byte(samplePayload), agent.ID().String(), false)

	f.Fuzz(func(t *testing.T, payload []byte, agentIdentifierHeader string, serviceOn bool) {
		telemetrySubscriptionAPI := newMockSubscriptionAPI(serviceOn)
		rapiServer := makeRapiServerWithMockSubscriptionAPI(flowTest, telemetrySubscriptionAPI)

		target := makeTargetURL(handlerPath, apiVersion)
		request := httptest.NewRequest("PUT", target, bytes.NewReader(payload))
		request.Header.Set(model.LambdaAgentIdentifier, agentIdentifierHeader)

		responseRecorder := serveTestRequest(rapiServer, request)

		if agentIdentifierHeader == "" {
			assertForbiddenErrorType(t, responseRecorder, model.ErrAgentIdentifierMissing)
			return
		}

		if _, err := uuid.Parse(agentIdentifierHeader); err != nil {
			assertForbiddenErrorType(t, responseRecorder, model.ErrAgentIdentifierInvalid)
			return
		}

		if agentIdentifierHeader != agent.ID().String() {
			assertForbiddenErrorType(t, responseRecorder, "Extension.UnknownExtensionIdentifier")
			return
		}

		if !serviceOn {
			assertForbiddenErrorType(t, responseRecorder, telemetrySubscriptionAPI.GetServiceClosedErrorType())
			return
		}

		assert.Equal(t, payload, telemetrySubscriptionAPI.receivedPayload)
	})
}

func makeRapiServerWithMockSubscriptionAPI(
	flowTest *testdata.FlowTest,
	telemetrySubscription telemetry.SubscriptionAPI,
) *Server {
	s, err := NewServer(
		netip.MustParseAddrPort("127.0.0.1:0"),
		flowTest.AppCtx,
		flowTest.RegistrationService,
		flowTest.RenderingService,
		telemetrySubscription,
		nil,
	)
	if err != nil {
		panic(err)
	}
	return s
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
	return interop.TelemetrySubscriptionMetrics{}
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

func (m *mockSubscriptionAPI) Configure(passphrase string, addr netip.AddrPort) {}
