// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build test

package functional

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
)

type SleepExtensionAction struct {
	Duration time.Duration
}

func (a SleepExtensionAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	if a.Duration == 0 {
		a.Duration = 100 * time.Millisecond
	}
	t.Logf("Extensions sleeping for %v\n", a.Duration)
	time.Sleep(a.Duration)
	return nil, nil
}

func (a SleepExtensionAction) ValidateStatus(t *testing.T, resp *http.Response) {}

func (a SleepExtensionAction) String() string {
	return fmt.Sprintf("Extensions: Sleep(duration=%v)", a.Duration)
}

type ExtensionsRegisterAction struct {
	AgentUniqueName string

	Events         []Event
	ExpectedStatus int
}

func (a ExtensionsRegisterAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	resp, err := client.ExtensionsRegister(a.AgentUniqueName, a.Events)
	require.NoError(t, err)
	return resp, nil
}

func (a ExtensionsRegisterAction) ValidateStatus(t *testing.T, resp *http.Response) {
	if a.ExpectedStatus != 0 {
		if resp != nil {
			assert.Equal(t, a.ExpectedStatus, resp.StatusCode, "ExtensionsRegisterAction expected status code %d", a.ExpectedStatus)
		}
	}
}

func (a ExtensionsRegisterAction) String() string {
	return fmt.Sprintf("Extensions: Register(agentUniqueName=%s, events=%v)", a.AgentUniqueName, a.Events)
}

type ExtensionsNextAction struct {
	AgentIdentifier string
	ExpectedStatus  int
}

func (a ExtensionsNextAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	_, err := client.ExtensionsNext(a.AgentIdentifier)
	if err != nil {
		require.True(t, errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, io.EOF))
	}
	return nil, nil
}

func (a ExtensionsNextAction) ValidateStatus(t *testing.T, resp *http.Response) {
	if a.ExpectedStatus == 0 {
		a.ExpectedStatus = http.StatusOK
	}
	if resp != nil {
		assert.Equal(t, a.ExpectedStatus, resp.StatusCode, "ExtensionsNextAction expected status code %d", a.ExpectedStatus)
	}
}

func (a ExtensionsNextAction) String() string {
	return "Extensions: Next()"
}

type ExtensionsNextParallelAction struct {
	AgentIdentifier string
	ExpectedStatus  int
	ParallelActions []ExecutionEnvironmentAction
	Environment     *ExtensionsExecutionEnvironment
}

func (a ExtensionsNextParallelAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	if a.Environment != nil && len(a.ParallelActions) > 0 {
		go func() {
			tempEnv := *a.Environment
			tempEnv.Actions = a.ParallelActions
			tempEnv.executeEnvActions(client, t)
		}()
	}

	_, err := client.ExtensionsNext(a.AgentIdentifier)
	assert.NotNil(t, err)
	return nil, nil
}

func (a ExtensionsNextParallelAction) ValidateStatus(t *testing.T, resp *http.Response) {
	if a.ExpectedStatus == 0 {
		a.ExpectedStatus = http.StatusOK
	}
	if resp != nil {
		assert.Equal(t, a.ExpectedStatus, resp.StatusCode, "ExtensionsNextParallelAction expected status code %d", a.ExpectedStatus)
	}
}

func (a ExtensionsNextParallelAction) String() string {
	return "Extensions: Next() with other parallel actions"
}

type ExtensionsInitErrorAction struct {
	AgentIdentifier   string
	FunctionErrorType string
	Payload           string
	ExpectedStatus    int
}

func (a ExtensionsInitErrorAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	_, err := client.ExtensionsInitError(a.AgentIdentifier, a.FunctionErrorType, a.Payload)
	require.NoError(t, err)
	return nil, nil
}

func (a ExtensionsInitErrorAction) ValidateStatus(t *testing.T, resp *http.Response) {
	if a.ExpectedStatus != 0 {
		if resp != nil {
			assert.Equal(t, a.ExpectedStatus, resp.StatusCode, "ExtensionsInitErrorAction expected status code %d", a.ExpectedStatus)
		}
	}
}

func (a ExtensionsInitErrorAction) String() string {
	return "Extensions: InitError()"
}

type ExtensionsExitErrorAction struct {
	AgentIdentifier   string
	FunctionErrorType string
	Payload           string
	ExpectedStatus    int
}

func (a ExtensionsExitErrorAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	_, err := client.ExtensionsExitError(a.AgentIdentifier, a.FunctionErrorType, a.Payload)
	require.NoError(t, err)
	return nil, nil
}

func (a ExtensionsExitErrorAction) ValidateStatus(t *testing.T, resp *http.Response) {
	if a.ExpectedStatus != 0 {
		if resp != nil {
			assert.Equal(t, a.ExpectedStatus, resp.StatusCode, "ExtensionsExitErrorAction expected status code %d", a.ExpectedStatus)
		}
	}
}

func (a ExtensionsExitErrorAction) String() string {
	return fmt.Sprintf("Extensions: ExitError(type=%s)", a.FunctionErrorType)
}

type ExtensionsTelemetryAPIHTTPSubscriberAction struct {
	addrPort          netip.AddrPort
	Subscription      ExtensionTelemetrySubscribeAction
	InMemoryEventsApi *InMemoryEventsApi
}

func (a ExtensionsTelemetryAPIHTTPSubscriberAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	var mu sync.Mutex
	go func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		mu.Lock()
		a.addrPort = netip.MustParseAddrPort(listener.Addr().String())
		mu.Unlock()
		handler := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			slog.Info(string(body))

			var events []telemetry.Event
			require.NoError(t, json.Unmarshal(body, &events))

			for _, event := range events {
				parseTelemetryEvent(t, event, a.InMemoryEventsApi)
			}
		}))
		require.NoError(t, http.Serve(listener, handler))
	}()

	for {
		mu.Lock()
		address := a.addrPort.String()
		mu.Unlock()
		conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	a.Subscription.Payload = strings.NewReader(fmt.Sprintf(`{"schemaVersion": "2025-01-29", "destination":{"protocol":"HTTP","URI":"http://sandbox.localdomain:%d"}, "types": ["platform", "function", "extension"]}`, a.addrPort.Port()))
	return a.Subscription.Execute(t, client)
}

func (a ExtensionsTelemetryAPIHTTPSubscriberAction) ValidateStatus(t *testing.T, resp *http.Response) {
	a.Subscription.ValidateStatus(t, resp)
}

func (a ExtensionsTelemetryAPIHTTPSubscriberAction) String() string {
	return "Extensions: ExtensionsTelemetryAPIHTTPSubscriberAction"
}

type ExtensionsTelemetryAPITCPSubscriberAction struct {
	addrPort          netip.AddrPort
	Subscription      ExtensionTelemetrySubscribeAction
	InMemoryEventsApi *InMemoryEventsApi
}

func (a ExtensionsTelemetryAPITCPSubscriberAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	var mu sync.Mutex
	go func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		mu.Lock()
		a.addrPort = netip.MustParseAddrPort(listener.Addr().String())
		mu.Unlock()
		for {
			conn, err := listener.Accept()
			require.NoError(t, err)
			go func() {
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					line := scanner.Bytes()
					t.Log(string(line))

					var event telemetry.Event
					require.NoError(t, json.Unmarshal(line, &event))
					parseTelemetryEvent(t, event, a.InMemoryEventsApi)
				}
				require.NoError(t, scanner.Err())
			}()
		}
	}()

	for {
		mu.Lock()
		address := a.addrPort.String()
		mu.Unlock()
		conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	a.Subscription.Payload = strings.NewReader(fmt.Sprintf(`{"schemaVersion": "2025-01-29", "destination":{"protocol":"TCP","port":%d}, "types": ["platform", "function", "extension"]}`, a.addrPort.Port()))
	return a.Subscription.Execute(t, client)
}

func (a ExtensionsTelemetryAPITCPSubscriberAction) ValidateStatus(t *testing.T, resp *http.Response) {
	a.Subscription.ValidateStatus(t, resp)
}

func (a ExtensionsTelemetryAPITCPSubscriberAction) String() string {
	return "Extensions: ExtensionsTelemetryAPITCPSubscriberAction"
}

type ExtensionTelemetrySubscribeAction struct {
	AgentIdentifier string
	AgentName       string

	Payload io.Reader

	Headers map[string][]string

	RemoteAddr string

	ExpectedStatus int

	ExpectedErrorType string

	ExpectedErrorMessage string
}

func (a ExtensionTelemetrySubscribeAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	resp, err := client.ExtensionsTelemetrySubscribe(
		a.AgentIdentifier,
		a.AgentName,
		a.Payload,
		a.Headers,
		a.RemoteAddr,
	)

	require.NoError(t, err)
	return resp, nil
}

func (a ExtensionTelemetrySubscribeAction) ValidateStatus(t *testing.T, resp *http.Response) {

	defer func() { require.NoError(t, resp.Body.Close()) }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read resp body")

	if a.ExpectedStatus != 0 {
		if resp != nil {
			assert.Equal(t, a.ExpectedStatus, resp.StatusCode, "ExtensionTelemetrySubscribeAction expected status code %d, body: %s", a.ExpectedStatus, string(body))
		}
	}

	if resp.StatusCode >= 400 {
		var errorResp model.ErrorResponse
		err = json.Unmarshal(body, &errorResp)
		require.NoError(t, err, "failed to unmarshal resp body")

		if a.ExpectedErrorMessage != "" {
			assert.Equal(t, a.ExpectedErrorMessage, errorResp.ErrorMessage, "ExtensionTelemetrySubscribeAction expected error message %s", a.ExpectedErrorMessage)
		}

		if a.ExpectedErrorType != "" {
			assert.Equal(t, a.ExpectedErrorType, errorResp.ErrorType, "ExtensionTelemetrySubscribeAction expected error type %s", a.ExpectedErrorType)
		}
	}
}

func (a ExtensionTelemetrySubscribeAction) String() string {
	return fmt.Sprintf("Extensions: TelemetrySubscribe(agentName=%s)", a.AgentName)
}

func parseTelemetryEvent(t *testing.T, event telemetry.Event, eventsApi *InMemoryEventsApi) {
	if eventsApi == nil {
		return
	}

	switch event.Type {
	case "platform.initStart":
		var data interop.InitStartData
		require.NoError(t, json.Unmarshal(event.Record, &data))
		require.NoError(t, eventsApi.SendInitStart(data))

	case "platform.initRuntimeDone":
		var data interop.InitRuntimeDoneData
		require.NoError(t, json.Unmarshal(event.Record, &data))
		require.NoError(t, eventsApi.SendInitRuntimeDone(data))

	case "platform.initReport":
		var data interop.InitReportData
		require.NoError(t, json.Unmarshal(event.Record, &data))
		require.NoError(t, eventsApi.SendInitReport(data))

	case "platform.start":
		var data interop.InvokeStartData
		require.NoError(t, json.Unmarshal(event.Record, &data))
		require.NoError(t, eventsApi.SendInvokeStart(data))

	case "platform.report":
		var data interop.ReportData
		require.NoError(t, json.Unmarshal(event.Record, &data))
		require.NoError(t, eventsApi.SendReport(data))

	case "platform.extension":
		var data interop.ExtensionInitData
		require.NoError(t, json.Unmarshal(event.Record, &data))
		require.NoError(t, eventsApi.SendExtensionInit(data))
	case "function", "extension":
		eventsApi.RecordLogLine(event)
	default:

		t.Logf("Received telemetry event of type %s (not forwarded to EventsAPI)", event.Type)
	}
}
