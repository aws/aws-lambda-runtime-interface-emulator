// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package testdata

import (
	"context"
	"io"
	"net/http"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/core/statejson"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi/rendering"
	"go.amzn.com/lambda/telemetry"
	"go.amzn.com/lambda/testdata/mockthread"
)

type MockInteropServer struct {
	Response       *interop.Response
	ErrorResponse  *interop.ErrorResponse
	ActiveInvokeID string
}

// SendResponse writes response to a shared memory.
func (i *MockInteropServer) SendResponse(invokeID string, response *interop.Response) error {
	i.Response = response
	return nil
}

// SendErrorResponse writes error response to a shared memory and sends GIRD FAULT.
func (i *MockInteropServer) SendErrorResponse(invokeID string, response *interop.ErrorResponse) error {
	i.ErrorResponse = response
	return nil
}

func (i *MockInteropServer) GetCurrentInvokeID() string {
	return i.ActiveInvokeID
}

func (i *MockInteropServer) CommitResponse() error { return nil }

// SendRunning sends GIRD RUNNING.
func (i *MockInteropServer) SendRunning(*interop.Running) error { return nil }

// SendDone sends GIRD DONE.
func (i *MockInteropServer) SendDone(*interop.Done) error { return nil }

// SendDoneFail sends GIRD DONEFAIL.
func (i *MockInteropServer) SendDoneFail(*interop.DoneFail) error { return nil }

// StartChan returns Start emitter
func (i *MockInteropServer) StartChan() <-chan *interop.Start { return nil }

// InvokeChan returns Invoke emitter
func (i *MockInteropServer) InvokeChan() <-chan *interop.Invoke { return nil }

// ResetChan returns Reset emitter
func (i *MockInteropServer) ResetChan() <-chan *interop.Reset { return nil }

// ShutdownChan returns Shutdown emitter
func (i *MockInteropServer) ShutdownChan() <-chan *interop.Shutdown { return nil }

// TransportErrorChan emits errors if there was parsing/connection issue
func (i *MockInteropServer) TransportErrorChan() <-chan error { return nil }

func (i *MockInteropServer) Clear() {}

func (i *MockInteropServer) IsResponseSent() bool {
	return !(i.Response == nil && i.ErrorResponse == nil)
}

func (i *MockInteropServer) SendRuntimeReady() error { return nil }

func (i *MockInteropServer) SetInternalStateGetter(isd interop.InternalStateGetter) {}

func (m *MockInteropServer) Init(i *interop.Start, invokeTimeoutMs int64) {}

func (m *MockInteropServer) Invoke(w io.Writer, i *interop.Invoke) error { return nil }

func (m *MockInteropServer) Shutdown(shutdown *interop.Shutdown) *statejson.InternalStateDescription { return nil }


// FlowTest provides configuration for tests that involve synchronization flows.
type FlowTest struct {
	AppCtx              appctx.ApplicationContext
	InitFlow            core.InitFlowSynchronization
	InvokeFlow          core.InvokeFlowSynchronization
	RegistrationService core.RegistrationService
	RenderingService    *rendering.EventRenderingService
	Runtime             *core.Runtime
	InteropServer       *MockInteropServer
	TelemetryService    *MockNoOpTelemetryService
}

// ConfigureForInit initialize synchronization gates and states for init.
func (s *FlowTest) ConfigureForInit() {
	s.RegistrationService.PreregisterRuntime(s.Runtime)
}

// ConfigureForInvoke initialize synchronization gates and states for invoke.
func (s *FlowTest) ConfigureForInvoke(ctx context.Context, invoke *interop.Invoke) {
	s.InteropServer.ActiveInvokeID = invoke.ID
	s.InvokeFlow.InitializeBarriers()
	s.RenderingService.SetRenderer(rendering.NewInvokeRenderer(ctx, invoke, telemetry.GetCustomerTracingHeader))
}

// MockNoOpTelemetryService is a no-op telemetry API used in tests where it does not matter
type MockNoOpTelemetryService struct{}

// Subscribe writes response to a shared memory
func (m *MockNoOpTelemetryService) Subscribe(agentName string, body io.Reader, headers map[string][]string) ([]byte, int, map[string][]string, error) {
	return []byte(`{}`), http.StatusOK, map[string][]string{}, nil
}

func (s *MockNoOpTelemetryService) RecordCounterMetric(metricName string, count int) {
	// NOOP
}

func (s *MockNoOpTelemetryService) FlushMetrics() interop.LogsAPIMetrics {
	return interop.LogsAPIMetrics(map[string]int{})
}

func (m *MockNoOpTelemetryService) Clear() {
	// NOOP
}

func (m *MockNoOpTelemetryService) TurnOff() {
	// NOOP
}

// NewFlowTest returns new FlowTest configuration.
func NewFlowTest() *FlowTest {
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	invokeFlow := core.NewInvokeFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow, invokeFlow)
	renderingService := rendering.NewRenderingService()
	runtime := core.NewRuntime(initFlow, invokeFlow)
	runtime.ManagedThread = &mockthread.MockManagedThread{}
	interopServer := &MockInteropServer{}
	appctx.StoreInteropServer(appCtx, interopServer)
	return &FlowTest{
		AppCtx:              appCtx,
		InitFlow:            initFlow,
		InvokeFlow:          invokeFlow,
		RegistrationService: registrationService,
		RenderingService:    renderingService,
		TelemetryService:    &MockNoOpTelemetryService{},
		Runtime:             runtime,
		InteropServer:       interopServer,
	}
}
