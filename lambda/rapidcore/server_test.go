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
)

func waitForChanWithTimeout(channel <-chan error, timeout time.Duration) error {
	select {
	case err := <-channel:
		return err
	case <-time.After(timeout):
		return nil
	}
}

func TestReserveDoesNotDeadlockWhenCalledMultipleTimes(t *testing.T) {
	srv := NewServer(context.Background())
	srv.SetInternalStateGetter(func() statejson.InternalStateDescription { return statejson.InternalStateDescription{} })

	go func() { <-srv.StartChan() }()
	go srv.SendRunning(&interop.Running{})
	srv.Init(&interop.Start{CorrelationID: "initCorrelationID"}, int64(1*time.Second*time.Millisecond))

	go srv.SendDone(&interop.Done{CorrelationID: "initCorrelationID"})
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

	go func() {
		<-srv.StartChan()
		require.NoError(t, srv.SendRunning(&interop.Running{}))
		require.NoError(t, srv.SendDone(&interop.Done{CorrelationID: "initCorrelationID"}))
	}()

	srv.Init(&interop.Start{CorrelationID: "initCorrelationID"}, int64(1*time.Second*time.Millisecond))
	require.Equal(t, phaseInitializing, srv.getRapidPhase())

	_, err := srv.Reserve("", "", "")
	require.NoError(t, err)
	require.Equal(t, phaseIdle, srv.getRapidPhase())
	require.Equal(t, runtimeState(runtimeInitComplete), srv.getRuntimeState())
}

func TestInitErrorBeforeReserve(t *testing.T) {
	srv := NewServer(context.Background())
	srv.SetInternalStateGetter(func() statejson.InternalStateDescription { return statejson.InternalStateDescription{} })

	initErrorResponseSent := make(chan error)
	go func() {
		<-srv.StartChan()
		require.NoError(t, srv.SendRunning(&interop.Running{}))
		require.NoError(t, srv.SendErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'A.B' }")}))
		require.NoError(t, srv.SendDoneFail(&interop.DoneFail{CorrelationID: "initCorrelationID", ErrorType: "foobar"}))
		initErrorResponseSent <- errors.New("initErrorResponseSent")
	}()

	srv.Init(&interop.Start{CorrelationID: "initCorrelationID"}, int64(1*time.Second*time.Millisecond))

	if msg := waitForChanWithTimeout(initErrorResponseSent, 1*time.Second); msg == nil {
		require.Fail(t, "Timed out waiting for init error response to be sent")
	}

	resp, err := srv.Reserve("", "", "")
	require.EqualError(t, err, ErrInitError.Error())
	require.True(t, len(resp.Token.InvokeID) > 0)
	require.Equal(t, runtimeState(runtimeInitFailed), srv.getRuntimeState())
}

func TestInitErrorDuringReserve(t *testing.T) {
	srv := NewServer(context.Background())
	srv.SetInternalStateGetter(func() statejson.InternalStateDescription { return statejson.InternalStateDescription{} })

	go func() {
		<-srv.StartChan()
		require.NoError(t, srv.SendRunning(&interop.Running{}))
		require.NoError(t, srv.SendErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'A.B' }")}))
		require.NoError(t, srv.SendDoneFail(&interop.DoneFail{CorrelationID: "initCorrelationID", ErrorType: "foobar"}))
	}()

	srv.Init(&interop.Start{CorrelationID: "initCorrelationID"}, int64(1*time.Second*time.Millisecond))
	resp, err := srv.Reserve("", "", "")
	require.EqualError(t, err, ErrInitError.Error())
	require.True(t, len(resp.Token.InvokeID) > 0)
	require.Equal(t, runtimeState(runtimeInitFailed), srv.getRuntimeState())
}

func TestInvokeSuccess(t *testing.T) {
	srv := NewServer(context.Background())
	srv.SetInternalStateGetter(func() statejson.InternalStateDescription { return statejson.InternalStateDescription{} })

	go func() {
		<-srv.StartChan()
		require.NoError(t, srv.SendRunning(&interop.Running{}))
		require.NoError(t, srv.SendDone(&interop.Done{CorrelationID: "initCorrelationID"}))

		<-srv.InvokeChan()
		require.NoError(t, srv.SendResponse(srv.GetCurrentInvokeID(), "application/json", bytes.NewReader([]byte("response"))))
		require.NoError(t, srv.SendRuntimeReady())
		require.NoError(t, srv.SendDone(&interop.Done{CorrelationID: "invokeCorrelationID"}))
	}()

	srv.Init(&interop.Start{CorrelationID: "initCorrelationID"}, int64(1*time.Second*time.Millisecond))
	require.Equal(t, phaseInitializing, srv.getRapidPhase())

	_, err := srv.Reserve("", "", "")
	require.NoError(t, err)
	require.Equal(t, phaseIdle, srv.getRapidPhase())
	require.Equal(t, runtimeState(runtimeInitComplete), srv.getRuntimeState())

	responseRecorder := httptest.NewRecorder()
	invokeErr := srv.FastInvoke(responseRecorder, &interop.Invoke{CorrelationID: "invokeCorrelationID"}, false)
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

	go func() {
		<-srv.StartChan()
		require.NoError(t, srv.SendRunning(&interop.Running{}))
		require.NoError(t, srv.SendDone(&interop.Done{CorrelationID: "initCorrelationID"}))

		<-srv.InvokeChan()

		require.NoError(t, srv.SendErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'A.B' }"), ContentType: "application/json"}))
		require.NoError(t, srv.SendRuntimeReady())
		require.NoError(t, srv.SendDone(&interop.Done{CorrelationID: "invokeCorrelationID"}))
	}()

	srv.Init(&interop.Start{CorrelationID: "initCorrelationID"}, int64(1*time.Second*time.Millisecond))
	require.Equal(t, phaseInitializing, srv.getRapidPhase())

	_, err := srv.Reserve("", "", "")
	require.NoError(t, err)
	require.Equal(t, phaseIdle, srv.getRapidPhase())
	require.Equal(t, runtimeState(runtimeInitComplete), srv.getRuntimeState())

	responseRecorder := httptest.NewRecorder()
	invokeErr := srv.FastInvoke(responseRecorder, &interop.Invoke{CorrelationID: "invokeCorrelationID"}, false)
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
	go func() {
		<-srv.StartChan()
		require.NoError(t, srv.SendRunning(&interop.Running{}))
		require.NoError(t, srv.SendErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'A.B' }")}))
		require.NoError(t, srv.SendDoneFail(&interop.DoneFail{CorrelationID: "initCorrelationID", ErrorType: "foobar"}))
		initErrorCompleted <- errors.New("initErrorSequenceCompleted")

		<-srv.ResetChan()
		require.NoError(t, srv.SendDone(&interop.Done{CorrelationID: "resetCorrelationID"}))

		<-srv.InvokeChan() // run only after FastInvoke is called
		require.NoError(t, srv.SendResponse(srv.GetCurrentInvokeID(), "", bytes.NewReader([]byte("response"))))
		require.NoError(t, srv.SendDone(&interop.Done{CorrelationID: "invokeCorrelationID"}))
	}()

	srv.Init(&interop.Start{CorrelationID: "initCorrelationID"}, int64(1*time.Second*time.Millisecond))
	require.Equal(t, phaseInitializing, srv.getRapidPhase())

	if msg := waitForChanWithTimeout(initErrorCompleted, 1*time.Second); msg == nil {
		require.Fail(t, "Timed out waiting for init error sequence to be called")
	}

	_, err := srv.Reserve("", "", "")
	require.EqualError(t, err, ErrInitError.Error())
	require.Equal(t, runtimeState(runtimeInitFailed), srv.getRuntimeState())

	_, err = srv.Reset(autoresetReasonReserveFail, resetDefaultTimeoutMs) // prepare for suppressed init
	require.NoError(t, err)

	_, err = srv.Reserve("", "", "")
	require.EqualError(t, err, ErrInitAlreadyDone.Error())

	responseRecorder := httptest.NewRecorder()
	successChan := make(chan error)
	go func() {
		directInvoke := false
		invokeErr := srv.FastInvoke(responseRecorder, &interop.Invoke{CorrelationID: "invokeCorrelationID"}, directInvoke)
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

	releaseChan := make(chan error)
	go func() {
		<-srv.StartChan()
		require.NoError(t, srv.SendRunning(&interop.Running{}))
		require.NoError(t, srv.SendErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'A.B' }")}))
		require.NoError(t, srv.SendDoneFail(&interop.DoneFail{CorrelationID: "initCorrelationID", ErrorType: "A.B"}))

		<-srv.ResetChan()
		srv.SendDone(&interop.Done{CorrelationID: "resetCorrelationID"})

		<-srv.InvokeChan()
		require.NoError(t, srv.SendErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'A.B' }")}))
		releaseChan <- nil
		require.NoError(t, srv.SendDoneFail(&interop.DoneFail{CorrelationID: "invokeCorrelationID", ErrorType: "A.B"}))
	}()

	srv.Init(&interop.Start{CorrelationID: "initCorrelationID"}, int64(1*time.Second*time.Millisecond))

	_, err := srv.Reserve("", "", "")
	require.EqualError(t, err, ErrInitError.Error())
	require.Equal(t, phaseIdle, srv.getRapidPhase())
	require.Equal(t, runtimeState(runtimeInitFailed), srv.getRuntimeState())

	_, err = srv.Reset(autoresetReasonReserveFail, resetDefaultTimeoutMs) // prepare for invoke with suppressed init
	require.NoError(t, err)
	require.Equal(t, phaseIdle, srv.getRapidPhase())

	_, err = srv.Reserve("", "", "")
	require.EqualError(t, err, ErrInitAlreadyDone.Error())
	require.Equal(t, phaseIdle, srv.getRapidPhase())

	responseRecorder := httptest.NewRecorder()
	invokeErr := srv.FastInvoke(responseRecorder, &interop.Invoke{CorrelationID: "invokeCorrelationID"}, false)
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

	go func() {
		<-srv.StartChan()
		require.NoError(t, srv.SendRunning(&interop.Running{}))
		require.NoError(t, srv.SendErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'A.B' }")}))
		require.NoError(t, srv.SendDoneFail(&interop.DoneFail{CorrelationID: "initCorrelationID", ErrorType: "A.B"}))

		<-srv.ResetChan()
		srv.SendDone(&interop.Done{CorrelationID: "resetCorrelationID"})

		<-srv.InvokeChan()
		require.NoError(t, srv.SendErrorResponse(srv.GetCurrentInvokeID(), &interop.ErrorResponse{Payload: []byte("{ 'errorType': 'B.C' }")}))
		require.NoError(t, srv.SendRuntimeReady())
		require.NoError(t, srv.SendDone(&interop.Done{CorrelationID: "invokeCorrelationID"}))
	}()

	srv.Init(&interop.Start{CorrelationID: "initCorrelationID"}, int64(1*time.Second*time.Millisecond))
	require.Equal(t, phaseInitializing, srv.getRapidPhase())

	_, err := srv.Reserve("", "", "")
	require.EqualError(t, err, ErrInitError.Error())
	require.Equal(t, phaseIdle, srv.getRapidPhase())
	require.Equal(t, runtimeState(runtimeInitFailed), srv.getRuntimeState())

	_, err = srv.Reset(autoresetReasonReserveFail, resetDefaultTimeoutMs) // prepare for invoke with suppressed init
	require.NoError(t, err)
	require.Equal(t, phaseIdle, srv.getRapidPhase())

	_, err = srv.Reserve("", "", "")
	require.EqualError(t, err, ErrInitAlreadyDone.Error())
	require.Equal(t, phaseIdle, srv.getRapidPhase())

	responseRecorder := httptest.NewRecorder()
	invokeErr := srv.FastInvoke(responseRecorder, &interop.Invoke{CorrelationID: "invokeCorrelationID"}, false)
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

	go func() {
		<-srv.StartChan()
		require.NoError(t, srv.SendRunning(&interop.Running{}))
		require.NoError(t, srv.SendDone(&interop.Done{CorrelationID: "initCorrelationID"}))
	}()

	srv.Init(&interop.Start{CorrelationID: "initCorrelationID"}, int64(1*time.Second*time.Millisecond))
	require.Equal(t, phaseInitializing, srv.getRapidPhase())

	invokeFunc := func(i int) {
		<-srv.InvokeChan()
		require.NoError(t, srv.SendResponse(srv.GetCurrentInvokeID(), "", bytes.NewReader([]byte("response-"+fmt.Sprint(i)))))
		require.NoError(t, srv.SendRuntimeReady())
		require.NoError(t, srv.SendDone(&interop.Done{CorrelationID: "invokeCorrelationID"}))
	}
	go func() {
		for i := 0; i < 3; i++ {
			invokeFunc(i)
		}
	}()

	for i := 0; i < 3; i++ {
		_, err := srv.Reserve("", "", "")
		require.Contains(t, []error{nil, ErrInitAlreadyDone}, err)
		require.Equal(t, phaseIdle, srv.getRapidPhase())

		responseRecorder := httptest.NewRecorder()
		invokeErr := srv.FastInvoke(responseRecorder, &interop.Invoke{CorrelationID: "invokeCorrelationID"}, false)
		require.NoError(t, invokeErr)
		require.Equal(t, "response-"+fmt.Sprint(i), responseRecorder.Body.String())

		_, err = srv.AwaitRelease()
		require.NoError(t, err)
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
