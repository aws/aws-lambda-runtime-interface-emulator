// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package testdata

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
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
	ErrorResponse        *interop.ErrorResponse
	ResponseContentType  string
	FunctionResponseMode string
	ActiveInvokeID       string
}

// SendResponse writes response to a shared memory.
func (i *MockInteropServer) SendResponse(invokeID string, headers map[string]string, reader io.Reader, trailers http.Header, request *interop.CancellableRequest) error {
	bytes, err := ioutil.ReadAll(reader)
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
	i.ResponseContentType = headers[contentTypeHeader]
	i.FunctionResponseMode = headers[functionResponseModeHeader]
	return nil
}

// SendErrorResponse writes error response to a shared memory and sends GIRD FAULT.
func (i *MockInteropServer) SendErrorResponse(invokeID string, response *interop.ErrorResponse) error {
	i.ErrorResponse = response
	i.ResponseContentType = response.ContentType
	i.FunctionResponseMode = response.FunctionResponseMode
	return nil
}

// SendInitErrorResponse writes error response during init to a shared memory and sends GIRD FAULT.
func (i *MockInteropServer) SendInitErrorResponse(invokeID string, response *interop.ErrorResponse) error {
	i.ErrorResponse = response
	i.ResponseContentType = response.ContentType
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
	EventsAPI             telemetry.EventsAPI
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

func (s *FlowTest) ConfigureForRestore() {
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
