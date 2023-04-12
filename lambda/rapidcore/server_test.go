// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.amzn.com/lambda/core/statejson"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapidcore/env"
)

func waitForChanWithTimeout(channel <-chan error, timeout time.Duration) error {
	select {
	case err := <-channel:
		return err
	case <-time.After(timeout):
		return nil
	}
}

func sendInitStartedResponse(responseChannel chan<- interop.InitStarted, msg interop.InitStarted) {
	msg.Ack = make(chan struct{})
	responseChannel <- msg
	<-msg.Ack
}

func sendInitSuccessResponse(responseChannel chan<- interop.InitSuccess, msg interop.InitSuccess) {
	msg.Ack = make(chan struct{})
	responseChannel <- msg
	<-msg.Ack
}

func sendInitFailureResponse(responseChannel chan<- interop.InitFailure, msg interop.InitFailure) {
	msg.Ack = make(chan struct{})
	responseChannel <- msg
	<-msg.Ack
}

type mockRapidCtx struct {
	initHandler   func(start chan<- interop.InitStarted, success chan<- interop.InitSuccess, fail chan<- interop.InitFailure)
	invokeHandler func() (interop.InvokeSuccess, *interop.InvokeFailure)
	resetHandler  func() (interop.ResetSuccess, *interop.ResetFailure)
}

func (r *mockRapidCtx) HandleInit(init *interop.Init, startResp chan<- interop.InitStarted, successResp chan<- interop.InitSuccess, failureResp chan<- interop.InitFailure) {
	r.initHandler(startResp, successResp, failureResp)
}

func (r *mockRapidCtx) HandleInvoke(invoke *interop.Invoke, sbInfoFromInit interop.SandboxInfoFromInit) (interop.InvokeSuccess, *interop.InvokeFailure) {
	return r.invokeHandler()
}

func (r *mockRapidCtx) HandleReset(reset *interop.Reset, invokeReceivedTime int64, invokeResponseMetrics *interop.InvokeResponseMetrics) (interop.ResetSuccess, *interop.ResetFailure) {
	return r.resetHandler()
}

func (r *mockRapidCtx) HandleShutdown(shutdown *interop.Shutdown) interop.ShutdownSuccess {
	return interop.ShutdownSuccess{}
}

func (r *mockRapidCtx) HandleRestore(restore *interop.Restore) error {
	return nil
}

func (r *mockRapidCtx) Clear() {}

func TestReserveDoesNotDeadlockWhenCalledMultipleTimes(t *testing.T) {
	srv := NewServer(context.Background())
	srv.SetInternalStateGetter(func() statejson.InternalStateDescription { return statejson.InternalStateDescription{} })

	initHandler := func(startResp chan<- interop.InitStarted, successResp chan<- interop.InitSuccess, failureResp chan<- interop.InitFailure) {
		sendInitStartedResponse(startResp, interop.InitStarted{})
		sendInitSuccessResponse(successResp, interop.InitSuccess{})
	}
	srv.SetSandboxContext(&SandboxContext{&mockRapidCtx{
		initHandler,
		func() (interop.InvokeSuccess, *interop.InvokeFailure) { return interop.InvokeSuccess{}, nil },
		func() (interop.ResetSuccess, *interop.ResetFailure) { return interop.ResetSuccess{}, nil },
	}, "handler", "runtimeAPIhost:999", 0, &interop.InvokeResponseMetrics{}})

	srv.Init(&interop.Init{EnvironmentVariables: env.NewEnvironment()}, int64(1*time.Second*time.Millisecond))

	_, err := srv.Reserve("", "", "") // reserve successfully
	require.NoError(t, err)

	resp, err := srv.Reserve("", "", "") // attempt double reservation
	require.Nil(t, resp)
	require.Equal(t, ErrAlreadyReserved, err)

	successChan := make(chan error)
	go func() {
		resp, err := srv.Reserve("", "", "")
		require.Nil(t, resp)
		require.Equal(t, ErrAlreadyReserved, err)
		successChan <- nil
	}()

	select {
	case <-time.After(1 * time.Second):
		require.Fail(t, "Timed out while waiting for Reserve() response")
	case <-successChan:
	}
}

func TestInitSuccess(t *testing.T) {
	srv := NewServer(context.Background())
	srv.SetInternalStateGetter(func() statejson.InternalStateDescription { return statejson.InternalStateDescription{} })

	initHandler := func(startResp chan<- interop.InitStarted, successResp chan<- interop.InitSuccess, failureResp chan<- interop.InitFailure) {
		sendInitStartedResponse(startResp, interop.InitStarted{})
		sendInitSuccessResponse(successResp, interop.InitSuccess{})
	}
	srv.SetSandboxContext(&SandboxContext{&mockRapidCtx{
		initHandler,
		func() (interop.InvokeSuccess, *interop.InvokeFailure) { return interop.InvokeSuccess{}, nil },
		func() (interop.ResetSuccess, *interop.ResetFailure) { return interop.ResetSuccess{}, nil },
	}, "handler", "runtimeAPIhost:999", 0, &interop.InvokeResponseMetrics{}})

	srv.Init(&interop.Init{EnvironmentVariables: env.NewEnvironment()}, int64(1*time.Second*time.Millisecond))
	require.Equal(t, phaseInitializing, srv.getRapidPhase())

	_, err := srv.Reserve("", "", "")
	require.NoError(t, err)
}

func TestInitErrorBeforeReserve(t *testing.T) {
	// Rapid thread sending init failure should not be blocked even if reserve hasn't arrived
	srv := NewServer(context.Background())
	srv.SetInternalStateGetter(func() statejson.InternalStateDescription { return statejson.InternalStateDescription{} })

	initErrorResponseSent := make(chan error)
	initHandler := func(startResp chan<- interop.InitStarted, successResp chan<- interop.InitSuccess, failureResp chan<- interop.InitFailure) {
		sendInitStartedResponse(startResp, interop.InitStarted{})
		require.NoError(t, srv.SendInitErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'A.B' }")}))
		sendInitFailureResponse(failureResp, interop.InitFailure{})
		initErrorResponseSent <- errors.New("initErrorResponseSent")
	}
	srv.SetSandboxContext(&SandboxContext{&mockRapidCtx{
		initHandler,
		func() (interop.InvokeSuccess, *interop.InvokeFailure) { return interop.InvokeSuccess{}, nil },
		func() (interop.ResetSuccess, *interop.ResetFailure) { return interop.ResetSuccess{}, nil },
	}, "handler", "runtimeAPIhost:999", 0, &interop.InvokeResponseMetrics{}})

	srv.Init(&interop.Init{EnvironmentVariables: env.NewEnvironment()}, int64(1*time.Second*time.Millisecond))

	if msg := waitForChanWithTimeout(initErrorResponseSent, 1*time.Second); msg == nil {
		require.Fail(t, "Timed out waiting for init error response to be sent")
	}

	resp, err := srv.Reserve("", "", "")
	require.NoError(t, err)
	require.True(t, len(resp.Token.InvokeID) > 0)

	awaitInitErr := srv.AwaitInitialized()
	require.Error(t, ErrInitDoneFailed, awaitInitErr)

	_, err = srv.AwaitRelease()
	require.Error(t, err, ErrReleaseReservationDone)
	require.Equal(t, runtimeState(runtimeInvokeComplete), srv.getRuntimeState())
}

func TestInitErrorDuringReserve(t *testing.T) {
	srv := NewServer(context.Background())
	srv.SetInternalStateGetter(func() statejson.InternalStateDescription { return statejson.InternalStateDescription{} })

	initHandler := func(startResp chan<- interop.InitStarted, successResp chan<- interop.InitSuccess, failureResp chan<- interop.InitFailure) {
		sendInitStartedResponse(startResp, interop.InitStarted{})
		require.NoError(t, srv.SendInitErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'A.B' }")}))
		sendInitFailureResponse(failureResp, interop.InitFailure{})
	}
	srv.SetSandboxContext(&SandboxContext{&mockRapidCtx{
		initHandler,
		func() (interop.InvokeSuccess, *interop.InvokeFailure) { return interop.InvokeSuccess{}, nil },
		func() (interop.ResetSuccess, *interop.ResetFailure) { return interop.ResetSuccess{}, nil },
	}, "handler", "runtimeAPIhost:999", 0, &interop.InvokeResponseMetrics{}})

	srv.Init(&interop.Init{EnvironmentVariables: env.NewEnvironment()}, int64(1*time.Second*time.Millisecond))
	resp, err := srv.Reserve("", "", "")
	require.NoError(t, err)
	require.True(t, len(resp.Token.InvokeID) > 0)

	awaitInitErr := srv.AwaitInitialized()
	require.Error(t, ErrInitDoneFailed, awaitInitErr)

	_, err = srv.AwaitRelease()
	require.Error(t, err, ErrReleaseReservationDone)
	require.Equal(t, runtimeState(runtimeInvokeComplete), srv.getRuntimeState())
}

func TestInvokeSuccess(t *testing.T) {
	srv := NewServer(context.Background())
	srv.SetInternalStateGetter(func() statejson.InternalStateDescription { return statejson.InternalStateDescription{} })

	releaseRuntimeInit := make(chan struct{})
	initHandler := func(startResp chan<- interop.InitStarted, successResp chan<- interop.InitSuccess, failureResp chan<- interop.InitFailure) {
		sendInitStartedResponse(startResp, interop.InitStarted{})
		<-releaseRuntimeInit
		sendInitSuccessResponse(successResp, interop.InitSuccess{})
	}

	invokeHandler := func() (interop.InvokeSuccess, *interop.InvokeFailure) {
		require.NoError(t, srv.SendResponse(srv.GetCurrentInvokeID(), map[string]string{"Content-Type": "application/json"}, bytes.NewReader([]byte("response")), nil, nil))
		require.NoError(t, srv.SendRuntimeReady())
		return interop.InvokeSuccess{}, nil
	}

	resetHandler := func() (interop.ResetSuccess, *interop.ResetFailure) { return interop.ResetSuccess{}, nil }
	srv.SetSandboxContext(&SandboxContext{&mockRapidCtx{initHandler, invokeHandler, resetHandler}, "handler", "runtimeAPIhost:999", 0, &interop.InvokeResponseMetrics{}})

	srv.Init(&interop.Init{EnvironmentVariables: env.NewEnvironment()}, int64(1*time.Second*time.Millisecond))
	require.Equal(t, phaseInitializing, srv.getRapidPhase())
	releaseRuntimeInit <- struct{}{}

	_, err := srv.Reserve("", "", "")
	require.NoError(t, err)
	require.Equal(t, phaseInitializing, srv.getRapidPhase()) // Reserve does not wait for init completion

	awaitInitErr := srv.AwaitInitialized()
	require.NoError(t, awaitInitErr)

	responseRecorder := httptest.NewRecorder()
	invokeErr := srv.FastInvoke(responseRecorder, &interop.Invoke{}, false)
	require.NoError(t, invokeErr)
	require.Equal(t, "response", responseRecorder.Body.String())
	require.Equal(t, "application/json", responseRecorder.Result().Header.Get("Content-Type"))

	_, err = srv.AwaitRelease()
	require.NoError(t, err)
	require.Equal(t, runtimeState(runtimeInvokeComplete), srv.getRuntimeState())
}

func TestInvokeError(t *testing.T) {
	srv := NewServer(context.Background())
	srv.SetInternalStateGetter(func() statejson.InternalStateDescription { return statejson.InternalStateDescription{} })

	initHandler := func(startResp chan<- interop.InitStarted, successResp chan<- interop.InitSuccess, failureResp chan<- interop.InitFailure) {
		sendInitStartedResponse(startResp, interop.InitStarted{})
		sendInitSuccessResponse(successResp, interop.InitSuccess{})
	}

	invokeHandler := func() (interop.InvokeSuccess, *interop.InvokeFailure) {
		require.NoError(t, srv.SendErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'A.B' }"), ContentType: "application/json"}))
		require.NoError(t, srv.SendRuntimeReady())
		return interop.InvokeSuccess{}, nil
	}

	resetHandler := func() (interop.ResetSuccess, *interop.ResetFailure) {
		return interop.ResetSuccess{}, nil
	}

	srv.SetSandboxContext(&SandboxContext{&mockRapidCtx{initHandler, invokeHandler, resetHandler}, "handler", "runtimeAPIhost:999", 0, &interop.InvokeResponseMetrics{}})

	srv.Init(&interop.Init{EnvironmentVariables: env.NewEnvironment()}, int64(1*time.Second*time.Millisecond))
	require.Equal(t, phaseInitializing, srv.getRapidPhase())

	_, err := srv.Reserve("", "", "")
	require.NoError(t, err)
	require.Equal(t, phaseInitializing, srv.getRapidPhase())

	awaitInitErr := srv.AwaitInitialized()
	require.NoError(t, awaitInitErr)

	responseRecorder := httptest.NewRecorder()
	invokeErr := srv.FastInvoke(responseRecorder, &interop.Invoke{}, false)
	require.NoError(t, invokeErr)
	require.Equal(t, "{ 'errorType': 'A.B' }", responseRecorder.Body.String())
	require.Equal(t, "application/json", responseRecorder.Result().Header.Get("Content-Type"))

	_, err = srv.AwaitRelease()
	require.NoError(t, err)
	require.Equal(t, runtimeState(runtimeInvokeComplete), srv.getRuntimeState())
}

func TestInvokeWithSuppressedInitSuccess(t *testing.T) {
	// Tests an init/error followed by suppressed init:
	// Runtime may have called init/error before Reserve, in which case we
	// expect a suppressed init, i.e. init during the invoke.
	// The first Reserve() after init/error returns ErrInitError because
	// SendDoneFail was called on init/error.
	// We expect the caller to then call Reset() to prepare for suppressed init,
	// followed by Reserve() so that a valid reservation context is available.
	// Reserve() returns ErrInitAlreadyDone, since the server implementation
	// closes the InitDone channel after the first InitDone message.

	srv := NewServer(context.Background())
	srv.SetInternalStateGetter(func() statejson.InternalStateDescription { return statejson.InternalStateDescription{} })

	initErrorCompleted := make(chan error)
	initHandler := func(startResp chan<- interop.InitStarted, successResp chan<- interop.InitSuccess, failureResp chan<- interop.InitFailure) {
		sendInitStartedResponse(startResp, interop.InitStarted{})
		require.NoError(t, srv.SendInitErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'A.B' }")}))
		sendInitFailureResponse(failureResp, interop.InitFailure{})
		initErrorCompleted <- errors.New("initErrorSequenceCompleted")
	}

	invokeHandler := func() (interop.InvokeSuccess, *interop.InvokeFailure) {
		require.NoError(t, srv.SendResponse(srv.GetCurrentInvokeID(), nil, bytes.NewReader([]byte("response")), nil, nil))
		return interop.InvokeSuccess{}, nil
	}

	resetHandler := func() (interop.ResetSuccess, *interop.ResetFailure) {
		return interop.ResetSuccess{}, nil
	}

	srv.SetSandboxContext(&SandboxContext{&mockRapidCtx{initHandler, invokeHandler, resetHandler}, "handler", "runtimeAPIhost:999", 0, &interop.InvokeResponseMetrics{}})

	srv.Init(&interop.Init{EnvironmentVariables: env.NewEnvironment()}, int64(1*time.Second*time.Millisecond))
	require.Equal(t, phaseInitializing, srv.getRapidPhase())

	if msg := waitForChanWithTimeout(initErrorCompleted, 1*time.Second); msg == nil {
		require.Fail(t, "Timed out waiting for init error sequence to be called")
	}

	resp, err := srv.Reserve("", "", "")
	require.NoError(t, err)
	require.True(t, len(resp.Token.InvokeID) > 0)

	awaitInitErr := srv.AwaitInitialized()
	require.Error(t, ErrInitDoneFailed, awaitInitErr)

	_, err = srv.Reset(autoresetReasonReserveFail, resetDefaultTimeoutMs) // prepare for suppressed init
	require.NoError(t, err)

	_, err = srv.Reserve("", "", "")
	require.NoError(t, err)

	responseRecorder := httptest.NewRecorder()
	successChan := make(chan error)
	go func() {
		directInvoke := false
		invokeErr := srv.FastInvoke(responseRecorder, &interop.Invoke{}, directInvoke)
		require.NoError(t, invokeErr)
		successChan <- errors.New("invokeResponseWritten")
	}()

	invokeErr := waitForChanWithTimeout(successChan, 1*time.Second)
	if invokeErr == nil {
		require.Fail(t, "Timed out while waiting for invoke response")
	}

	require.Equal(t, "response", responseRecorder.Body.String())

	_, err = srv.AwaitRelease()
	require.NoError(t, err)
	require.Equal(t, runtimeState(runtimeInvokeComplete), srv.getRuntimeState())
}

func TestInvokeWithSuppressedInitErrorDueToInitError(t *testing.T) {
	// Tests init/error followed by init/error during suppressed init
	srv := NewServer(context.Background())
	srv.SetInternalStateGetter(func() statejson.InternalStateDescription { return statejson.InternalStateDescription{} })

	initHandler := func(startResp chan<- interop.InitStarted, successResp chan<- interop.InitSuccess, failureResp chan<- interop.InitFailure) {
		sendInitStartedResponse(startResp, interop.InitStarted{})
		require.NoError(t, srv.SendInitErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'A.B' }")}))
		sendInitFailureResponse(failureResp, interop.InitFailure{})
	}

	releaseChan := make(chan error)
	invokeHandler := func() (interop.InvokeSuccess, *interop.InvokeFailure) {
		require.NoError(t, srv.SendErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'A.B' }")}))
		releaseChan <- nil
		return interop.InvokeSuccess{}, &interop.InvokeFailure{ErrorType: "A.B", RequestReset: true, DefaultErrorResponse: &interop.ErrorResponse{}}
	}

	resetHandler := func() (interop.ResetSuccess, *interop.ResetFailure) {
		return interop.ResetSuccess{}, nil
	}

	srv.SetSandboxContext(&SandboxContext{&mockRapidCtx{initHandler, invokeHandler, resetHandler}, "handler", "runtimeAPIhost:999", 0, &interop.InvokeResponseMetrics{}})

	srv.Init(&interop.Init{EnvironmentVariables: env.NewEnvironment()}, int64(1*time.Second*time.Millisecond))

	resp, err := srv.Reserve("", "", "")
	require.NoError(t, err)
	require.True(t, len(resp.Token.InvokeID) > 0)
	require.Equal(t, phaseInitializing, srv.getRapidPhase())

	awaitInitErr := srv.AwaitInitialized()
	require.Error(t, ErrInitDoneFailed, awaitInitErr)

	_, err = srv.Reset(autoresetReasonReserveFail, resetDefaultTimeoutMs) // prepare for invoke with suppressed init
	require.NoError(t, err)
	require.Equal(t, phaseIdle, srv.getRapidPhase())

	_, err = srv.Reserve("", "", "")
	require.NoError(t, err)
	require.Equal(t, phaseIdle, srv.getRapidPhase())

	responseRecorder := httptest.NewRecorder()
	invokeErr := srv.FastInvoke(responseRecorder, &interop.Invoke{}, false)
	require.NoError(t, invokeErr)
	require.Equal(t, "{ 'errorType': 'A.B' }", responseRecorder.Body.String())
	require.Equal(t, phaseInvoking, srv.getRapidPhase())

	<-releaseChan // Unblock gorotune to send donefail
	_, err = srv.AwaitRelease()
	require.EqualError(t, err, ErrInvokeDoneFailed.Error())
	require.Equal(t, phaseIdle, srv.getRapidPhase())
	require.Equal(t, runtimeState(runtimeInvokeComplete), srv.getRuntimeState())
}

func TestInvokeWithSuppressedInitErrorDueToInvokeError(t *testing.T) {
	// Tests init/error followed by init/error during suppressed init
	srv := NewServer(context.Background())
	srv.SetInternalStateGetter(func() statejson.InternalStateDescription { return statejson.InternalStateDescription{} })

	initHandler := func(startResp chan<- interop.InitStarted, successResp chan<- interop.InitSuccess, failureResp chan<- interop.InitFailure) {
		sendInitStartedResponse(startResp, interop.InitStarted{})
		require.NoError(t, srv.SendInitErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'A.B' }")}))
		sendInitFailureResponse(failureResp, interop.InitFailure{})
	}
	invokeHandler := func() (interop.InvokeSuccess, *interop.InvokeFailure) {
		require.NoError(t, srv.SendInitErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'B.C' }")}))
		require.NoError(t, srv.SendRuntimeReady())
		return interop.InvokeSuccess{}, nil
	}

	resetHandler := func() (interop.ResetSuccess, *interop.ResetFailure) {
		return interop.ResetSuccess{}, nil
	}

	srv.SetSandboxContext(&SandboxContext{&mockRapidCtx{initHandler, invokeHandler, resetHandler}, "handler", "runtimeAPIhost:999", 0, &interop.InvokeResponseMetrics{}})

	srv.Init(&interop.Init{EnvironmentVariables: env.NewEnvironment()}, int64(1*time.Second*time.Millisecond))
	require.Equal(t, phaseInitializing, srv.getRapidPhase())

	resp, err := srv.Reserve("", "", "")
	require.NoError(t, err)
	require.True(t, len(resp.Token.InvokeID) > 0)

	awaitInitErr := srv.AwaitInitialized()
	require.Error(t, ErrInitDoneFailed, awaitInitErr)

	_, err = srv.Reset(autoresetReasonReserveFail, resetDefaultTimeoutMs) // prepare for invoke with suppressed init
	require.NoError(t, err)
	require.Equal(t, phaseIdle, srv.getRapidPhase())

	_, err = srv.Reserve("", "", "")
	require.NoError(t, err)
	require.Equal(t, phaseIdle, srv.getRapidPhase())

	responseRecorder := httptest.NewRecorder()
	invokeErr := srv.FastInvoke(responseRecorder, &interop.Invoke{}, false)
	require.NoError(t, invokeErr)
	require.Equal(t, "{ 'errorType': 'B.C' }", responseRecorder.Body.String())

	_, err = srv.AwaitRelease()
	require.NoError(t, err) // /invocation/error -> /invocation/next returns no error / donefail
	require.Equal(t, phaseIdle, srv.getRapidPhase())
	require.Equal(t, runtimeState(runtimeInvokeComplete), srv.getRuntimeState())
}

func TestMultipleInvokeSuccess(t *testing.T) {
	srv := NewServer(context.Background())
	srv.SetInternalStateGetter(func() statejson.InternalStateDescription { return statejson.InternalStateDescription{} })

	initHandler := func(startResp chan<- interop.InitStarted, successResp chan<- interop.InitSuccess, failureResp chan<- interop.InitFailure) {
		sendInitStartedResponse(startResp, interop.InitStarted{})
		sendInitSuccessResponse(successResp, interop.InitSuccess{})
	}
	i := 0
	invokeHandler := func() (interop.InvokeSuccess, *interop.InvokeFailure) {
		require.NoError(t, srv.SendResponse(srv.GetCurrentInvokeID(), nil, bytes.NewReader([]byte("response-"+fmt.Sprint(i))), nil, nil))
		require.NoError(t, srv.SendRuntimeReady())
		i++
		return interop.InvokeSuccess{}, nil
	}

	resetHandler := func() (interop.ResetSuccess, *interop.ResetFailure) {
		return interop.ResetSuccess{}, nil
	}

	srv.SetSandboxContext(&SandboxContext{&mockRapidCtx{initHandler, invokeHandler, resetHandler}, "handler", "runtimeAPIhost:999", 0, &interop.InvokeResponseMetrics{}})

	srv.Init(&interop.Init{EnvironmentVariables: env.NewEnvironment()}, int64(1*time.Second*time.Millisecond))
	require.Equal(t, phaseInitializing, srv.getRapidPhase())

	for i := 0; i < 3; i++ {
		_, err := srv.Reserve("", "", "")
		require.NoError(t, err)

		awaitInitErr := srv.AwaitInitialized()
		require.NoError(t, awaitInitErr)

		responseRecorder := httptest.NewRecorder()
		invokeErr := srv.FastInvoke(responseRecorder, &interop.Invoke{}, false)
		require.NoError(t, invokeErr)
		require.Equal(t, "response-"+fmt.Sprint(i), responseRecorder.Body.String())
		require.Equal(t, phaseInvoking, srv.getRapidPhase())

		_, err = srv.AwaitRelease()
		require.NoError(t, err)
		require.Equal(t, phaseIdle, srv.getRapidPhase())
		require.Equal(t, runtimeState(runtimeInvokeComplete), srv.getRuntimeState())
	}
}

/* Unit tests remaining:
- Shutdown behaviour
- Reset behaviour during various phases
- Runtime / extensions process exit sequences
- Invoke() and Init() api tests

See PlantUML state diagram for potential other uncovered paths
through the state machine
*/
