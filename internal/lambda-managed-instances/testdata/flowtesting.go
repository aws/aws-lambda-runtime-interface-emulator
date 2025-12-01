// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package testdata

import (
	"bytes"
	"context"
	"io/ioutil"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/testdata/mockthread"
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
}

func (i *MockInteropServer) SendResponse(invokeID string, resp *interop.StreamableInvokeResponse) (*interop.InvokeResponseMetrics, error) {
	bytes, err := ioutil.ReadAll(resp.Payload)
	if err != nil {
		return nil, err
	}
	if len(bytes) > interop.MaxPayloadSize {
		return nil, &interop.ErrorResponseTooLarge{
			ResponseSize:    len(bytes),
			MaxResponseSize: interop.MaxPayloadSize,
		}
	}
	i.Response = bytes
	i.ResponseContentType = resp.Headers[contentTypeHeader]
	i.FunctionResponseMode = resp.Headers[functionResponseModeHeader]
	return nil, nil
}

func (i *MockInteropServer) SendErrorResponse(invokeID string, response *interop.ErrorInvokeResponse) (*interop.InvokeResponseMetrics, error) {
	i.ErrorResponse = response
	i.ResponseContentType = response.Headers.ContentType
	i.FunctionResponseMode = response.Headers.FunctionResponseMode
	return nil, nil
}

func (i *MockInteropServer) SendInitErrorResponse(response *interop.ErrorInvokeResponse) (*interop.InvokeResponseMetrics, error) {
	i.ErrorResponse = response
	i.ResponseContentType = response.Headers.ContentType
	return nil, nil
}

type FlowTest struct {
	AppCtx                appctx.ApplicationContext
	InitFlow              core.InitFlowSynchronization
	RegistrationService   core.RegistrationService
	RenderingService      *rendering.EventRenderingService
	Runtime               *core.Runtime
	InteropServer         *MockInteropServer
	TelemetrySubscription *telemetry.NoOpSubscriptionAPI
	EventsAPI             interop.EventsAPI
}

func (s *FlowTest) ConfigureForInit() {
	s.RegistrationService.PreregisterRuntime(s.Runtime)
}

func (s *FlowTest) ConfigureInvokeRenderer(ctx context.Context, invoke *interop.Invoke, buf *bytes.Buffer) {
	s.RenderingService.SetRenderer(rendering.NewInvokeRenderer(ctx, invoke, buf, func(context.Context) string { return "" }))
}

func NewFlowTest() *FlowTest {
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow)
	renderingService := rendering.NewRenderingService()
	runtime := core.NewRuntime(initFlow)
	runtime.ManagedThread = &mockthread.MockManagedThread{}
	interopServer := &MockInteropServer{}
	eventsAPI := telemetry.NoOpEventsAPI{}
	appctx.StoreInteropServer(appCtx, interopServer)
	appctx.StoreResponseSender(appCtx, interopServer)

	return &FlowTest{
		AppCtx:                appCtx,
		InitFlow:              initFlow,
		RegistrationService:   registrationService,
		RenderingService:      renderingService,
		TelemetrySubscription: &telemetry.NoOpSubscriptionAPI{},
		Runtime:               runtime,
		InteropServer:         interopServer,
		EventsAPI:             &eventsAPI,
	}
}
