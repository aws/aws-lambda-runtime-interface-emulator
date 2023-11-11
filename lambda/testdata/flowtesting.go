// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package testdata

import (
	"bytes"
	"context"
	"io/ioutil"
	"time"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi/rendering"
	"go.amzn.com/lambda/telemetry"
	"go.amzn.com/lambda/testdata/mockthread"
)

const (
	contentTypeHeader          = "Content-Type"
	functionResponseModeHeader = "Lambda-Runtime-Function-Response-Mode"
)

type MockInteropServer struct {
	Response             []byte
	ErrorResponse        *interop.ErrorInvokeResponse
	ResponseContentType  string
	FunctionResponseMode string
	ActiveInvokeID       string
}

// SendResponse writes response to a shared memory.
func (i *MockInteropServer) SendResponse(invokeID string, resp *interop.StreamableInvokeResponse) error {
	bytes, err := ioutil.ReadAll(resp.Payload)
	if err != nil {
		return err
	}
	if len(bytes) > interop.MaxPayloadSize {
		return &interop.ErrorResponseTooLarge{
			ResponseSize:    len(bytes),
			MaxResponseSize: interop.MaxPayloadSize,
		}
	}
	i.Response = bytes
	i.ResponseContentType = resp.Headers[contentTypeHeader]
	i.FunctionResponseMode = resp.Headers[functionResponseModeHeader]
	return nil
}

// SendErrorResponse writes error response to a shared memory and sends GIRD FAULT.
func (i *MockInteropServer) SendErrorResponse(invokeID string, response *interop.ErrorInvokeResponse) error {
	i.ErrorResponse = response
	i.ResponseContentType = response.Headers.ContentType
	i.FunctionResponseMode = response.Headers.FunctionResponseMode
	return nil
}

// SendInitErrorResponse writes error response during init to a shared memory and sends GIRD FAULT.
func (i *MockInteropServer) SendInitErrorResponse(response *interop.ErrorInvokeResponse) error {
	i.ErrorResponse = response
	i.ResponseContentType = response.Headers.ContentType
	return nil
}

func (i *MockInteropServer) GetCurrentInvokeID() string {
	return i.ActiveInvokeID
}

func (i *MockInteropServer) SendRuntimeReady() error { return nil }

// FlowTest provides configuration for tests that involve synchronization flows.
type FlowTest struct {
	AppCtx                appctx.ApplicationContext
	InitFlow              core.InitFlowSynchronization
	InvokeFlow            core.InvokeFlowSynchronization
	RegistrationService   core.RegistrationService
	RenderingService      *rendering.EventRenderingService
	Runtime               *core.Runtime
	InteropServer         *MockInteropServer
	TelemetrySubscription *telemetry.NoOpSubscriptionAPI
	CredentialsService    core.CredentialsService
	EventsAPI             interop.EventsAPI
}

// ConfigureForInit initialize synchronization gates and states for init.
func (s *FlowTest) ConfigureForInit() {
	s.RegistrationService.PreregisterRuntime(s.Runtime)
}

// ConfigureForInvoke initialize synchronization gates and states for invoke.
func (s *FlowTest) ConfigureForInvoke(ctx context.Context, invoke *interop.Invoke) {
	s.InteropServer.ActiveInvokeID = invoke.ID
	s.InvokeFlow.InitializeBarriers()
	var buf bytes.Buffer // create default invoke renderer with new request buffer each time
	s.ConfigureInvokeRenderer(ctx, invoke, &buf)
}

// ConfigureInvokeRenderer overrides default invoke renderer to reuse request buffers (for benchmarks), etc.
func (s *FlowTest) ConfigureInvokeRenderer(ctx context.Context, invoke *interop.Invoke, buf *bytes.Buffer) {
	s.RenderingService.SetRenderer(rendering.NewInvokeRenderer(ctx, invoke, buf, telemetry.NewNoOpTracer().BuildTracingHeader()))
}

func (s *FlowTest) ConfigureForRestore() {
	s.RenderingService.SetRenderer(rendering.NewRestoreRenderer())
}

func (s *FlowTest) ConfigureForRestoring() {
	s.RegistrationService.PreregisterRuntime(s.Runtime)
	s.Runtime.SetState(s.Runtime.RuntimeRestoringState)
	s.RenderingService.SetRenderer(rendering.NewRestoreRenderer())
}

func (s *FlowTest) ConfigureForInitCaching(token, awsKey, awsSecret, awsSession string) {
	credentialsExpiration := time.Now().Add(30 * time.Minute)
	s.CredentialsService.SetCredentials(token, awsKey, awsSecret, awsSession, credentialsExpiration)
}

// NewFlowTest returns new FlowTest configuration.
func NewFlowTest() *FlowTest {
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	invokeFlow := core.NewInvokeFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow, invokeFlow)
	renderingService := rendering.NewRenderingService()
	credentialsService := core.NewCredentialsService()
	runtime := core.NewRuntime(initFlow, invokeFlow)
	runtime.ManagedThread = &mockthread.MockManagedThread{}
	interopServer := &MockInteropServer{}
	eventsAPI := telemetry.NoOpEventsAPI{}
	appctx.StoreInteropServer(appCtx, interopServer)
	appctx.StoreResponseSender(appCtx, interopServer)

	return &FlowTest{
		AppCtx:                appCtx,
		InitFlow:              initFlow,
		InvokeFlow:            invokeFlow,
		RegistrationService:   registrationService,
		RenderingService:      renderingService,
		TelemetrySubscription: &telemetry.NoOpSubscriptionAPI{},
		Runtime:               runtime,
		InteropServer:         interopServer,
		CredentialsService:    credentialsService,
		EventsAPI:             &eventsAPI,
	}
}
