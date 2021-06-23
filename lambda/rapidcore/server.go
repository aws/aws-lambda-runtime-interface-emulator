// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"sync"
	"time"

	"go.amzn.com/lambda/core/directinvoke"
	"go.amzn.com/lambda/core/statejson"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/metering"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	autoresetReasonTimeout     = "Timeout"
	autoresetReasonReserveFail = "ReserveFail"
	autoresetReasonReleaseFail = "ReleaseFail"
	standaloneVersionID        = "1"

	resetDefaultTimeoutMs = 2000
)

// rapidPhase tracks the state machine in the go.amzn.com/lambda/rapid receive loop. See
// a state diagram of how the events and states of rapid package and this interop server
type rapidPhase int

const (
	phaseIdle rapidPhase = iota
	phaseInitializing
	phaseInvoking
)

type runtimeState int

const (
	runtimeNotStarted = iota

	runtimeInitStarted
	runtimeInitError
	runtimeInitComplete
	runtimeInitFailed

	runtimeInvokeResponseSent
	runtimeInvokeError
	runtimeReady
	runtimeInvokeComplete
)

type DoneWithState struct {
	*interop.Done
	State statejson.InternalStateDescription
}

func (s *DoneWithState) String() string {
	return fmt.Sprintf("%v %v", *s.Done, string(s.State.AsJSON()))
}

type InvokeContext struct {
	Token       interop.Token
	ReplySent   bool
	ReplyStream http.ResponseWriter
	Direct      bool
}

type Server struct {
	InternalStateGetter interop.InternalStateGetter

	invokeChanOut   chan *interop.Invoke
	startChanOut    chan *interop.Start
	resetChanOut    chan *interop.Reset
	shutdownChanOut chan *interop.Shutdown
	errorChanOut    chan error

	sendRunningChan  chan *interop.Running
	sendResponseChan chan struct{}
	doneChan         chan *interop.Done

	InitDoneChan     chan DoneWithState
	InvokeDoneChan   chan DoneWithState
	ResetDoneChan    chan *interop.Done
	ShutdownDoneChan chan *interop.Done

	mutex         sync.Mutex
	invokeCtx     *InvokeContext
	invokeTimeout time.Duration

	reservationContext context.Context
	reservationCancel  func()

	rapidPhase   rapidPhase
	runtimeState runtimeState
}

func (s *Server) setRapidPhase(phase rapidPhase) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.rapidPhase = phase
}

func (s *Server) getRapidPhase() rapidPhase {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.rapidPhase
}

func (s *Server) setRuntimeState(state runtimeState) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.runtimeState = state
}

func (s *Server) getRuntimeState() runtimeState {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.runtimeState
}

func (s *Server) SetInvokeTimeout(timeout time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.invokeTimeout = timeout
}

func (s *Server) GetInvokeTimeout() time.Duration {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.invokeTimeout
}

func (s *Server) GetInvokeContext() *InvokeContext {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	ctx := *s.invokeCtx
	return &ctx
}

func (s *Server) setNewInvokeContext(invokeID string, traceID, lambdaSegmentID string) (*ReserveResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.invokeCtx != nil {
		return nil, ErrAlreadyReserved
	}

	s.invokeCtx = &InvokeContext{
		Token: interop.Token{
			ReservationToken: uuid.New().String(),
			InvokeID:         invokeID,
			VersionID:        standaloneVersionID,
			FunctionTimeout:  s.invokeTimeout,
			TraceID:          traceID,
			LambdaSegmentID:  lambdaSegmentID,
			InvackDeadlineNs: math.MaxInt64, // no INVACK in standalone
		},
	}

	resp := &ReserveResponse{
		Token: s.invokeCtx.Token,
	}

	s.reservationContext, s.reservationCancel = context.WithCancel(context.Background())

	return resp, nil
}

// Reserve allocates invoke context
func (s *Server) Reserve(id string, traceID, lambdaSegmentID string) (*ReserveResponse, error) {
	invokeID := uuid.New().String()
	if len(id) > 0 {
		invokeID = id
	}
	resp, err := s.setNewInvokeContext(invokeID, traceID, lambdaSegmentID)
	if err != nil {
		return nil, err
	}

	resp.InternalState, err = s.waitInit()
	return resp, err
}

func (s *Server) waitInit() (*statejson.InternalStateDescription, error) {
	for {
		select {

		case doneWithState, chanOpen := <-s.InitDoneChan:
			if !chanOpen {
				// init only happens once
				return nil, ErrInitAlreadyDone
			}

			close(s.InitDoneChan) // this was first call to reserve

			if s.getRuntimeState() == runtimeInitFailed {
				return &doneWithState.State, ErrInitError
			}

			if len(doneWithState.ErrorType) > 0 {
				log.Errorf("INIT DONE failed: %s", doneWithState.ErrorType)
				return &doneWithState.State, ErrInitDoneFailed
			}

			return &doneWithState.State, nil

		case <-s.reservationContext.Done():
			return nil, ErrReserveReservationDone
		}
	}
}

func (s *Server) setReplyStream(w http.ResponseWriter, direct bool) (string, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.invokeCtx == nil {
		return "", ErrNotReserved
	}

	if s.invokeCtx.ReplySent {
		return "", ErrAlreadyReplied
	}

	if s.invokeCtx.ReplyStream != nil {
		return "", ErrAlreadyInvocating
	}

	s.invokeCtx.ReplyStream = w
	s.invokeCtx.Direct = direct
	return s.invokeCtx.Token.InvokeID, nil
}

// Release closes the invocation, making server ready for reserve again
func (s *Server) Release() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.invokeCtx == nil {
		return ErrNotReserved
	}

	if s.reservationCancel != nil {
		s.reservationCancel()
	}

	s.invokeCtx = nil
	return nil
}

// GetCurrentInvokeID
func (s *Server) GetCurrentInvokeID() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.invokeCtx == nil {
		return ""
	}

	return s.invokeCtx.Token.InvokeID
}

// SetInternalStateGetter is used to set callback which returnes internal state for /test/internalState request
func (s *Server) SetInternalStateGetter(cb interop.InternalStateGetter) {
	s.InternalStateGetter = cb
}

// StartChan returns Start emitter
func (s *Server) StartChan() <-chan *interop.Start {
	return s.startChanOut
}

// InvokeChan returns Invoke emitter
func (s *Server) InvokeChan() <-chan *interop.Invoke {
	return s.invokeChanOut
}

// ResetChan returns Reset emitter
func (s *Server) ResetChan() <-chan *interop.Reset {
	return s.resetChanOut
}

// ShutdownChan returns Shutdown emitter
func (s *Server) ShutdownChan() <-chan *interop.Shutdown {
	return s.shutdownChanOut
}

// InvalidMessageChan emits errors if there was something we could not parse
func (s *Server) TransportErrorChan() <-chan error {
	return s.errorChanOut
}

func (s *Server) sendResponseUnsafe(invokeID string, status int, payload io.Reader) error {
	if s.invokeCtx == nil || invokeID != s.invokeCtx.Token.InvokeID {
		return interop.ErrInvalidInvokeID
	}

	if s.invokeCtx.ReplySent {
		return interop.ErrResponseSent
	}

	if s.invokeCtx.ReplyStream == nil {
		return fmt.Errorf("ReplyStream not available")
	}

	// TODO: earlier, status was set to 500 if runtime called /invocation/error. However, the integration
	// tests do not differentiate between /invocation/error and /invocation/response, but they check the error type:
	// To identify user-errors, we should also allowlist custom errortypes and propagate them via headers.

	// s.invokeCtx.ReplyStream.WriteHeader(status)

	if s.invokeCtx.Direct {
		if err := directinvoke.SendDirectInvokeResponse(nil, payload, s.invokeCtx.ReplyStream); err != nil {
			// we intentionally do not return an error here:
			// even if error happened, the response has already been initiated (and might be partially written into the socket)
			// so there is no other option except to consider response to be sent.
			log.Errorf("Failed to write response to %s: %s", invokeID, err)
		}
	} else {
		data, err := ioutil.ReadAll(payload)
		if err != nil {
			return fmt.Errorf("Failed to read response on %s: %s", invokeID, err)
		}
		if len(data) > interop.MaxPayloadSize {
			return &interop.ErrorResponseTooLarge{
				ResponseSize:    len(data),
				MaxResponseSize: interop.MaxPayloadSize,
			}
		}
		if _, err := s.invokeCtx.ReplyStream.Write(data); err != nil {
			return fmt.Errorf("Failed to write response to %s: %s", invokeID, err)
		}
	}

	s.sendResponseChan <- struct{}{}
	s.invokeCtx.ReplySent = true
	s.invokeCtx.Direct = false
	return nil
}

func (s *Server) SendResponse(invokeID string, reader io.Reader) error {
	s.setRuntimeState(runtimeInvokeResponseSent)
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.sendResponseUnsafe(invokeID, http.StatusOK, reader)
}

func (s *Server) CommitResponse() error { return nil }

func (s *Server) SendRunning(run *interop.Running) error {
	s.setRuntimeState(runtimeInitStarted)
	s.sendRunningChan <- run
	return nil
}

func (s *Server) SendErrorResponse(invokeID string, resp *interop.ErrorResponse) error {
	switch s.getRapidPhase() {
	case phaseInitializing:
		s.setRuntimeState(runtimeInitError)
		return nil
	case phaseInvoking:
		// This branch can also occur during a suppressed init error, which is reported as invoke error
		s.setRuntimeState(runtimeInvokeError)
		s.mutex.Lock()
		defer s.mutex.Unlock()
		return s.sendResponseUnsafe(invokeID, http.StatusInternalServerError, bytes.NewReader(resp.Payload))
	default:
		panic("received unexpected error response outside invoke or init phases")
	}
}

func (s *Server) SendDone(done *interop.Done) error {
	s.doneChan <- done
	return nil
}

func (s *Server) SendDoneFail(doneFail *interop.DoneFail) error {
	s.doneChan <- &interop.Done{
		ErrorType:     doneFail.ErrorType,
		CorrelationID: doneFail.CorrelationID, // filipovi: correlationID is required to dispatch message into correct channel
		Meta:          doneFail.Meta,
	}
	return nil
}

func (s *Server) Reset(reason string, timeoutMs int64) (*statejson.ResetDescription, error) {
	// pass reset to rapid
	s.resetChanOut <- &interop.Reset{
		Reason:        reason,
		DeadlineNs:    deadlineNsFromTimeoutMs(timeoutMs),
		CorrelationID: "resetCorrelationID",
	}

	// TODO do not block on reset, instead consume ResetDoneChan in waitForRelease handler,
	// this will get us more aligned on async reset notification handling.
	done := <-s.ResetDoneChan
	s.Release()

	if done.ErrorType != "" {
		return nil, errors.New(string(done.ErrorType))
	}

	return &statejson.ResetDescription{ExtensionsResetMs: done.Meta.ExtensionsResetMs}, nil
}

func NewServer(ctx context.Context) *Server {
	s := &Server{
		startChanOut:    make(chan *interop.Start),
		invokeChanOut:   make(chan *interop.Invoke),
		errorChanOut:    make(chan error),
		resetChanOut:    make(chan *interop.Reset),
		shutdownChanOut: make(chan *interop.Shutdown),

		sendRunningChan:  make(chan *interop.Running),
		sendResponseChan: make(chan struct{}),
		doneChan:         make(chan *interop.Done),

		// These two channels are buffered, because they are depleted asynchronously (by reserve and waitUntilRelease) and we don't want to block in SendDone until they are called
		InitDoneChan:   make(chan DoneWithState, 1),
		InvokeDoneChan: make(chan DoneWithState, 1),

		ResetDoneChan:    make(chan *interop.Done),
		ShutdownDoneChan: make(chan *interop.Done),
	}

	go s.dispatchDone()

	return s
}

func (s *Server) setInitDoneRuntimeState(done *interop.Done) {
	if len(done.ErrorType) > 0 {
		s.setRuntimeState(runtimeInitFailed) // donefail
	} else {
		s.setRuntimeState(runtimeInitComplete) // done
	}
}

// Note, the dispatch loop below has potential to block, when
// channel is not drained. E.g. if test assumes sandbox init
// completion before dispatching reset, then reset will block
// until init channel is drained.
func (s *Server) dispatchDone() {
	for {
		done := <-s.doneChan
		log.Debug("Dispatching DONE:", done.CorrelationID)
		internalState := s.InternalStateGetter()
		s.setRapidPhase(phaseIdle)
		if done.CorrelationID == "initCorrelationID" {
			s.setInitDoneRuntimeState(done)
			s.InitDoneChan <- DoneWithState{Done: done, State: internalState}
		} else if done.CorrelationID == "invokeCorrelationID" {
			s.setRuntimeState(runtimeInvokeComplete)
			s.InvokeDoneChan <- DoneWithState{Done: done, State: internalState}
		} else if done.CorrelationID == "resetCorrelationID" {
			s.setRuntimeState(runtimeNotStarted)
			s.ResetDoneChan <- done
		} else if done.CorrelationID == "shutdownCorrelationID" {
			s.setRuntimeState(runtimeNotStarted)
			s.ShutdownDoneChan <- done
		} else {
			panic("Received DONE without correlation ID")
		}
	}
}

func drainChannel(c chan DoneWithState) {
	for {
		select {
		case dws := <-c:
			log.Warnf("Discard DONE response: %s", dws.String())
			break
		default:
			return
		}
	}
}

func (s *Server) Clear() {
	// we do not drain InitDoneChannel, because Init is only done once during rapid lifetime

	drainChannel(s.InvokeDoneChan)
	s.Release()
}

func (s *Server) IsResponseSent() bool {
	panic("unexpected call to unimplemented method in rapidcore: IsResponseSent()")
}

func (s *Server) SendRuntimeReady() error {
	// only called when extensions are enabled
	s.setRuntimeState(runtimeReady)
	return nil
}

func deadlineNsFromTimeoutMs(timeoutMs int64) int64 {
	mono := metering.Monotime()
	return mono + timeoutMs*1000*1000
}

func (s *Server) Init(i *interop.Start, invokeTimeoutMs int64) {
	s.SetInvokeTimeout(time.Duration(invokeTimeoutMs) * time.Millisecond)

	s.startChanOut <- i
	s.setRapidPhase(phaseInitializing)
	<-s.sendRunningChan
	log.Debug("Received RUNNING")
}

func (s *Server) FastInvoke(w http.ResponseWriter, i *interop.Invoke, direct bool) error {
	invokeID, err := s.setReplyStream(w, direct)
	if err != nil {
		return err
	}

	s.setRapidPhase(phaseInvoking)

	i.ID = invokeID

	select {
	case s.invokeChanOut <- i:
		break
	case <-s.sendResponseChan:
		// we didn't pass invoke to rapid yet, but rapid already has written some response
		// It can happend if runtime/agent crashed even before we passed invoke to it
		return ErrInvokeResponseAlreadyWritten
	}

	select {
	case <-s.sendResponseChan:
		break
	case <-s.reservationContext.Done():
		return ErrInvokeReservationDone
	}

	return nil
}

func (s *Server) CurrentToken() *interop.Token {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.invokeCtx == nil {
		return nil
	}
	tok := s.invokeCtx.Token
	return &tok
}

// Invoke is used by the Runtime Interface Emulator (Rapid Local)
// https://github.com/aws/aws-lambda-runtime-interface-emulator
func (s *Server) Invoke(responseWriter http.ResponseWriter, invoke *interop.Invoke) error {
	resetCtx, resetCancel := context.WithCancel(context.Background())
	defer resetCancel()

	timeoutChan := make(chan error)
	go func() {
		select {
		case <-time.After(s.GetInvokeTimeout()):
			timeoutChan <- ErrInvokeTimeout
			s.Reset(autoresetReasonTimeout, resetDefaultTimeoutMs)
		case <-resetCtx.Done():
			log.Debugf("execute finished, autoreset cancelled")
		}
	}()

	reserveResp, err := s.Reserve(invoke.ID, "", "")
	if err != nil {
		switch err {
		case ErrInitError:
			// Simulate 'Suppressed Init' scenario
			s.Reset(autoresetReasonReserveFail, resetDefaultTimeoutMs)
			reserveResp, err = s.Reserve("", "", "")
			if err == ErrInitAlreadyDone {
				break
			}
			return err
		case ErrInitDoneFailed, ErrTerminated:
			s.Reset(autoresetReasonReserveFail, resetDefaultTimeoutMs)
			return err

		case ErrInitAlreadyDone:
			// This is a valid response (e.g. for 2nd and 3rd invokes)
			// TODO: switch on ReserveResponse status instead of err,
			// since these are valid values
			if s.InternalStateGetter == nil {
				responseWriter.Write([]byte("error: internal state callback not set"))
				return ErrInternalServerError
			}

		default:
			return err
		}
	}

	invoke.DeadlineNs = fmt.Sprintf("%d", metering.Monotime()+reserveResp.Token.FunctionTimeout.Nanoseconds())

	invokeChan := make(chan error)
	go func() {
		if err := s.FastInvoke(responseWriter, invoke, false); err != nil {
			invokeChan <- err
		}
	}()

	releaseChan := make(chan error)
	go func() {
		_, err := s.AwaitRelease()
		releaseChan <- err
	}()

	// TODO: verify the order of channel receives. When timeouts happen, Reset()
	// is called first, which also does Release() => this may signal a type
	// Err<*>ReservationDone error to the non-timeout channels. This is currently
	// handled by the http handler, which returns GatewayTimeout for reservation errors
	// too. However, Timeouts should ideally be only represented by ErrInvokeTimeout.
	select {
	case err = <-invokeChan:
	case err = <-timeoutChan:
	case err = <-releaseChan:
		if err != nil {
			s.Reset(autoresetReasonReleaseFail, resetDefaultTimeoutMs)
		}
	}
	return err
}

func (s *Server) AwaitRelease() (*statejson.InternalStateDescription, error) {
	select {
	case doneWithState := <-s.InvokeDoneChan:
		if len(doneWithState.ErrorType) > 0 {
			log.Errorf("Invoke DONE failed: %s", doneWithState.ErrorType)
			return nil, ErrInvokeDoneFailed
		}

		s.Release()
		return &doneWithState.State, nil

	case <-s.reservationContext.Done():
		return nil, ErrReleaseReservationDone
	}
}

func (s *Server) Shutdown(shutdown *interop.Shutdown) *statejson.InternalStateDescription {
	s.shutdownChanOut <- shutdown
	<-s.ShutdownDoneChan

	state := s.InternalStateGetter()
	return &state
}

func (s *Server) InternalState() (*statejson.InternalStateDescription, error) {
	if s.InternalStateGetter == nil {
		return nil, errors.New("InternalStateGetterNotSet")
	}

	state := s.InternalStateGetter()
	return &state, nil
}
