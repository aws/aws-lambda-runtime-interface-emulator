// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package raptor

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	internalModel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/raptor/internal"
)

func TestStartApp(t *testing.T) {
	t.Run("successful start", func(t *testing.T) {
		app := &App{
			state:  internal.NewStateGuard(),
			doneCh: make(chan struct{}),
		}
		assert.Equal(t, internal.Idle, app.state.GetState())
	})
}

func TestAppInitSuccessful(t *testing.T) {
	mockRapidCtx, app, initRequest, initMetrics, _, _ := setupAppTest(t)
	assert.Equal(t, internal.Idle, app.state.GetState())

	mockRapidCtx.On("HandleInit", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	initErr := app.Init(context.Background(), initRequest, initMetrics)
	assert.Nil(t, initErr)

	assert.Equal(t, internal.Initialized, app.state.GetState())
	mockRapidCtx.AssertExpectations(t)
}

func TestAppInitFailure(t *testing.T) {
	expectedErr := model.NewCustomerError(model.ErrorReasonRuntimeExecFailed, model.WithSeverity(model.ErrorSeverityFatal))
	mockRapidCtx, app, initRequest, initMetrics, _, _ := setupAppTest(t)
	mockRapidCtx.On("HandleInit", mock.Anything, mock.Anything, mock.Anything).Return(expectedErr)
	mockRapidCtx.On("HandleShutdown", mock.Anything, mock.Anything).Return(nil)

	initErr := app.Init(context.Background(), initRequest, initMetrics)
	assert.Equal(t, expectedErr, initErr)

	assert.Equal(t, internal.Shutdown, app.state.GetState())
	mockRapidCtx.AssertExpectations(t)
}

func TestAppInitInvalidState(t *testing.T) {
	_, app, initRequest, initMetrics, _, _ := setupAppTest(t)

	require.NoError(t, app.state.SetState(internal.ShuttingDown))

	response := app.Init(context.Background(), initRequest, initMetrics)
	clientErr, ok := response.(interop.ClientError)
	assert.True(t, ok)
	assert.Equal(t, model.ErrorInvalidRequest, clientErr.ErrorType())
}

func TestAppInitSuccessfulButStateError(t *testing.T) {
	mockRapidCtx, app, initRequest, initMetrics, _, _ := setupAppTest(t)
	assert.Equal(t, internal.Idle, app.state.GetState())

	mockRapidCtx.On("HandleInit", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {

		require.NoError(t, app.state.SetState(internal.ShuttingDown))
	}).Return(nil)

	initErr := app.Init(context.Background(), initRequest, initMetrics)
	assert.NotNil(t, initErr)
	assert.Equal(t, model.ErrorSeverityFatal, initErr.Severity())
	assert.Equal(t, model.ErrorInvalidRequest, initErr.ErrorType())

	mockRapidCtx.AssertExpectations(t)
}

func TestStartProcessTerminationMonitor(t *testing.T) {
	mockRapidCtx := interop.NewMockRapidContext(t)

	termChan := make(chan model.AppError)

	notifierCalled := make(chan struct{})

	mockRapidCtx.On("ProcessTerminationNotifier").Return((<-chan model.AppError)(termChan)).Run(func(args mock.Arguments) {
		close(notifierCalled)
	}).Maybe()
	mockRapidCtx.On("HandleShutdown", mock.Anything, mock.Anything).Return(nil)

	mockLogger := newMockRaptorLogger(t)
	mockLogger.On("Log", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	app := &App{
		rapidCtx:     mockRapidCtx,
		state:        internal.NewStateGuard(),
		doneCh:       make(chan struct{}),
		raptorLogger: mockLogger,
	}

	assert.Equal(t, internal.Idle, app.state.GetState())
	app.StartProcessTerminationMonitor()

	<-notifierCalled
	close(termChan)

	stateChanged := make(chan struct{})
	go func() {
		for {
			if app.state.GetState() == internal.Shutdown {
				close(stateChanged)
				return
			}
			time.Sleep(1 * time.Millisecond)
		}
	}()

	select {
	case <-stateChanged:

	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timed out waiting for state to change to Shutdown")
	}

	assert.Equal(t, internal.Shutdown, app.state.GetState())
	mockRapidCtx.AssertExpectations(t)
}

func TestAppShutdown(t *testing.T) {
	mockRapidCtx, app, _, _, _, _ := setupAppTest(t)

	mockRapidCtx.On("HandleShutdown", mock.Anything, mock.Anything).Return(nil)

	assert.NoError(t, app.Err())
	select {
	case <-app.Done():
		t.Fatal("app.Done must have been blocked")
	default:
	}

	err := model.NewCustomerError(model.ErrorRuntimeUnknown)
	app.Shutdown(err)

	assert.Equal(t, err, app.Err())
	select {
	case <-app.Done():
	default:
		t.Fatal("app.Done must have been unblocked")
	}

	assert.Equal(t, internal.Shutdown, app.state.GetState())
}

func TestAppInvokeStateValidation(t *testing.T) {
	testCases := []struct {
		name          string
		states        []internal.Status
		wantErrorType model.ErrorType
		wantError     error
	}{
		{
			name:          "Idle",
			states:        []internal.Status{},
			wantErrorType: model.ErrorInitIncomplete,
			wantError:     ErrNotInitialized,
		},
		{
			name:          "Initializing",
			states:        []internal.Status{internal.Initializing},
			wantErrorType: model.ErrorInitIncomplete,
			wantError:     ErrNotInitialized,
		},
		{
			name:          "ShuttingDown",
			states:        []internal.Status{internal.ShuttingDown},
			wantErrorType: model.ErrorEnvironmentUnhealthy,
			wantError:     ErrorEnvironmentUnhealthy,
		},
		{
			name:          "Shutdown",
			states:        []internal.Status{internal.ShuttingDown, internal.Shutdown},
			wantErrorType: model.ErrorEnvironmentUnhealthy,
			wantError:     ErrorEnvironmentUnhealthy,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, app, _, _, invokeMsg, invokeMetrics := setupAppTest(t)

			for _, state := range tc.states {
				require.NoError(t, app.state.SetState(state))
			}

			err, wasResponseSent := app.Invoke(context.Background(), invokeMsg, invokeMetrics)

			assert.False(t, wasResponseSent)
			assert.ErrorAs(t, err, &interop.ClientError{})
			assert.Equal(t, tc.wantErrorType, err.ErrorType())
			assert.Equal(t, tc.wantError, err.Unwrap())
		})
	}
}

func setupAppTest(t *testing.T) (*interop.MockRapidContext, *App, *internalModel.InitRequestMessage, interop.InitMetrics, interop.InvokeRequest, interop.InvokeMetrics) {
	mockRapidCtx := interop.NewMockRapidContext(t)

	mockAddr, _ := netip.ParseAddrPort("127.0.0.1:8080")
	mockRapidCtx.On("RuntimeAPIAddrPort").Return(mockAddr).Maybe()

	mockChan := make(chan model.AppError)
	mockRapidCtx.On("ProcessTerminationNotifier").Return((<-chan model.AppError)(mockChan)).Maybe()

	mockLogger := newMockRaptorLogger(t)
	mockLogger.On("Log", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	mockLogger.On("SetInitData", mock.Anything).Maybe()

	app := &App{
		rapidCtx:     mockRapidCtx,
		state:        internal.NewStateGuard(),
		doneCh:       make(chan struct{}),
		raptorLogger: mockLogger,
	}

	app.StartProcessTerminationMonitor()
	initRequest := &internalModel.InitRequestMessage{
		Handler:             "handler",
		TelemetryAPIAddress: internalModel.TelemetryAddr(netip.MustParseAddrPort("127.0.0.1:8081")),
	}

	mockInvokeReq := interop.NewMockInvokeRequest(t)

	mockInvokeReq.On("ContentType").Return("").Maybe()
	mockInvokeReq.On("InvokeID").Return("").Maybe()
	mockInvokeReq.On("Deadline").Return(time.Time{}).Maybe()
	mockInvokeReq.On("TraceId").Return("").Maybe()
	mockInvokeReq.On("ClientContext").Return("").Maybe()
	mockInvokeReq.On("CognitoId").Return("").Maybe()
	mockInvokeReq.On("CognitoPoolId").Return("").Maybe()
	mockInvokeReq.On("ResponseBandwidthRate").Return(int64(0)).Maybe()
	mockInvokeReq.On("ResponseBandwidthBurstRate").Return(int64(0)).Maybe()
	mockInvokeReq.On("MaxPayloadSize").Return(int64(0)).Maybe()
	mockInvokeReq.On("BodyReader").Return(nil).Maybe()
	mockInvokeReq.On("ResponseWriter").Return(nil).Maybe()
	mockInvokeReq.On("SetResponseHeader", mock.Anything, mock.Anything).Return().Maybe()
	mockInvokeReq.On("AddResponseHeader", mock.Anything, mock.Anything).Return().Maybe()
	mockInvokeReq.On("WriteResponseHeaders", mock.Anything).Return().Maybe()
	mockInvokeReq.On("UpdateFromInitData", mock.Anything).Return(nil).Maybe()

	mockInitMetrics := interop.NewMockInitMetrics(t)
	mockInitMetrics.On("SetInitData", mock.Anything).Maybe()

	mockInvokeMetrics := interop.NewMockInvokeMetrics(t)

	return mockRapidCtx, app, initRequest, mockInitMetrics, mockInvokeReq, mockInvokeMetrics
}
