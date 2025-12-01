// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build test

package test

import (
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/aws-lambda-rie/internal"
	rmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/testutils"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/testutils/functional"
)

func TestRie_SingleInvoke(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		expectedStatus  int
		expectedPayload string
		expectedHeaders map[string]string
		runtimeEnv      functional.RuntimeEnv
	}{
		{
			name:            "simple_invoke",
			expectedStatus:  http.StatusOK,
			expectedPayload: "test response",
			expectedHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			runtimeEnv: functional.RuntimeEnv{
				Workers: []functional.RuntimeExecutionEnvironment{
					{
						Actions: []functional.ExecutionEnvironmentAction{
							functional.NextAction{},
							functional.InvocationResponseAction{
								Payload:     strings.NewReader("test response"),
								ContentType: "application/json",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.runtimeEnv.T = t
			supv := functional.NewMockSupervisor(t, &tt.runtimeEnv, nil, nil)

			sigCh := make(chan os.Signal, 1)

			mockFileUtil := functional.MakeMockFileUtil(nil)

			args := []string{"--runtime-api-address", "127.0.0.1:0", "--runtime-interface-emulator-address", "127.0.0.1:0", "echo", "hello"}
			server, rieHandler, _, err := internal.Run(supv, args, mockFileUtil, sigCh)
			require.NoError(t, err)

			require.NoError(t, rieHandler.Init())

			resp, err := http.Post("http://"+server.Addr.String()+"/2015-03-31/functions/function/invocations", "application/json", strings.NewReader("{}"))
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			for key, expectedValue := range tt.expectedHeaders {
				assert.Equal(t, expectedValue, resp.Header.Get(key))
			}

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedPayload, string(body))

			tt.runtimeEnv.Done.Wait()

			sigCh <- syscall.SIGTERM
			<-server.Done()
			assert.NoError(t, server.Err())
		})
	}
}

func TestRie_SigtermDuringInvoke(t *testing.T) {
	t.Parallel()

	runtime := &functional.RuntimeEnv{
		Workers: []functional.RuntimeExecutionEnvironment{
			{
				Actions: []functional.ExecutionEnvironmentAction{
					functional.NextAction{},
				},
			},
		},
		ForcedError: nil,
		T:           t,
	}
	supv := functional.NewMockSupervisor(t, runtime, nil, nil)

	sigCh := make(chan os.Signal, 1)

	mockFileUtil := functional.MakeMockFileUtil(nil)

	args := []string{"--runtime-api-address", "127.0.0.1:0", "--runtime-interface-emulator-address", "127.0.0.1:0", "echo", "hello"}
	server, rieHandler, _, err := internal.Run(supv, args, mockFileUtil, sigCh)
	require.NoError(t, err)

	require.NoError(t, rieHandler.Init())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		resp, err := http.Post("http://"+server.Addr.String()+"/2015-03-31/functions/function/invocations", "application/json", strings.NewReader("{}"))
		require.NoError(t, err)
		defer func() { require.NoError(t, resp.Body.Close()) }()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		assert.Equal(t, "Client.ExecutionEnvironmentShutDown", resp.Header.Get("Error-Type"))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"errorType":"Client.ExecutionEnvironmentShutDown"}`, string(body))

		wg.Done()
	}()

	time.Sleep(200 * time.Millisecond)

	sigCh <- syscall.SIGTERM
	<-server.Done()
	assert.NoError(t, server.Err())

	wg.Wait()
}

func TestRie_InitError(t *testing.T) {
	t.Parallel()

	runtime := &functional.RuntimeEnv{
		Workers: []functional.RuntimeExecutionEnvironment{
			{
				Actions: []functional.ExecutionEnvironmentAction{
					functional.InitErrorAction{
						ErrorType:      "Function.TestError",
						ExpectedStatus: http.StatusAccepted,
					},
					functional.ExitAction{},
				},
			},
		},
		ForcedError: nil,
		T:           t,
	}
	supv := functional.NewMockSupervisor(t, runtime, nil, nil)

	sigCh := make(chan os.Signal, 1)

	args := []string{"--runtime-api-address", "127.0.0.1:0", "--runtime-interface-emulator-address", "127.0.0.1:0", "echo", "hello"}
	server, rieHandler, _, err := internal.Run(supv, args, functional.MakeMockFileUtil(nil), sigCh)
	require.NoError(t, err)

	initErr := rieHandler.Init()
	assert.Error(t, initErr)
	assert.Contains(t, []rmodel.ErrorType{"Function.TestError", "Runtime.ExitError"}, initErr.ErrorType())

	<-server.Done()
	serverErr := server.Err()
	assert.Error(t, serverErr)
	assert.Contains(t, []rmodel.ErrorType{"Function.TestError", "Runtime.ExitError"}, initErr.(rmodel.CustomerError).ErrorType())
}

func TestRie_InvokeWaitingForInitError(t *testing.T) {
	t.Parallel()

	runtime := &functional.RuntimeEnv{
		Workers: []functional.RuntimeExecutionEnvironment{
			{
				Actions: []functional.ExecutionEnvironmentAction{
					functional.SleepAction{Duration: 100 * time.Millisecond},
					functional.ExitAction{},
				},
			},
		},
		ForcedError: nil,
		T:           t,
	}
	supv := functional.NewMockSupervisor(t, runtime, nil, nil)

	sigCh := make(chan os.Signal, 1)

	mockFileUtil := functional.MakeMockFileUtil(nil)

	args := []string{"--runtime-api-address", "127.0.0.1:0", "--runtime-interface-emulator-address", "127.0.0.1:0", "echo", "hello"}
	server, rieHandler, _, err := internal.Run(supv, args, mockFileUtil, sigCh)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		resp, err := http.Post("http://"+server.Addr.String()+"/2015-03-31/functions/function/invocations", "application/json", strings.NewReader("{}"))
		require.NoError(t, err)
		defer func() { require.NoError(t, resp.Body.Close()) }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		expectedHeaders := map[string]string{
			"Content-Type": "application/json",
			"Error-Type":   "Runtime.ExitError",
		}
		for key, expectedValue := range expectedHeaders {
			assert.Equal(t, expectedValue, resp.Header.Get(key))
		}

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"errorType":"Runtime.ExitError"}`, string(body))

		wg.Done()
	}()

	time.Sleep(200 * time.Millisecond)
	initErr := rieHandler.Init()
	require.Error(t, initErr)

	<-server.Done()
	serverErr := server.Err()
	assert.Error(t, serverErr)
	assert.Equal(t, initErr, serverErr)

	wg.Wait()
}

func TestRie_InvokeFatalError(t *testing.T) {
	t.Parallel()

	runtime := &functional.RuntimeEnv{
		Workers: []functional.RuntimeExecutionEnvironment{
			{
				Actions: []functional.ExecutionEnvironmentAction{
					functional.NextAction{},
					functional.ExitAction{},
				},
			},
		},
		ForcedError: nil,
		T:           t,
	}
	supv := functional.NewMockSupervisor(t, runtime, nil, nil)

	sigCh := make(chan os.Signal, 1)

	mockFileUtil := functional.MakeMockFileUtil(nil)

	args := []string{"--runtime-api-address", "127.0.0.1:0", "--runtime-interface-emulator-address", "127.0.0.1:0", "echo", "hello"}
	server, rieHandler, _, err := internal.Run(supv, args, mockFileUtil, sigCh)
	require.NoError(t, err)

	require.NoError(t, rieHandler.Init())

	resp, err := http.Post("http://"+server.Addr.String()+"/2015-03-31/functions/function/invocations", "application/json", strings.NewReader("{}"))
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	assert.Equal(t, "Runtime.ExitError", resp.Header.Get("Error-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"errorType":"Runtime.ExitError"}`, string(body))

	<-server.Done()
	assert.Error(t, server.Err())
}

func TestRIE_TelemetryAPI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		expectedStatus  int
		expectedPayload string
		expectedHeaders map[string]string
		runtimeEnv      functional.RuntimeEnv
		extensionEnv    functional.ExtensionsEnv
	}{
		{
			name:            "simple_invoke",
			expectedStatus:  http.StatusOK,
			expectedPayload: "test response",
			expectedHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			runtimeEnv: functional.RuntimeEnv{
				Workers: []functional.RuntimeExecutionEnvironment{
					{
						Actions: []functional.ExecutionEnvironmentAction{
							functional.NextAction{},
							functional.StdoutAction{Payload: "runtime: test stdout log\n"},
							functional.StderrAction{Payload: "runtime: test stderr log\n"},
							functional.InvocationResponseAction{
								Payload:     strings.NewReader("test response"),
								ContentType: "application/json",
							},
						},
					},
				},
			},
			extensionEnv: functional.ExtensionsEnv{
				"http": &functional.ExtensionsExecutionEnvironment{
					Actions: []functional.ExecutionEnvironmentAction{
						functional.ExtensionsRegisterAction{
							AgentUniqueName: "http",
							ExpectedStatus:  http.StatusOK,
						},
						functional.ExtensionsTelemetryAPIHTTPSubscriberAction{
							InMemoryEventsApi: functional.NewInMemoryEventsApi(t),
							Subscription: functional.ExtensionTelemetrySubscribeAction{
								AgentName:      "http",
								ExpectedStatus: http.StatusOK,
							},
						},
						functional.StdoutAction{Payload: "extension http: test stdout log\n"},
						functional.StderrAction{Payload: "extension http: test stderr log\n"},
						functional.ExtensionsNextAction{
							ExpectedStatus: http.StatusOK,
						},
					},
				},
				"tcp": &functional.ExtensionsExecutionEnvironment{
					Actions: []functional.ExecutionEnvironmentAction{
						functional.ExtensionsRegisterAction{
							AgentUniqueName: "tcp",
							ExpectedStatus:  http.StatusOK,
						},
						functional.ExtensionsTelemetryAPITCPSubscriberAction{
							InMemoryEventsApi: functional.NewInMemoryEventsApi(t),
							Subscription: functional.ExtensionTelemetrySubscribeAction{
								AgentName:      "tcp",
								ExpectedStatus: http.StatusOK,
							},
						},
						functional.StdoutAction{Payload: "extension tcp: test stdout log\n"},
						functional.StderrAction{Payload: "extension tcp: test stderr log\n"},
						functional.ExtensionsNextAction{
							ExpectedStatus: http.StatusOK,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.runtimeEnv.T = t
			for _, ext := range tt.extensionEnv {
				ext.T = t
			}

			var httpEventsApi, tcpEventsApi *functional.InMemoryEventsApi

			if httpExt, ok := tt.extensionEnv["http"]; ok {
				for _, action := range httpExt.Actions {
					if httpAction, ok := action.(functional.ExtensionsTelemetryAPIHTTPSubscriberAction); ok {
						httpEventsApi = httpAction.InMemoryEventsApi
						break
					}
				}
			}
			if tcpExt, ok := tt.extensionEnv["tcp"]; ok {
				for _, action := range tcpExt.Actions {
					if tcpAction, ok := action.(functional.ExtensionsTelemetryAPITCPSubscriberAction); ok {
						tcpEventsApi = tcpAction.InMemoryEventsApi
						break
					}
				}
			}

			supv := functional.NewMockSupervisor(t, &tt.runtimeEnv, tt.extensionEnv, nil)

			sigCh := make(chan os.Signal, 1)

			mockFileUtil := functional.MakeMockFileUtil(tt.extensionEnv)

			args := []string{"--runtime-api-address", "127.0.0.1:0", "--runtime-interface-emulator-address", "127.0.0.1:0", "echo", "hello"}
			server, rieHandler, _, err := internal.Run(supv, args, mockFileUtil, sigCh)
			require.NoError(t, err)

			initStartTime := time.Now()
			require.NoError(t, rieHandler.Init())
			initFinishTime := time.Now()

			req, err := http.NewRequest(http.MethodPost, "http://"+server.Addr.String()+"/2015-03-31/functions/function/invocations", strings.NewReader("{}"))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			invokeID := uuid.NewString()
			req.Header.Set("x-amzn-RequestId", invokeID)

			invokeStartTime := time.Now()
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			for key, expectedValue := range tt.expectedHeaders {
				assert.Equal(t, expectedValue, resp.Header.Get(key))
			}

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedPayload, string(body))

			invokeFinishTime := time.Now()

			sigCh <- syscall.SIGTERM
			<-server.Done()
			assert.NoError(t, server.Err())

			initPayload := testutils.MakeInitPayload()

			expectedInitEvents := []functional.ExpectedInitEvent{
				{
					EventType: functional.PlatformInitStart,
					Status:    "success",
				},
				{
					EventType: functional.PlatformInitRuntimeDone,
					Status:    "success",
				},
				{
					EventType: functional.PlatformInitReport,
					Status:    "success",
				},
			}

			expectedExtensionEvents := []functional.ExpectedExtensionEvents{
				{
					ExtensionName: "http",
					State:         "Ready",
				},
				{
					ExtensionName: "tcp",
					State:         "Ready",
				},
			}

			expectedInvokeEvents := []functional.ExpectedInvokeEvents{
				{
					EventType: functional.PlatformRuntimeStart,
				},
				{
					EventType: functional.PlatformReport,
					Status:    "success",
					Spans:     []string{"responseLatency", "responseDuration"},
				},
			}

			for _, mock := range []*functional.InMemoryEventsApi{httpEventsApi, tcpEventsApi} {
				mock.CheckSimpleInitExpectations(initStartTime, initFinishTime, expectedInitEvents, initPayload)
				mock.CheckSimpleExtensionExpectations(expectedExtensionEvents)
				mock.CheckSimpleInvokeExpectations(invokeStartTime, invokeFinishTime, invokeID, expectedInvokeEvents, initPayload)
				assert.Len(t, mock.LogLines(), 6)
			}
		})
	}
}
