// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

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

	resetDefaultTimeoutMs = 2000
)

type DoneWithState struct {
	*interop.Done
	State statejson.InternalStateDescription
}

func (s *DoneWithState) String() string {
	return fmt.Sprintf("%v %v", *s.Done, string(s.State.AsJSON()))
}

type InvokeContext struct {
	ID          string
	ReplySent   bool
	ReplyStream io.Writer
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

// Reserve allocates invoke context, returnes new invokeID
func (s *Server) Reserve(id string) (string, *statejson.InternalStateDescription, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.invokeCtx != nil {
		return "", nil, ErrAlreadyReserved
	}
	invokeID := uuid.New().String()
	if len(id) > 0 {
		invokeID = id
	}

	s.invokeCtx = &InvokeContext{
		ID: invokeID,
	}

	s.reservationContext, s.reservationCancel = context.WithCancel(context.Background())

	internalState, err := s.waitInit()
	return invokeID, internalState, err
}

func (s *Server) waitInit() (*statejson.InternalStateDescription, error) {
	for {
		select {

		case doneWithState, chanOpen := <-s.InitDoneChan:
			if !chanOpen {
				return nil, ErrInitAlreadyDone
			}

			// this was first call to reserve
			close(s.InitDoneChan)

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

func (s *Server) setReplyStream(w io.Writer) (string, error) {
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
	return s.invokeCtx.ID, nil
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

	return s.invokeCtx.ID
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

func (s *Server) sendResponseUnsafe(invokeID string, status int, payload []byte) error {
	if s.invokeCtx == nil || invokeID != s.invokeCtx.ID {
		return interop.ErrInvalidInvokeID
	}

	if s.invokeCtx.ReplySent {
		return interop.ErrResponseSent
	}

	if s.invokeCtx.ReplyStream == nil {
		return fmt.Errorf("ReplyStream not available")
	}

	// s.invokeCtx.ReplyStream.WriteHeader(status)
	if _, err := s.invokeCtx.ReplyStream.Write(payload); err != nil {
		return fmt.Errorf("Failed to write response to %s: %s", invokeID, err)
	}

	s.sendResponseChan <- struct{}{}

	s.invokeCtx.ReplySent = true
	return nil
}

func (s *Server) SendResponse(invokeID string, resp *interop.Response) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.sendResponseUnsafe(invokeID, 200, resp.Payload)
}

func (s *Server) CommitResponse() error { return nil }

func (s *Server) SendRunning(run *interop.Running) error {
	s.sendRunningChan <- run
	return nil
}

func (s *Server) SendErrorResponse(invokeID string, resp *interop.ErrorResponse) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.sendResponseUnsafe(invokeID, 500, resp.Payload)
}

func (s *Server) SendDone(done *interop.Done) error {
	s.doneChan <- done
	return nil
}

func (s *Server) SendDoneFail(doneFail *interop.DoneFail) error {
	s.doneChan <- &interop.Done{
		RuntimeRelease:      doneFail.RuntimeRelease,
		NumActiveExtensions: doneFail.NumActiveExtensions,
		ErrorType:           doneFail.ErrorType,
		CorrelationID:       doneFail.CorrelationID, // filipovi: correlationID is required to dispatch message into correct channel
	}
	return nil
}

func (s *Server) Reset(reason string, timeoutMs int64) error {
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
		return errors.New(done.ErrorType)
	}

	return nil
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

// Note, the dispatch loop below has potential to block, when
// channel is not drained. E.g. if test assumes sandbox init
// completion before dispatching reset, then reset will block
// until init channel is drained.
func (s *Server) dispatchDone() {
	for {
		done := <-s.doneChan
		log.Debug("Dispatching DONE:", done.CorrelationID)

		if done.CorrelationID == "initCorrelationID" {
			s.InitDoneChan <- DoneWithState{Done: done, State: s.InternalStateGetter()}
		} else if done.CorrelationID == "invokeCorrelationID" {
			s.InvokeDoneChan <- DoneWithState{Done: done, State: s.InternalStateGetter()}
		} else if done.CorrelationID == "resetCorrelationID" {
			s.ResetDoneChan <- done
		} else if done.CorrelationID == "shutdownCorrelationID" {
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

func (s *Server) SendRuntimeReady() error { return nil }

func deadlineNsFromTimeoutMs(timeoutMs int64) int64 {
	mono := metering.Monotime()
	return mono + timeoutMs*1000*1000
}

func (s *Server) Init(i *interop.Start, invokeTimeoutMs int64) {
	s.SetInvokeTimeout(time.Duration(invokeTimeoutMs) * time.Millisecond)

	s.startChanOut <- i
	<-s.sendRunningChan
	log.Debug("Received RUNNING")
}

func (s *Server) FastInvoke(w io.Writer, i *interop.Invoke) error {
	i.DeadlineNs = fmt.Sprintf("%v", time.Now().Add(s.invokeTimeout).UnixNano())
	invokeID, err := s.setReplyStream(w)
	if err != nil {
		return err
	}

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

func (s *Server) Invoke(responseWriter io.Writer, invoke *interop.Invoke) error {
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
	if _, _, err := s.Reserve(invoke.ID); err != nil {
		switch err {
		case ErrInitDoneFailed, ErrTerminated:
			s.Reset(autoresetReasonReserveFail, resetDefaultTimeoutMs)
			return err
		case ErrInitAlreadyDone:
			// init already happened, just return internal state
			// this was retained to prevent execute test regressions
			if s.InternalStateGetter == nil {
				responseWriter.Write([]byte("error: internal state callback not set"))
				return ErrInternalServerError
			}

		default:
			return err
		}
	}
	invoke.DeadlineNs = fmt.Sprintf("%v", time.Now().Add(s.invokeTimeout).UnixNano())

	invokeChan := make(chan error)
	go func() {
		if err := s.FastInvoke(responseWriter, invoke); err != nil {
			invokeChan <- err
		}
	}()

	releaseChan := make(chan error)
	go func() {
		_, err := s.AwaitRelease()
		releaseChan <- err
	}()

	var err error

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
