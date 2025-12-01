// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	internalmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapidcore/env"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/supervisor/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils"
)

func TestGetExtensionNamesWithNoExtensions(t *testing.T) {
	rs := core.NewRegistrationService(nil)

	c := &rapidContext{
		registrationService: rs,
	}

	assert.Equal(t, "", c.GetExtensionNames())
}

func TestGetExtensionNamesWithMultipleExtensions(t *testing.T) {
	rs := core.NewRegistrationService(nil)
	_, _ = rs.CreateExternalAgent("Example1")
	_, _ = rs.CreateInternalAgent("Example2")
	_, _ = rs.CreateExternalAgent("Example3")
	_, _ = rs.CreateInternalAgent("Example4")

	c := &rapidContext{
		registrationService: rs,
	}

	r := regexp.MustCompile(`^(Example\d;){3}(Example\d)$`)
	assert.True(t, r.MatchString(c.GetExtensionNames()))
}

func TestGetExtensionNamesWithTooManyExtensions(t *testing.T) {
	rs := core.NewRegistrationService(nil)
	for i := 10; i < 60; i++ {
		_, _ = rs.CreateExternalAgent("E" + strconv.Itoa(i))
	}

	c := &rapidContext{
		registrationService: rs,
	}

	output := c.GetExtensionNames()

	r := regexp.MustCompile(`^(E\d\d;){30}(E\d\d)$`)
	assert.LessOrEqual(t, len(output), maxExtensionNamesLength)
	assert.True(t, r.MatchString(output))
}

func TestGetExtensionNamesWithTooLongExtensionName(t *testing.T) {
	rs := core.NewRegistrationService(nil)
	for i := 10; i < 60; i++ {
		_, _ = rs.CreateExternalAgent(strings.Repeat("E", 130))
	}

	c := &rapidContext{
		registrationService: rs,
	}

	assert.Equal(t, "", c.GetExtensionNames())
}

func makeRapidTestEnv() (runtimeEnv internalmodel.KVMap, extensionEnv internalmodel.KVMap) {

	config := &internalmodel.InitRequestMessage{
		TaskName:        "runtime",
		AwsRegion:       "platform",
		AwsKey:          "creds",
		AwsSecret:       "creds",
		AwsSession:      "creds",
		FunctionVersion: "platform",
		MemorySizeBytes: 3 * 1024 * 1024,
		EnvVars:         make(map[string]string),
	}

	return env.SetupEnvironment(config, "host:port", "/path")
}

func makeFileUtils(withExtensions bool) *utils.MockFileUtil {

	mockFileUtil := &utils.MockFileUtil{}
	mockFileUtil.On("Stat", mock.Anything).Return(nil, nil)
	if withExtensions {
		mockFileUtil.On("ReadDirectory", mock.MatchedBy(func(path string) bool {
			return strings.Contains(path, "/opt/extensions")
		})).Return([]os.DirEntry{
			utils.NewMockDirEntry("NoOp.ext", false),
		}, nil)
	}

	mockFileUtil.On("ReadDirectory", mock.Anything).Return(nil, nil)
	mockFileUtil.On("IsNotExist", mock.Anything).Return(true)

	return mockFileUtil
}

func makeRapidContext(appCtx appctx.ApplicationContext, initFlow core.InitFlowSynchronization, registrationService core.RegistrationService, supervisor *processSupervisor, fileUtils *utils.MockFileUtil) (*rapidContext, internalmodel.KVMap, internalmodel.KVMap) {
	appctx.StoreInteropServer(appCtx, &interop.MockServer{})

	renderingService := rendering.NewRenderingService()

	runtime := core.NewRuntime(initFlow)

	if err := registrationService.PreregisterRuntime(runtime); err != nil {
		panic(err)
	}
	runtime.SetState(runtime.RuntimeReadyState)

	rapidCtx := &rapidContext{

		appCtx:                   appCtx,
		initFlow:                 initFlow,
		registrationService:      registrationService,
		renderingService:         renderingService,
		shutdownContext:          newShutdownContext(),
		eventsAPI:                &telemetry.NoOpEventsAPI{},
		logsEgressAPI:            &telemetry.NoOpLogsEgressAPI{},
		telemetrySubscriptionAPI: &telemetry.NoOpSubscriptionAPI{},
		processTermChan:          make(chan rapidmodel.AppError, 20),
		fileUtils:                fileUtils,
	}
	if supervisor != nil {
		rapidCtx.supervisor = *supervisor
	}

	runtimeEnv, extensionEnv := makeRapidTestEnv()
	return rapidCtx, runtimeEnv, extensionEnv
}

func TestSetupEventWatcherRuntimeErrorHandling(t *testing.T) {
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow)
	mockedProcessSupervisor := &model.MockProcessSupervisor{}
	mockedProcessSupervisor.On("Events", mock.Anything).Return(nil, fmt.Errorf("events call failed"))
	procSupv := &processSupervisor{ProcessSupervisor: mockedProcessSupervisor}
	initMetrics := &interop.MockInitMetrics{}
	defer initMetrics.AssertExpectations(t)

	rapidCtx, _, _ := makeRapidContext(appCtx, initFlow, registrationService, procSupv, makeFileUtils(false))

	sbStaticData := interop.InitExecutionData{
		TelemetrySubscriptionConfig: interop.TelemetrySubscriptionConfig{
			Passphrase: "test-passphrase",
			APIAddr:    netip.MustParseAddrPort("127.0.0.1:1234"),
		},
	}

	ctx := context.Background()
	initErr := rapidCtx.HandleInit(ctx, sbStaticData, initMetrics)
	assert.NotNil(t, initErr)

	assert.NotNil(t, "We receive a not nil error", initErr.Error())
	assert.Equal(t, rapidmodel.ErrorSeverityFatal, initErr.Severity())
	assert.Equal(t, rapidmodel.ErrorSourceSandbox, initErr.Source())
}

func TestInitTimeoutHandling(t *testing.T) {
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow)
	mockedProcessSupervisor := &model.MockProcessSupervisor{}
	eventChan := make(chan model.Event)
	mockedProcessSupervisor.On("Events", mock.Anything).Return((<-chan model.Event)(eventChan), nil)
	initMetrics := NewInitMetrics(nil)

	mockedProcessSupervisor.On("Exec", mock.Anything, mock.MatchedBy(func(execRequest *model.ExecRequest) bool {
		return execRequest.Name == "extension-NoOp.ext" &&
			execRequest.Path == "/opt/extensions/NoOp.ext"
	})).Return(nil)

	mockedProcessSupervisor.On("Exec", mock.Anything, mock.MatchedBy(func(execRequest *model.ExecRequest) bool {
		return execRequest.Name == "runtime"
	})).WaitUntil(time.After(300 * time.Millisecond)).Return(nil)
	procSupv := &processSupervisor{ProcessSupervisor: mockedProcessSupervisor}

	rapidCtx, testEnv, testEnv2 := makeRapidContext(appCtx, initFlow, registrationService, procSupv, makeFileUtils(true))

	var initTimeoutMs int64 = 200

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(initTimeoutMs)*time.Millisecond)
	defer cancel()

	_, extensionEnv := testEnv, testEnv2
	sbStaticData := interop.InitExecutionData{
		ExtensionEnv: extensionEnv,
		TelemetrySubscriptionConfig: interop.TelemetrySubscriptionConfig{
			Passphrase: "test-passphrase",
			APIAddr:    netip.MustParseAddrPort("127.0.0.1:1234"),
		},
		Runtime: rapidmodel.Runtime{
			ExecConfig: rapidmodel.RuntimeExec{
				Cmd:        make([]string, 2),
				WorkingDir: "",
				Env:        internalmodel.KVMap{"AWS_REGION": "us-west-2"},
			},
		},
	}

	initErr := rapidCtx.HandleInit(ctx, sbStaticData, initMetrics)

	assert.Equal(t, 1, initMetrics.externalExtensionCount+initMetrics.internalExtensionCount)
	assert.Equal(t, "Sandbox.Timedout", string(initErr.ErrorType()))
	assert.Equal(t, "Sandbox.Timedout: errTimeout", initErr.Error())
}

func TestRuntimeExecFailureOnPlatformError(t *testing.T) {
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow)
	mockedProcessSupervisor := &model.MockProcessSupervisor{}
	defer mockedProcessSupervisor.AssertExpectations(t)
	initMetrics := &interop.MockInitMetrics{}
	mockInitMetricsRuntimeFail(initMetrics)
	defer initMetrics.AssertExpectations(t)

	eventChan := make(chan model.Event)
	mockedProcessSupervisor.On("Events", mock.Anything).Return((<-chan model.Event)(eventChan), nil)
	err := &model.SupervisorError{
		SourceErr: model.ErrorSourceServer,
		ReasonErr: "AnyErrorReason",
		CauseErr:  "msg",
	}

	mockedProcessSupervisor.On("Exec", mock.Anything, mock.MatchedBy(func(execRequest *model.ExecRequest) bool {
		return execRequest.Name == "extension-NoOp.ext" &&
			execRequest.Path == "/opt/extensions/NoOp.ext"
	})).Return(nil)

	mockedProcessSupervisor.On("Exec", mock.Anything, mock.Anything).Return(err)

	procSupv := &processSupervisor{ProcessSupervisor: mockedProcessSupervisor}

	rapidCtx, testEnv, testEnv2 := makeRapidContext(appCtx, initFlow, registrationService, procSupv, makeFileUtils(true))

	_, extensionEnv := testEnv, testEnv2
	sbStaticData := interop.InitExecutionData{
		ExtensionEnv: extensionEnv,
		TelemetrySubscriptionConfig: interop.TelemetrySubscriptionConfig{
			Passphrase: "test-passphrase",
			APIAddr:    netip.MustParseAddrPort("127.0.0.1:1234"),
		},
		Runtime: rapidmodel.Runtime{
			ExecConfig: rapidmodel.RuntimeExec{
				Cmd:        make([]string, 2),
				WorkingDir: "",
				Env:        internalmodel.KVMap{"FOO": "BAR"},
			},
		},
	}

	ctx := context.Background()

	require.NoError(t, initFlow.SetExternalAgentsRegisterCount(1))
	require.NoError(t, initFlow.ExternalAgentRegistered())

	initErr := rapidCtx.HandleInit(ctx, sbStaticData, initMetrics)

	assert.Equal(t, rapidmodel.ErrorCategory("Invalid.Runtime"), initErr.ErrorCategory())
	assert.Equal(t, rapidmodel.ErrorRuntimeInvalidEntryPoint, initErr.ErrorType())
}

func TestRuntimeExecFailureOnCustomerError(t *testing.T) {
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow)
	mockedProcessSupervisor := &model.MockProcessSupervisor{}
	defer mockedProcessSupervisor.AssertExpectations(t)

	eventChan := make(chan model.Event)
	mockedProcessSupervisor.On("Events", mock.Anything).Return((<-chan model.Event)(eventChan), nil)

	mockedProcessSupervisor.On("Exec", mock.Anything, mock.MatchedBy(func(execRequest *model.ExecRequest) bool {
		return execRequest.Name == "extension-NoOp.ext" &&
			execRequest.Path == "/opt/extensions/NoOp.ext"
	})).Return(nil).Once()

	initMetrics := &interop.MockInitMetrics{}
	mockInitMetricsRuntimeFail(initMetrics)
	defer initMetrics.AssertExpectations(t)

	err := &model.SupervisorError{
		SourceErr: model.ErrorSourceFunction,
		ReasonErr: "AnyErrorReason",
		CauseErr:  "msg",
	}

	mockedProcessSupervisor.On("Exec", mock.Anything, mock.MatchedBy(func(exec *model.ExecRequest) bool {
		return exec.Name == "runtime"
	})).Return(err)

	procSupv := &processSupervisor{ProcessSupervisor: mockedProcessSupervisor}

	rapidCtx, testEnv, testEnv2 := makeRapidContext(appCtx, initFlow, registrationService, procSupv, makeFileUtils(true))

	_, extensionEnv := testEnv, testEnv2
	sbStaticData := interop.InitExecutionData{
		ExtensionEnv: extensionEnv,
		TelemetrySubscriptionConfig: interop.TelemetrySubscriptionConfig{
			Passphrase: "test-passphrase",
			APIAddr:    netip.MustParseAddrPort("127.0.0.1:1234"),
		},
		Runtime: rapidmodel.Runtime{
			ExecConfig: rapidmodel.RuntimeExec{
				Cmd:        make([]string, 2),
				WorkingDir: "",
				Env:        internalmodel.KVMap{"FOO": "BAR"},
			},
		},
	}

	ctx := context.Background()

	_ = initFlow.SetExternalAgentsRegisterCount(1)
	_ = initFlow.ExternalAgentRegistered()

	initErr := rapidCtx.HandleInit(ctx, sbStaticData, initMetrics)

	assert.Equal(t, "Runtime.InvalidEntrypoint", string(initErr.ErrorType()))
	assert.Equal(t, "Runtime.InvalidEntrypoint: invalid entrypoint, runtime process spawn failed", initErr.Error())
}

func TestRuntimeProcessTerminationWithDifferentCause(t *testing.T) {
	testCases := []struct {
		name              string
		TerminationCause  model.ProcessTerminationCause
		expectedErrorType rapidmodel.ErrorType
	}{
		{
			name:              "runtimeTerminationDueToOOM",
			TerminationCause:  model.OomKilled,
			expectedErrorType: rapidmodel.ErrorRuntimeOutOfMemory,
		},
		{
			name:              "runtimeTerminationWithExitCodeZero",
			TerminationCause:  model.Exited,
			expectedErrorType: rapidmodel.ErrorRuntimeExit,
		},
		{
			name:              "runtimeTerminationSiganled",
			TerminationCause:  model.Signaled,
			expectedErrorType: rapidmodel.ErrorRuntimeExit,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			appCtx := appctx.NewApplicationContext()
			initFlow := core.NewInitFlowSynchronization()

			registrationService := core.NewRegistrationService(initFlow)
			mockedProcessSupervisor := &model.MockProcessSupervisor{}
			defer mockedProcessSupervisor.AssertExpectations(t)

			runtimeEvents := make(chan model.Event)
			mockedProcessSupervisor.On("Events", mock.Anything).Return((<-chan model.Event)(runtimeEvents), nil)
			execDone := make(chan struct{})
			mockedProcessSupervisor.On("Exec", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) { execDone <- struct{}{} })

			initMetrics := &interop.MockInitMetrics{}
			mockInitMetricsRuntimeFail(initMetrics)
			defer initMetrics.AssertExpectations(t)

			go func() {
				<-execDone
				runtimeEvents <- model.Event{
					Time: time.Now().UnixMilli(),
					Event: model.EventData{
						EvType:     model.ProcessTerminationType,
						Name:       "runtime",
						Cause:      tc.TerminationCause,
						ExitStatus: new(int32),
					},
				}
			}()

			procSupv := &processSupervisor{ProcessSupervisor: mockedProcessSupervisor}

			rapidCtx, testEnv, testEnv2 := makeRapidContext(appCtx, initFlow, registrationService, procSupv, makeFileUtils(false))

			_, extensionEnv := testEnv, testEnv2
			sbStaticData := interop.InitExecutionData{
				ExtensionEnv: extensionEnv,
				TelemetrySubscriptionConfig: interop.TelemetrySubscriptionConfig{
					Passphrase: "test-passphrase",
					APIAddr:    netip.MustParseAddrPort("127.0.0.1:1234"),
				},
				Runtime: rapidmodel.Runtime{
					ExecConfig: rapidmodel.RuntimeExec{
						Cmd:        make([]string, 2),
						WorkingDir: "",
						Env:        make(internalmodel.KVMap),
					},
				},
			}

			ctx := context.Background()
			initErr := rapidCtx.HandleInit(ctx, sbStaticData, initMetrics)
			assert.Equal(t, tc.expectedErrorType, initErr.ErrorType())
		})
	}
}

func TestRuntimeExtensionTerminationWithDifferentCause(t *testing.T) {
	testCases := []struct {
		name              string
		TerminationCause  model.ProcessTerminationCause
		expectedErrorType rapidmodel.ErrorType
	}{
		{
			name:              "extensionTerminationDueToOOM",
			TerminationCause:  model.OomKilled,
			expectedErrorType: rapidmodel.ErrorAgentCrash,
		},
		{
			name:              "extensionTerminationWithExitCodeZero",
			TerminationCause:  model.Exited,
			expectedErrorType: rapidmodel.ErrorAgentCrash,
		},
		{
			name:              "extensionTerminationSiganled",
			TerminationCause:  model.Signaled,
			expectedErrorType: rapidmodel.ErrorAgentCrash,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			appCtx := appctx.NewApplicationContext()
			initFlow := core.NewInitFlowSynchronization()

			registrationService := core.NewRegistrationService(initFlow)
			mockedProcessSupervisor := &model.MockProcessSupervisor{}
			defer mockedProcessSupervisor.AssertExpectations(t)

			initMetrics := &interop.MockInitMetrics{}
			mockInitMetricsExtensionStartFailed(initMetrics)
			defer initMetrics.AssertExpectations(t)

			runtimeEvents := make(chan model.Event)
			go func() {
				runtimeEvents <- model.Event{
					Time: time.Now().UnixMilli(),
					Event: model.EventData{
						EvType:     model.ProcessTerminationType,
						Name:       "extension-example1",
						Cause:      tc.TerminationCause,
						ExitStatus: new(int32),
					},
				}
			}()
			mockedProcessSupervisor.On("Events", mock.Anything).Return((<-chan model.Event)(runtimeEvents), nil)
			mockedProcessSupervisor.On("Exec", mock.Anything, mock.Anything).Return(nil)

			procSupv := &processSupervisor{ProcessSupervisor: mockedProcessSupervisor}

			rapidCtx, testEnv, testEnv2 := makeRapidContext(appCtx, initFlow, registrationService, procSupv, makeFileUtils(true))

			_, extensionEnv := testEnv, testEnv2
			sbStaticData := interop.InitExecutionData{
				ExtensionEnv: extensionEnv,
				TelemetrySubscriptionConfig: interop.TelemetrySubscriptionConfig{
					Passphrase: "test-passphrase",
					APIAddr:    netip.MustParseAddrPort("127.0.0.1:1234"),
				},
				Runtime: rapidmodel.Runtime{
					ExecConfig: rapidmodel.RuntimeExec{
						Cmd:        make([]string, 2),
						WorkingDir: "",
						Env:        make(internalmodel.KVMap),
					},
				},
			}

			ctx := context.Background()
			initErr := rapidCtx.HandleInit(ctx, sbStaticData, initMetrics)

			assert.Equal(t, tc.expectedErrorType, initErr.ErrorType())
		})
	}
}

type MockedEventsAPI struct {
	mock.Mock
	telemetry.NoOpEventsAPI
}

var _ interop.EventsAPI = (*MockedEventsAPI)(nil)

func (m *MockedEventsAPI) SendInitReport(report interop.InitReportData) error {
	args := m.Called(report)
	return args.Error(0)
}

func (m *MockedEventsAPI) SendInitRuntimeDone(data interop.InitRuntimeDoneData) error {
	args := m.Called(data)
	return args.Error(0)
}

func (m *MockedEventsAPI) Flush() {

}

type MockInvokeResponseSender struct {
	mock.Mock
}

var _ interop.InvokeResponseSender = (*MockInvokeResponseSender)(nil)

func (m *MockInvokeResponseSender) SendResponse(invokeID string, response *interop.StreamableInvokeResponse) (*interop.InvokeResponseMetrics, error) {
	args := m.Called(invokeID, response)
	return args.Get(0).(*interop.InvokeResponseMetrics), args.Error(1)
}

func (m *MockInvokeResponseSender) SendErrorResponse(invokeID string, response *interop.ErrorInvokeResponse) (*interop.InvokeResponseMetrics, error) {
	args := m.Called(invokeID, response)
	return args.Get(0).(*interop.InvokeResponseMetrics), args.Error(1)
}

func (m *MockInvokeResponseSender) SetInvokeError(response *interop.ErrorInvokeResponse) {
	m.Called(response)
}

func (m *MockInvokeResponseSender) WaitForResponse() *interop.InvokeResponseMetrics {
	args := m.Called()
	return args.Get(0).(*interop.InvokeResponseMetrics)
}

func TestAgentCountInitResponseTimeout(t *testing.T) {
	rapidCtx, registrationService, sbStaticData, _, initMetrics := createRapidContext()

	var initTimeoutMs int64 = 50

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(initTimeoutMs)*time.Millisecond)
	defer cancel()

	numExternalExtensions := 3
	numInternalExtensions := 4

	err := registerDummyExtensions(registrationService, numInternalExtensions, numExternalExtensions)
	assert.NoError(t, err)

	initErr := rapidCtx.HandleInit(ctx, sbStaticData, initMetrics)
	assert.NotNil(t, initErr)

	assert.Equal(t, numInternalExtensions, initMetrics.internalExtensionCount)
	assert.Equal(t, numExternalExtensions, initMetrics.externalExtensionCount)
}

func TestAgentCountInitResponseFailure(t *testing.T) {
	rapidCtx, registrationService, sbStaticData, _, initMetrics := createRapidContext()

	var initTimeoutMs int64 = 50

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(initTimeoutMs)*time.Millisecond)
	defer cancel()

	numExternalExtensions := 3
	numInternalExtensions := 4

	err := registerDummyExtensions(registrationService, numInternalExtensions, numExternalExtensions)
	assert.NoError(t, err)

	registrationService.CancelFlows(fmt.Errorf("lolError"))
	initErr := rapidCtx.HandleInit(ctx, sbStaticData, initMetrics)
	assert.NotNil(t, initErr)
	assert.Equal(t, numInternalExtensions, initMetrics.internalExtensionCount)
	assert.Equal(t, numExternalExtensions, initMetrics.externalExtensionCount)
}

func TestMultipleNextFromRuntimesDuringInit(t *testing.T) {
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow)
	mockedProcessSupervisor := &model.MockProcessSupervisor{}
	defer mockedProcessSupervisor.AssertExpectations(t)

	initMetrics := &interop.MockInitMetrics{}
	mockInitMetricsFullFlow(initMetrics)
	defer initMetrics.AssertExpectations(t)

	eventChan := make(chan model.Event)
	mockedProcessSupervisor.On("Events", mock.Anything).Return((<-chan model.Event)(eventChan), nil)

	mockedProcessSupervisor.On("Exec", mock.Anything, mock.Anything).Return(nil)

	procSupv := &processSupervisor{ProcessSupervisor: mockedProcessSupervisor}

	rapidCtx, testEnv, testEnv2 := makeRapidContext(appCtx, initFlow, registrationService, procSupv, makeFileUtils(false))

	_, extensionEnv := testEnv, testEnv2
	sbStaticData := interop.InitExecutionData{
		ExtensionEnv: extensionEnv,
		TelemetrySubscriptionConfig: interop.TelemetrySubscriptionConfig{
			Passphrase: "test-passphrase",
			APIAddr:    netip.MustParseAddrPort("127.0.0.1:1234"),
		},
		Runtime: rapidmodel.Runtime{
			ExecConfig: rapidmodel.RuntimeExec{
				Cmd:        make([]string, 2),
				WorkingDir: "",
				Env:        internalmodel.KVMap{"FOO": "BAR"},
			},
		},
	}

	ctx := context.Background()

	for range 4 {
		_ = registrationService.InitFlow().RuntimeReady()
	}

	initErr := rapidCtx.HandleInit(ctx, sbStaticData, initMetrics)
	assert.Nil(t, initErr)
}

func createRapidContext() (*rapidContext, core.RegistrationService, interop.InitExecutionData, chan model.Event, *initMetrics) {
	invokeStarted := make(chan bool)
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow)
	mockedProcessSupervisor := &model.MockProcessSupervisor{}
	eventChan := make(chan model.Event)
	mockedProcessSupervisor.On("Events", mock.Anything).Return((<-chan model.Event)(eventChan), nil)
	mockedProcessSupervisor.On("Exec", mock.Anything, mock.Anything).Return(nil).Once()
	mockedProcessSupervisor.On("Exec", mock.Anything, mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) { invokeStarted <- true })
	mockedProcessSupervisor.On("Kill", mock.Anything, mock.Anything).Return(nil)
	procSupv := &processSupervisor{ProcessSupervisor: mockedProcessSupervisor}

	rapidCtx, testEnv, testEnv2 := makeRapidContext(appCtx, initFlow, registrationService, procSupv, makeFileUtils(false))

	_, extensionEnv := testEnv, testEnv2
	sbStaticData := interop.InitExecutionData{
		ExtensionEnv: extensionEnv,
		TelemetrySubscriptionConfig: interop.TelemetrySubscriptionConfig{
			Passphrase: "test-passphrase",
			APIAddr:    netip.MustParseAddrPort("127.0.0.1:1234"),
		},
		Runtime: rapidmodel.Runtime{
			ExecConfig: rapidmodel.RuntimeExec{
				Cmd:        make([]string, 2),
				WorkingDir: "",
				Env:        make(internalmodel.KVMap),
			},
		},
	}

	initMetrics := NewInitMetrics(nil)
	return rapidCtx, registrationService, sbStaticData, eventChan, initMetrics
}

func mockInitMetricsExtensionStartFailed(initMetrics *interop.MockInitMetrics) {
	initMetrics.On("TriggerStartRequest")
	initMetrics.On("SetExtensionsNumber", mock.AnythingOfType("int"), mock.AnythingOfType("int"))
	initMetrics.On("TriggerInitCustomerPhaseDone")
	initMetrics.On("RunDuration").Return(time.Millisecond)
	initMetrics.On("SetLogsAPIMetrics", mock.AnythingOfType("interop.TelemetrySubscriptionMetrics"))
}

func mockInitMetricsRuntimeFail(initMetrics *interop.MockInitMetrics) {
	mockInitMetricsExtensionStartFailed(initMetrics)
	initMetrics.On("TriggerStartingRuntime")
}

func mockInitMetricsFullFlow(initMetrics *interop.MockInitMetrics) {
	mockInitMetricsRuntimeFail(initMetrics)
	initMetrics.On("TriggerRuntimeDone")
}

func registerDummyExtensions(registrationService core.RegistrationService, numInternalExtensions, numExternalExtensions int) error {
	for i := 0; i < numExternalExtensions; i++ {
		_, err := registrationService.CreateExternalAgent("external/" + strconv.Itoa(i))
		if err != nil {
			return err
		}
	}

	for i := 0; i < numInternalExtensions; i++ {
		_, err := registrationService.CreateInternalAgent("internal/" + strconv.Itoa(i))
		if err != nil {
			return err
		}
	}

	return nil
}

func TestExecFailureOnPlatformErrorForExtensions(t *testing.T) {
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow)
	mockedProcessSupervisor := &model.MockProcessSupervisor{}
	defer mockedProcessSupervisor.AssertExpectations(t)

	eventChan := make(chan model.Event)
	mockedProcessSupervisor.On("Events", mock.Anything).Return((<-chan model.Event)(eventChan), nil)
	err := &model.SupervisorError{
		SourceErr: model.ErrorSourceServer,
		ReasonErr: "AnyErrorReason",
		CauseErr:  "msg",
	}
	mockedProcessSupervisor.On("Exec", mock.Anything, mock.MatchedBy(func(exec *model.ExecRequest) bool {
		return exec.Name == "extension-NoOp.ext"
	})).Return(err).Once()

	initMetrics := &interop.MockInitMetrics{}
	mockInitMetricsExtensionStartFailed(initMetrics)
	defer initMetrics.AssertExpectations(t)

	procSupv := &processSupervisor{ProcessSupervisor: mockedProcessSupervisor}

	rapidCtx, testEnv, testEnv2 := makeRapidContext(appCtx, initFlow, registrationService, procSupv, makeFileUtils(true))

	_, extensionEnv := testEnv, testEnv2
	sbStaticData := interop.InitExecutionData{
		ExtensionEnv: extensionEnv,
		TelemetrySubscriptionConfig: interop.TelemetrySubscriptionConfig{
			Passphrase: "test-passphrase",
			APIAddr:    netip.MustParseAddrPort("127.0.0.1:1234"),
		},
		Runtime: rapidmodel.Runtime{
			ExecConfig: rapidmodel.RuntimeExec{
				Cmd:        make([]string, 2),
				WorkingDir: "",
				Env:        internalmodel.KVMap{"FOO": "BAR"},
			},
		},
	}

	ctx := context.Background()

	require.NoError(t, initFlow.SetExternalAgentsRegisterCount(1))
	require.NoError(t, initFlow.ExternalAgentRegistered())

	initErr := rapidCtx.HandleInit(ctx, sbStaticData, initMetrics)
	assert.NotNil(t, initErr)

	assert.Equal(t, "Extension.LaunchError", string(initErr.ErrorType()))
	assert.Equal(t, "Invalid.Runtime", string(initErr.ErrorCategory()))
}

func TestPrepareRuntimeBootstrap(t *testing.T) {
	testCases := []struct {
		name              string
		cmd               []string
		workingDir        string
		env               internalmodel.KVMap
		artefactType      internalmodel.ArtefactType
		setupMocks        func(*utils.MockFileUtil)
		expectedCmd       []string
		expectedEnv       internalmodel.KVMap
		expectedCwd       string
		expectedErrorType rapidmodel.ErrorType
		shouldHaveError   bool
	}{
		{
			name:         "ZIP: Bootstrap order - first location exists",
			cmd:          []string{},
			workingDir:   "/var/task",
			env:          internalmodel.KVMap{"KEY": "VALUE"},
			artefactType: internalmodel.ArtefactTypeZIP,
			setupMocks: func(mockFileUtil *utils.MockFileUtil) {
				fileInfo := utils.NewMockFileInfo()
				fileInfo.On("IsDir").Return(false)
				mockFileUtil.On("Stat", "/var/runtime/bootstrap").Return(fileInfo, nil)
				mockFileUtil.On("Stat", "/var/task").Return(fileInfo, nil)
			},
			expectedCmd:     []string{"/var/runtime/bootstrap"},
			expectedEnv:     internalmodel.KVMap{"KEY": "VALUE"},
			expectedCwd:     "/var/task",
			shouldHaveError: false,
		},
		{
			name:         "ZIP: Bootstrap order - second location exists",
			cmd:          []string{},
			workingDir:   "/var/task",
			env:          internalmodel.KVMap{"KEY": "VALUE"},
			artefactType: internalmodel.ArtefactTypeZIP,
			setupMocks: func(mockFileUtil *utils.MockFileUtil) {
				fileInfo := utils.NewMockFileInfo()
				fileInfo.On("IsDir").Return(false)

				mockFileUtil.On("Stat", "/var/runtime/bootstrap").Return(nil, fmt.Errorf("not found"))
				mockFileUtil.On("Stat", "/var/task/bootstrap").Return(fileInfo, nil)
				mockFileUtil.On("Stat", "/var/task").Return(fileInfo, nil)
			},
			expectedCmd:     []string{"/var/task/bootstrap"},
			expectedEnv:     internalmodel.KVMap{"KEY": "VALUE"},
			expectedCwd:     "/var/task",
			shouldHaveError: false,
		},
		{
			name:         "ZIP: Bootstrap order - third location exists",
			cmd:          []string{},
			workingDir:   "/var/task",
			env:          internalmodel.KVMap{"KEY": "VALUE"},
			artefactType: internalmodel.ArtefactTypeZIP,
			setupMocks: func(mockFileUtil *utils.MockFileUtil) {
				fileInfo := utils.NewMockFileInfo()
				fileInfo.On("IsDir").Return(false)

				mockFileUtil.On("Stat", "/var/runtime/bootstrap").Return(nil, fmt.Errorf("not found"))

				mockFileUtil.On("Stat", "/var/task/bootstrap").Return(nil, fmt.Errorf("not found"))
				mockFileUtil.On("Stat", "/opt/bootstrap").Return(fileInfo, nil)
				mockFileUtil.On("Stat", "/var/task").Return(fileInfo, nil)
			},
			expectedCmd:     []string{"/opt/bootstrap"},
			expectedEnv:     internalmodel.KVMap{"KEY": "VALUE"},
			expectedCwd:     "/var/task",
			shouldHaveError: false,
		},
		{
			name:         "ZIP: Bootstrap order preference - multiple bootstrap files exist",
			cmd:          []string{},
			workingDir:   "/var/task",
			env:          internalmodel.KVMap{"KEY": "VALUE"},
			artefactType: internalmodel.ArtefactTypeZIP,
			setupMocks: func(mockFileUtil *utils.MockFileUtil) {
				fileInfo := utils.NewMockFileInfo()
				fileInfo.On("IsDir").Return(false)

				mockFileUtil.On("Stat", "/var/runtime/bootstrap").Return(fileInfo, nil)
				mockFileUtil.On("Stat", "/var/task/bootstrap").Return(fileInfo, nil)
				mockFileUtil.On("Stat", "/opt/bootstrap").Return(fileInfo, nil)
				mockFileUtil.On("Stat", "/var/task").Return(fileInfo, nil)
			},

			expectedCmd:     []string{"/var/runtime/bootstrap"},
			expectedEnv:     internalmodel.KVMap{"KEY": "VALUE"},
			expectedCwd:     "/var/task",
			shouldHaveError: false,
		},
		{
			name:         "ZIP: No bootstrap command and no valid default location",
			cmd:          []string{},
			workingDir:   "/var/task",
			env:          internalmodel.KVMap{"KEY": "VALUE"},
			artefactType: internalmodel.ArtefactTypeZIP,
			setupMocks: func(mockFileUtil *utils.MockFileUtil) {
				fileInfo := utils.NewMockFileInfo()
				fileInfo.On("IsDir").Return(false)
				mockFileUtil.On("Stat", "/var/task").Return(fileInfo, nil)
				mockFileUtil.On("Stat", "/var/runtime/bootstrap").Return(nil, fmt.Errorf("not found"))
				mockFileUtil.On("Stat", "/var/task/bootstrap").Return(nil, fmt.Errorf("not found"))
				mockFileUtil.On("Stat", "/opt/bootstrap").Return(nil, fmt.Errorf("not found"))
			},
			expectedCmd:       nil,
			expectedEnv:       nil,
			expectedCwd:       "",
			expectedErrorType: rapidmodel.ErrorRuntimeInvalidEntryPoint,
			shouldHaveError:   true,
		},
		{
			name:         "Invalid working directory",
			cmd:          []string{"cmd", "arg"},
			workingDir:   "/non/existent/dir",
			env:          internalmodel.KVMap{"KEY": "VALUE"},
			artefactType: internalmodel.ArtefactTypeOCI,
			setupMocks: func(mockFileUtil *utils.MockFileUtil) {
				mockFileUtil.On("Stat", "/non/existent/dir").Return(nil, fmt.Errorf("directory does not exist"))
			},
			expectedCmd:       nil,
			expectedEnv:       nil,
			expectedCwd:       "",
			expectedErrorType: rapidmodel.ErrorRuntimeInvalidWorkingDir,
			shouldHaveError:   true,
		},
		{
			name:         "OCI: No bootstrap command (should not look for default locations)",
			cmd:          []string{},
			workingDir:   "/var/task",
			env:          internalmodel.KVMap{"KEY": "VALUE"},
			artefactType: internalmodel.ArtefactTypeOCI,
			setupMocks: func(mockFileUtil *utils.MockFileUtil) {
				fileInfo := utils.NewMockFileInfo()
				fileInfo.On("IsDir").Return(false)
				mockFileUtil.On("Stat", "/var/task").Return(fileInfo, nil)
			},
			expectedCmd:     []string{},
			expectedEnv:     internalmodel.KVMap{"KEY": "VALUE"},
			expectedCwd:     "/var/task",
			shouldHaveError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			appCtx := appctx.NewApplicationContext()
			initFlow := core.NewInitFlowSynchronization()
			registrationService := core.NewRegistrationService(initFlow)

			mockEventsAPI := &MockedEventsAPI{}
			mockEventsAPI.On("SendImageError", mock.Anything).Return(nil)

			mockFileUtil := &utils.MockFileUtil{}
			tc.setupMocks(mockFileUtil)

			rapidCtx := &rapidContext{
				appCtx:              appCtx,
				initFlow:            initFlow,
				registrationService: registrationService,
				eventsAPI:           mockEventsAPI,
				fileUtils:           mockFileUtil,
			}

			sbStaticData := interop.InitExecutionData{
				Runtime: rapidmodel.Runtime{
					ExecConfig: rapidmodel.RuntimeExec{
						Cmd:        tc.cmd,
						Env:        tc.env,
						WorkingDir: tc.workingDir,
					},
				},
				StaticData: interop.EEStaticData{ArtefactType: tc.artefactType},
			}

			cmd, env, cwd, err := prepareRuntimeBootstrap(rapidCtx, sbStaticData)

			if tc.shouldHaveError {
				assert.NotNil(t, err)
				assert.Equal(t, tc.expectedErrorType, err.ErrorType())
				assert.Empty(t, cmd)
				assert.Empty(t, env)
				assert.Empty(t, cwd)
			} else {
				assert.Equal(t, tc.expectedCmd, cmd)
				assert.Equal(t, tc.expectedEnv, env)
				assert.Equal(t, tc.expectedCwd, cwd)
			}
		})
	}
}
