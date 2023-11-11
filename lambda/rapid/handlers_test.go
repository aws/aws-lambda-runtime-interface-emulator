// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi"
	"go.amzn.com/lambda/rapi/handler"
	"go.amzn.com/lambda/rapi/rendering"
	"go.amzn.com/lambda/rapidcore/env"
	"go.amzn.com/lambda/supervisor/model"
	"go.amzn.com/lambda/telemetry"
	"go.amzn.com/lambda/testdata"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func BenchmarkChannelsSelect10(b *testing.B) {
	c1 := make(chan int)
	c2 := make(chan int)
	c3 := make(chan int)
	c4 := make(chan int)
	c5 := make(chan int)
	c6 := make(chan int)
	c7 := make(chan int)
	c8 := make(chan int)
	c9 := make(chan int)
	c10 := make(chan int)

	for n := 0; n < b.N; n++ {
		select {
		case <-c1:
		case <-c2:
		case <-c3:
		case <-c4:
		case <-c5:
		case <-c6:
		case <-c7:
		case <-c8:
		case <-c9:
		case <-c10:
		default:
		}
	}
}

func BenchmarkChannelsSelect2(b *testing.B) {
	c1 := make(chan int)
	c2 := make(chan int)

	for n := 0; n < b.N; n++ {
		select {
		case <-c1:
		case <-c2:
		default:
		}
	}
}

func TestGetExtensionNamesWithNoExtensions(t *testing.T) {
	rs := core.NewRegistrationService(nil, nil)

	c := &rapidContext{
		registrationService: rs,
	}

	assert.Equal(t, "", c.GetExtensionNames())
}

func TestGetExtensionNamesWithMultipleExtensions(t *testing.T) {
	rs := core.NewRegistrationService(nil, nil)
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
	rs := core.NewRegistrationService(nil, nil)
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
	rs := core.NewRegistrationService(nil, nil)
	for i := 10; i < 60; i++ {
		_, _ = rs.CreateExternalAgent(strings.Repeat("E", 130))
	}

	c := &rapidContext{
		registrationService: rs,
	}

	assert.Equal(t, "", c.GetExtensionNames())
}

// This test confirms our assumption that http client can establish a tcp connection
// to a listening server.
func TestListen(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	flowTest.ConfigureForInvoke(context.Background(), &interop.Invoke{ID: "ID", DeadlineNs: "1", Payload: strings.NewReader("MyTest")})

	ctx := context.Background()
	telemetryAPIEnabled := true
	server := rapi.NewServer("127.0.0.1", 0, flowTest.AppCtx, flowTest.RegistrationService, flowTest.RenderingService, telemetryAPIEnabled, flowTest.TelemetrySubscription, flowTest.TelemetrySubscription, flowTest.CredentialsService)
	err := server.Listen()
	assert.NoError(t, err)

	defer server.Close()

	go func() {
		time.Sleep(time.Second)
		fmt.Println("Serving...")
		server.Serve(ctx)
	}()

	done := make(chan struct{})

	go func() {
		fmt.Println("Connecting...")
		resp, err1 := http.Get(fmt.Sprintf("http://%s:%d/2018-06-01/runtime/invocation/next", server.Host(), server.Port()))
		assert.Nil(t, err1)

		body, err2 := io.ReadAll(resp.Body)
		assert.Nil(t, err2)

		assert.Equal(t, "MyTest", string(body))

		done <- struct{}{}
	}()

	<-done
}

func makeRapidContext(appCtx appctx.ApplicationContext, initFlow core.InitFlowSynchronization, invokeFlow core.InvokeFlowSynchronization, registrationService core.RegistrationService, supervisor *processSupervisor) *rapidContext {

	appctx.StoreInitType(appCtx, true)
	appctx.StoreInteropServer(appCtx, MockInteropServer{})

	renderingService := rendering.NewRenderingService()

	credentialsService := core.NewCredentialsService()
	credentialsService.SetCredentials("token", "key", "secret", "session", time.Now())

	// Runtime state machine
	runtime := core.NewRuntime(initFlow, invokeFlow)

	registrationService.PreregisterRuntime(runtime)
	runtime.SetState(runtime.RuntimeRestoreReadyState)

	rapidCtx := &rapidContext{
		// Internally initialized configurations
		appCtx:                appCtx,
		initDone:              true,
		initFlow:              initFlow,
		invokeFlow:            invokeFlow,
		registrationService:   registrationService,
		renderingService:      renderingService,
		credentialsService:    credentialsService,
		handlerExecutionMutex: sync.Mutex{},
		shutdownContext:       newShutdownContext(),
		eventsAPI:             &telemetry.NoOpEventsAPI{},
	}
	if supervisor != nil {
		rapidCtx.supervisor = *supervisor
	}

	return rapidCtx
}

const hookErrorType = "Runtime.RestoreHookUserErrorType"

func makeRequest(appCtx appctx.ApplicationContext) *http.Request {
	errorBody := []byte("My byte array is yours")

	request := appctx.RequestWithAppCtx(httptest.NewRequest("POST", "/", bytes.NewReader(errorBody)), appCtx)

	request.Header.Set("Content-Type", "application/MyBinaryType")
	request.Header.Set("Lambda-Runtime-Function-Error-Type", hookErrorType)

	return request
}

type MockInteropServer struct{}

func (server MockInteropServer) GetCurrentInvokeID() string {
	return ""
}

func (server MockInteropServer) SendRuntimeReady() error {
	return nil
}

func (server MockInteropServer) SendInitErrorResponse(response *interop.ErrorInvokeResponse) error {
	return nil
}

func TestRestoreErrorAndAwaitRestoreCompletionRaceCondition(t *testing.T) {
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	invokeFlow := core.NewInvokeFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow, invokeFlow)

	rapidCtx := makeRapidContext(appCtx, initFlow, invokeFlow, registrationService, nil /* don't set process supervisor */)

	// Runtime state machine
	runtime := core.NewRuntime(initFlow, invokeFlow)
	registrationService.PreregisterRuntime(runtime)
	runtime.SetState(runtime.RuntimeRestoreReadyState)

	restore := &interop.Restore{
		AwsKey:               "key",
		AwsSecret:            "secret",
		AwsSession:           "session",
		CredentialsExpiry:    time.Now(),
		RestoreHookTimeoutMs: 10 * 1000,
	}

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()
		_, err := rapidCtx.HandleRestore(restore)
		assert.Equal(t, err.Error(), "errRestoreHookUserError")
		v, ok := err.(interop.ErrRestoreHookUserError)
		assert.True(t, ok)
		assert.Equal(t, v.UserError.Type, fatalerror.ErrorType(hookErrorType))
	}()

	responseRecorder := httptest.NewRecorder()

	handler := handler.NewRestoreErrorHandler(registrationService)

	request := makeRequest(appCtx)

	wg.Add(1)

	time.Sleep(1 * time.Second)
	runtime.SetState(runtime.RuntimeRestoringState)

	go func() {
		defer wg.Done()
		handler.ServeHTTP(responseRecorder, request)
	}()

	wg.Wait()
}

type MockedProcessSupervisor struct {
	mock.Mock
}

func (supv *MockedProcessSupervisor) Exec(ctx context.Context, req *model.ExecRequest) error {
	args := supv.Called(req)
	return args.Error(0)
}

func (supv *MockedProcessSupervisor) Events(ctx context.Context, req *model.EventsRequest) (<-chan model.Event, error) {
	args := supv.Called(req)
	err := args.Error(1)
	if err != nil {
		return nil, err
	}
	return args.Get(0).(<-chan model.Event), nil
}

func (supv *MockedProcessSupervisor) Terminate(ctx context.Context, req *model.TerminateRequest) error {
	args := supv.Called(req)
	return args.Error(0)
}

func (supv *MockedProcessSupervisor) Kill(ctx context.Context, req *model.KillRequest) error {
	args := supv.Called(req)
	return args.Error(0)
}

var _ model.ProcessSupervisor = (*MockedProcessSupervisor)(nil)

func TestSetupEventWatcherErrorHandling(t *testing.T) {
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	invokeFlow := core.NewInvokeFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow, invokeFlow)
	mockedProcessSupervisor := &MockedProcessSupervisor{}
	mockedProcessSupervisor.On("Events", mock.Anything).Return(nil, fmt.Errorf("events call failed"))
	procSupv := &processSupervisor{ProcessSupervisor: mockedProcessSupervisor}

	rapidCtx := makeRapidContext(appCtx, initFlow, invokeFlow, registrationService, procSupv)

	initSuccessResponseChan := make(chan interop.InitSuccess)
	initFailureResponseChan := make(chan interop.InitFailure)
	init := &interop.Init{EnvironmentVariables: env.NewEnvironment()}

	go assert.NotPanics(t, func() {
		rapidCtx.HandleInit(init, initSuccessResponseChan, initFailureResponseChan)
	})

	failure := <-initFailureResponseChan
	failure.Ack <- struct{}{}
	errorType := interop.InitFailure(failure).ErrorType
	assert.Equal(t, fatalerror.SandboxFailure, errorType)
}
