// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"

	"go.amzn.com/lambda/core/directinvoke"
	"go.amzn.com/lambda/core/statejson"
	"go.amzn.com/lambda/fatalerror"
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

type rapidPhase int

const (
	phaseIdle rapidPhase = iota
	phaseInitializing
	phaseInvoking
)

type runtimeState int

const (
	runtimeNotStarted = iota

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

	initChanOut             chan *interop.Init
	interruptedResponseChan chan *interop.Reset

	sendResponseChan chan *interop.InvokeResponseMetrics
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

	sandboxContext          interop.SandboxContext
	initContext             interop.InitContext
	invoker                 interop.InvokeContext
	initFailures            chan interop.InitFailure
	cachedInitErrorResponse *interop.ErrorInvokeResponse
}

// Validate interface compliance
var _ interop.Server = (*Server)(nil)

func (s *Server) setRapidPhase(phase rapidPhase) {
	s.rapidPhase = phase
}

func (s *Server) getRapidPhase() rapidPhase {
	return s.rapidPhase
}

func (s *Server) setRuntimeState(state runtimeState) {
	s.runtimeState = state
}

func (s *Server) getRuntimeState() runtimeState {
	return s.runtimeState
}

func (s *Server) SetInvokeTimeout(timeout time.Duration) {
	s.invokeTimeout = timeout
}

func (s *Server) GetInvokeTimeout() time.Duration {
	return s.invokeTimeout
}

func (s *Server) GetInvokeContext() *InvokeContext {
	ctx := *s.invokeCtx
	return &ctx
}

func (s *Server) setNewInvokeContext(invokeID string, traceID, lambdaSegmentID string) (*ReserveResponse, error) {
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

type ReserveResponse struct {
	Token         interop.Token
	InternalState *statejson.InternalStateDescription
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

	// The two errors reserve returns in standalone mode are INIT timeout
	// and INIT failure (two types of failure: runtime exit, /init/error). Both require suppressed
	// initialization, so we succeed the reservation.
	invCtx := s.initContext.Reserve()
	s.invoker = invCtx
	resp.InternalState, err = s.InternalState()

	return resp, err
}

func (s *Server) awaitInitCompletion() {
	initSuccess, initFailure := s.initContext.Wait()
	if initFailure != nil {
		// In standalone, we don't have to block rapid start() goroutine until init failure is consumed
		// because there is no channel back to the invoker until an invoke arrives via a Reserve()
		initFailure.Ack <- struct{}{}
		s.initFailures <- *initFailure
	} else {
		initSuccess.Ack <- struct{}{}
	}
	// always closing the channel makes this method idempotent
	close(s.initFailures)
}

func (s *Server) setReplyStream(w http.ResponseWriter, direct bool) (string, error) {
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
	if s.invokeCtx == nil {
		return ErrNotReserved
	}

	if s.reservationCancel != nil {
		s.reservationCancel()
	}

	s.sandboxContext.SetRuntimeStartedTime(-1)
	s.sandboxContext.SetInvokeResponseMetrics(nil)
	s.invokeCtx = nil
	return nil
}

// GetCurrentInvokeID
func (s *Server) GetCurrentInvokeID() string {
	if s.invokeCtx == nil {
		return ""
	}

	return s.invokeCtx.Token.InvokeID
}

// SetSandboxContext is used to set the sandbox context after intiialization of interop server.
// After refactoring all messages, this needs to be removed and made an struct parameter on initialization.
func (s *Server) SetSandboxContext(sbCtx interop.SandboxContext) {
	s.sandboxContext = sbCtx
}

// SetInternalStateGetter is used to set callback which returnes internal state for /test/internalState request
func (s *Server) SetInternalStateGetter(cb interop.InternalStateGetter) {
	s.InternalStateGetter = cb
}

func (s *Server) sendResponseUnsafe(invokeID string, additionalHeaders map[string]string, payload io.Reader, trailers http.Header, request *interop.CancellableRequest, runtimeCalledResponse bool) error {
	if s.invokeCtx == nil || invokeID != s.invokeCtx.Token.InvokeID {
		return interop.ErrInvalidInvokeID
	}

	if s.invokeCtx.ReplySent {
		return interop.ErrResponseSent
	}

	if s.invokeCtx.ReplyStream == nil {
		return fmt.Errorf("ReplyStream not available")
	}

	var reportedErr error
	if s.invokeCtx.Direct {
		if err := directinvoke.SendDirectInvokeResponse(additionalHeaders, payload, trailers, s.invokeCtx.ReplyStream, s.interruptedResponseChan, s.sendResponseChan, request, runtimeCalledResponse, invokeID); err != nil {
			// TODO: Do we need to drain the reader in case of a large payload and connection reuse?
			log.Errorf("Failed to write response to %s: %s", invokeID, err)
			reportedErr = err
		}
	} else {
		data, err := io.ReadAll(payload)
		if err != nil {
			return fmt.Errorf("Failed to read response on %s: %s", invokeID, err)
		}
		if len(data) > interop.MaxPayloadSize {
			return &interop.ErrorResponseTooLarge{
				ResponseSize:    len(data),
				MaxResponseSize: interop.MaxPayloadSize,
			}
		}

		startReadingResponseMonoTimeMs := metering.Monotime()
		s.invokeCtx.ReplyStream.Header().Add(directinvoke.ContentTypeHeader, additionalHeaders[directinvoke.ContentTypeHeader])
		written, err := s.invokeCtx.ReplyStream.Write(data)
		if err != nil {
			return fmt.Errorf("Failed to write response to %s: %s", invokeID, err)
		}

		s.sendResponseChan <- &interop.InvokeResponseMetrics{
			ProducedBytes:                   int64(written),
			StartReadingResponseMonoTimeMs:  startReadingResponseMonoTimeMs,
			FinishReadingResponseMonoTimeMs: metering.Monotime(),
			TimeShapedNs:                    int64(-1),
			OutboundThroughputBps:           int64(-1),
			// FIXME:
			// The runtime tells whether the function response mode is streaming or not.
			// Ideally, we would want to use that value here. Since I'm just rebasing, I will leave
			// as-is, but we should use that instead of relying on our memory to set this here
			// because we "know" it's a streaming code path.
			FunctionResponseMode:  interop.FunctionResponseModeBuffered,
			RuntimeCalledResponse: runtimeCalledResponse,
		}
	}

	s.invokeCtx.ReplySent = true
	s.invokeCtx.Direct = false
	return reportedErr
}

func (s *Server) SendResponse(invokeID string, resp *interop.StreamableInvokeResponse) error {
	s.setRuntimeState(runtimeInvokeResponseSent)
	runtimeCalledResponse := true
	return s.sendResponseUnsafe(invokeID, resp.Headers, resp.Payload, resp.Trailers, resp.Request, runtimeCalledResponse)
}

func (s *Server) SendInitErrorResponse(resp *interop.ErrorInvokeResponse) error {
	log.Debugf("Sending Init Error Response: %s", resp.FunctionError.Type)
	if s.getRapidPhase() == phaseInvoking {
		// This branch occurs during suppressed init
		return s.SendErrorResponse(s.GetCurrentInvokeID(), resp)
	}

	// Handle an /init/error outside of the invoke phase
	s.setCachedInitErrorResponse(resp)
	s.setRuntimeState(runtimeInitError)
	return nil
}

func (s *Server) SendErrorResponse(invokeID string, resp *interop.ErrorInvokeResponse) error {
	log.Debugf("Sending Error Response: %s", resp.FunctionError.Type)
	s.setRuntimeState(runtimeInvokeError)
	additionalHeaders := map[string]string{
		directinvoke.ContentTypeHeader: resp.Headers.ContentType,
		directinvoke.ErrorTypeHeader:   string(resp.FunctionError.Type),
	}
	if functionResponseMode := resp.Headers.FunctionResponseMode; functionResponseMode != "" {
		additionalHeaders[directinvoke.FunctionResponseModeHeader] = functionResponseMode
	}
	runtimeCalledResponse := false // we are sending an error here, so runtime called /error or crashed/timeout
	return s.sendResponseUnsafe(invokeID, additionalHeaders, bytes.NewReader(resp.Payload), nil, nil, runtimeCalledResponse)
}

func (s *Server) Reset(reason string, timeoutMs int64) (*statejson.ResetDescription, error) {
	// pass reset to rapid
	reset := &interop.Reset{
		Reason:     reason,
		DeadlineNs: deadlineNsFromTimeoutMs(timeoutMs),
	}
	go func() {
		select {
		case s.interruptedResponseChan <- reset:
			<-s.interruptedResponseChan // wait for response streaming metrics being added to reset struct
			s.sandboxContext.SetInvokeResponseMetrics(reset.InvokeResponseMetrics)
		default:
		}

		resetSuccess, resetFailure := s.sandboxContext.Reset(reset)
		s.Clear() // clear server state to prepare for new invokes
		s.setRapidPhase(phaseIdle)
		s.setRuntimeState(runtimeNotStarted)

		var meta interop.DoneMetadata
		if reset.InvokeResponseMetrics != nil && interop.IsResponseStreamingMetrics(reset.InvokeResponseMetrics) {
			meta.RuntimeTimeThrottledMs = reset.InvokeResponseMetrics.TimeShapedNs / int64(time.Millisecond)
			meta.RuntimeProducedBytes = reset.InvokeResponseMetrics.ProducedBytes
			meta.RuntimeOutboundThroughputBps = reset.InvokeResponseMetrics.OutboundThroughputBps
			meta.MetricsDimensions = interop.DoneMetadataMetricsDimensions{
				InvokeResponseMode: reset.InvokeResponseMode,
			}

			// These metrics aren't present in reset struct, therefore we need to get
			// them from s.sandboxContext.Reset()  response
			if resetFailure != nil {
				meta.RuntimeResponseLatencyMs = resetFailure.ResponseMetrics.RuntimeResponseLatencyMs
			} else {
				meta.RuntimeResponseLatencyMs = resetSuccess.ResponseMetrics.RuntimeResponseLatencyMs
			}
		}

		if resetFailure != nil {
			meta.ExtensionsResetMs = resetFailure.ExtensionsResetMs
			s.ResetDoneChan <- &interop.Done{ErrorType: resetFailure.ErrorType, Meta: meta}
		} else {
			meta.ExtensionsResetMs = resetSuccess.ExtensionsResetMs
			s.ResetDoneChan <- &interop.Done{ErrorType: resetSuccess.ErrorType, Meta: meta}
		}
	}()

	done := <-s.ResetDoneChan
	s.Release()

	if done.ErrorType != "" {
		return nil, errors.New(string(done.ErrorType))
	}

	return &statejson.ResetDescription{
		ExtensionsResetMs: done.Meta.ExtensionsResetMs,
		ResponseMetrics: statejson.ResponseMetrics{
			RuntimeResponseLatencyMs: done.Meta.RuntimeResponseLatencyMs,
			Dimensions: statejson.ResponseMetricsDimensions{
				InvokeResponseMode: statejson.InvokeResponseMode(
					done.Meta.MetricsDimensions.InvokeResponseMode,
				),
			},
		},
	}, nil
}

func NewServer() *Server {
	s := &Server{
		initChanOut:             make(chan *interop.Init),
		interruptedResponseChan: make(chan *interop.Reset),

		sendResponseChan: make(chan *interop.InvokeResponseMetrics),
		doneChan:         make(chan *interop.Done),

		// These two channels are buffered, because they are depleted asynchronously (by reserve and waitUntilRelease) and we don't want to block in SendDone until they are called
		InitDoneChan:   make(chan DoneWithState, 1),
		InvokeDoneChan: make(chan DoneWithState, 1),

		ResetDoneChan:    make(chan *interop.Done),
		ShutdownDoneChan: make(chan *interop.Done),
	}

	return s
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

func (s *Server) SendRuntimeReady() error {
	// only called when extensions are enabled
	s.setRuntimeState(runtimeReady)
	return nil
}

func deadlineNsFromTimeoutMs(timeoutMs int64) int64 {
	mono := metering.Monotime()
	return mono + timeoutMs*1000*1000
}

func (s *Server) setInitFailuresChan() {
	s.initFailures = make(chan interop.InitFailure)
}

func (s *Server) getInitFailuresChan() chan interop.InitFailure {
	return s.initFailures
}

func (s *Server) Init(i *interop.Init, invokeTimeoutMs int64) error {
	s.SetInvokeTimeout(time.Duration(invokeTimeoutMs) * time.Millisecond)
	s.setRapidPhase(phaseInitializing)
	s.setInitFailuresChan()
	initCtx := s.sandboxContext.Init(i, invokeTimeoutMs)

	s.initContext = initCtx
	go s.awaitInitCompletion()

	return nil
}

func (s *Server) FastInvoke(w http.ResponseWriter, i *interop.Invoke, direct bool) error {
	invokeID, err := s.setReplyStream(w, direct)
	if err != nil {
		return err
	}

	s.setRapidPhase(phaseInvoking)

	i.ID = invokeID

	select {
	case <-s.sendResponseChan:
		// we didn't pass invoke to rapid yet, but rapid already has written some response
		// It can happend if runtime/agent crashed even before we passed invoke to it
		return ErrInvokeResponseAlreadyWritten
	default:
	}

	go func() {
		if s.invoker == nil {
			// Reset occurred, do not send invoke request
			s.InvokeDoneChan <- DoneWithState{State: s.InternalStateGetter()}
			s.setRuntimeState(runtimeInvokeComplete)
			return
		}
		s.invoker.SendRequest(i, s)
		invokeSuccess, invokeFailure := s.invoker.Wait()
		if invokeFailure != nil {
			if invokeFailure.ResetReceived {
				return
			}

			// Rapid constructs a response body itself when invoke fails, with error type.
			// These are on the handleInvokeError path, may occur during timeout resets,
			// failure reset (proc exit). It is expected to be non-nil on all invoke failures.
			if invokeFailure.DefaultErrorResponse == nil {
				log.Panicf("default error response was nil for invoke failure, %v", invokeFailure)
			}

			if cachedInitError := s.getCachedInitErrorResponse(); cachedInitError != nil {
				// /init/error was called
				s.trySendDefaultErrorResponse(cachedInitError)
			} else {
				// sent only if /error and /response not called
				s.trySendDefaultErrorResponse(invokeFailure.DefaultErrorResponse)
			}
			doneFail := doneFailFromInvokeFailure(invokeFailure)
			s.InvokeDoneChan <- DoneWithState{
				Done:  &interop.Done{ErrorType: doneFail.ErrorType, Meta: doneFail.Meta},
				State: s.InternalStateGetter(),
			}
		} else {
			done := doneFromInvokeSuccess(invokeSuccess)
			s.InvokeDoneChan <- DoneWithState{Done: done, State: s.InternalStateGetter()}
		}
	}()

	select {
	case i.InvokeResponseMetrics = <-s.sendResponseChan:
		s.sandboxContext.SetInvokeResponseMetrics(i.InvokeResponseMetrics)
		break
	case <-s.reservationContext.Done():
		return ErrInvokeReservationDone
	}

	return nil
}

func (s *Server) setCachedInitErrorResponse(errResp *interop.ErrorInvokeResponse) {
	s.cachedInitErrorResponse = errResp
}

func (s *Server) getCachedInitErrorResponse() *interop.ErrorInvokeResponse {
	return s.cachedInitErrorResponse
}

func (s *Server) trySendDefaultErrorResponse(resp *interop.ErrorInvokeResponse) {
	if err := s.SendErrorResponse(s.GetCurrentInvokeID(), resp); err != nil {
		if err != interop.ErrResponseSent {
			log.Panicf("Failed to send default error response: %s", err)
		}
	}
}

func (s *Server) CurrentToken() *interop.Token {
	if s.invokeCtx == nil {
		return nil
	}
	tok := s.invokeCtx.Token
	return &tok
}

// Invoke is used by the Runtime Interface Emulator (Rapid Local)
// https://github.com/aws/aws-lambda-runtime-interface-emulator
func (s *Server) Invoke(responseWriter http.ResponseWriter, invoke *interop.Invoke) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	resetCtx, resetCancel := context.WithCancel(context.Background())
	defer resetCancel()

	timeoutChan := make(chan error)
	go func() {
		select {
		case <-time.After(s.GetInvokeTimeout()):
			log.Debug("Invoke() timeout")
			timeoutChan <- ErrInvokeTimeout
		case <-resetCtx.Done():
			log.Debugf("execute finished, autoreset cancelled")
		}
	}()

	initFailures := s.getInitFailuresChan()
	if initFailures == nil {
		return ErrInitNotStarted
	}

	releaseErrChan := make(chan error)
	releaseSuccessChan := make(chan struct{})
	go func() {
		// This thread can block in one of two method calls Reserve() & AwaitRelease(),
		// corresponding to Init and Invoke phase.
		// FastInvoke is intended to be 'async' response stream copying.
		// When a timeout occurs, we send a 'Reset' with the timeout reason
		// When a Reset is sent, the reset handler in rapid lib cancels existing flows,
		// including init/invoke. This causes either initFailure/invokeFailure, and then
		// the Reset is handled and processed.
		// TODO: however, ideally Reserve() does not block on init, but FastInvoke does
		// The logic would be almost identical, except that init failures could manifest
		// through return values of FastInvoke and not Reserve()

		reserveResp, err := s.Reserve("", "", "")
		if err != nil {
			log.Infof("ReserveFailed: %s", err)
		}

		invoke.DeadlineNs = fmt.Sprintf("%d", metering.Monotime()+reserveResp.Token.FunctionTimeout.Nanoseconds())
		go func() {
			if initCompletionResp, err := s.awaitInitialized(); err != nil {
				switch err {
				case ErrInitResetReceived, ErrInitDoneFailed:
					// For init failures, cache the response so they can be checked later
					// We check if they have not already been set by a call to /init/error by runtime
					if s.getCachedInitErrorResponse() == nil {
						errType, errMsg := initCompletionResp.InitErrorType, initCompletionResp.InitErrorMessage.Error()
						headers := interop.InvokeResponseHeaders{}
						fnError := interop.FunctionError{Type: errType, Message: errMsg}
						s.setCachedInitErrorResponse(&interop.ErrorInvokeResponse{Headers: headers, FunctionError: fnError, Payload: []byte{}})
					}

					// Init failed, so we explicitly shutdown runtime (cleanup unused extensions).
					// Because following fast invoke will start new (supressed) Init phase without reset call
					s.Shutdown(&interop.Shutdown{DeadlineNs: metering.Monotime() + int64(resetDefaultTimeoutMs*1000*1000)})
				}
			}

			if err := s.FastInvoke(responseWriter, invoke, false); err != nil {
				log.Debugf("FastInvoke() error: %s", err)
			}
		}()

		_, err = s.AwaitRelease()
		if err != nil && err != ErrReleaseReservationDone {
			log.Debugf("AwaitRelease() error: %s", err)
			switch err {
			case ErrReleaseReservationDone: // not an error, expected return value when Reset is called
				if s.getCachedInitErrorResponse() != nil {
					// For Init failures, AwaitRelease returns ErrReleaseReservationDone
					// because the Reset calls Release & cancels the release context
					// We rename the error to ErrInitDoneFailed
					releaseErrChan <- ErrInitDoneFailed
				}
			case ErrInitDoneFailed, ErrInvokeDoneFailed:
				// Reset when either init or invoke failrues occur, i.e.
				// init/error, invocation/error, Runtime.ExitError, Extension.ExitError
				s.Reset(autoresetReasonReleaseFail, resetDefaultTimeoutMs)
				releaseErrChan <- err
			default:
				releaseErrChan <- err
			}
			return
		}

		releaseSuccessChan <- struct{}{}
	}()

	var err error
	select {
	case timeoutErr := <-timeoutChan:
		s.Reset(autoresetReasonTimeout, resetDefaultTimeoutMs)
		select {
		case releaseErr := <-releaseErrChan: // when AwaitRelease() has errors
			log.Debugf("Invoke() release error on Execute() timeout: %s", releaseErr)
		case <-releaseSuccessChan: // when AwaitRelease() finishes cleanly
		}
		err = timeoutErr
	case err = <-releaseErrChan:
		log.Debug("Invoke() release error")
	case <-releaseSuccessChan:
		s.Release()
		log.Debug("Invoke() success")
	}

	return err
}

type initCompletionResponse struct {
	InitErrorType    fatalerror.ErrorType
	InitErrorMessage error
}

func (s *Server) awaitInitialized() (initCompletionResponse, error) {
	initFailure, awaitingInitStatus := <-s.getInitFailuresChan()
	resp := initCompletionResponse{}

	if initFailure.ResetReceived {
		// Resets during Init are only received in standalone
		// during an invoke timeout
		s.setRuntimeState(runtimeInitFailed)
		resp.InitErrorType = initFailure.ErrorType
		resp.InitErrorMessage = initFailure.ErrorMessage
		return resp, ErrInitResetReceived
	}

	if awaitingInitStatus {
		// channel not closed, received init failure
		// Sandbox can be reserved even if init failed (due to function errors)
		s.setRuntimeState(runtimeInitFailed)
		resp.InitErrorType = initFailure.ErrorType
		resp.InitErrorMessage = initFailure.ErrorMessage
		return resp, ErrInitDoneFailed
	}

	// not awaiting init status (channel closed)
	return resp, nil
}

// AwaitInitialized waits until init is complete. It must be idempotent,
// since it can be called twice when a caller wants to wait until init is complete
func (s *Server) AwaitInitialized() error {
	if _, err := s.awaitInitialized(); err != nil {
		if releaseErr := s.Release(); releaseErr != nil {
			log.Infof("Error releasing after init failure %s: %s", err, releaseErr)
		}
		s.setRuntimeState(runtimeInitFailed)
		return err
	}
	s.setRuntimeState(runtimeInitComplete)
	return nil
}

func (s *Server) AwaitRelease() (*statejson.ReleaseResponse, error) {
	defer func() {
		s.setRapidPhase(phaseIdle)
		s.setRuntimeState(runtimeInvokeComplete)
	}()

	select {
	case doneWithState := <-s.InvokeDoneChan:
		if len(doneWithState.ErrorType) > 0 && string(doneWithState.ErrorType) == ErrInitDoneFailed.Error() {
			return nil, ErrInitDoneFailed
		}

		if len(doneWithState.ErrorType) > 0 {
			log.Errorf("Invoke DONE failed: %s", doneWithState.ErrorType)
			return nil, ErrInvokeDoneFailed
		}

		releaseResponse := statejson.ReleaseResponse{
			InternalStateDescription: &doneWithState.State,
			ResponseMetrics: statejson.ResponseMetrics{
				RuntimeResponseLatencyMs: doneWithState.Meta.RuntimeResponseLatencyMs,
				Dimensions: statejson.ResponseMetricsDimensions{
					InvokeResponseMode: statejson.InvokeResponseMode(
						doneWithState.Meta.MetricsDimensions.InvokeResponseMode,
					),
				},
			},
		}

		s.Release()
		return &releaseResponse, nil

	case <-s.reservationContext.Done():
		return nil, ErrReleaseReservationDone
	}
}

func (s *Server) Shutdown(shutdown *interop.Shutdown) *statejson.InternalStateDescription {
	shutdownSuccess := s.sandboxContext.Shutdown(shutdown)
	if len(shutdownSuccess.ErrorType) > 0 {
		log.Errorf("Shutdown first fatal error: %s", shutdownSuccess.ErrorType)
	}

	s.setRapidPhase(phaseIdle)
	s.setRuntimeState(runtimeNotStarted)

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

func (s *Server) Restore(restore *interop.Restore) (interop.RestoreResult, error) {
	return s.sandboxContext.Restore(restore)
}

func doneFromInvokeSuccess(successMsg interop.InvokeSuccess) *interop.Done {
	return &interop.Done{
		Meta: interop.DoneMetadata{
			RuntimeRelease:          successMsg.RuntimeRelease,
			NumActiveExtensions:     successMsg.NumActiveExtensions,
			ExtensionNames:          successMsg.ExtensionNames,
			InvokeRequestReadTimeNs: successMsg.InvokeMetrics.InvokeRequestReadTimeNs,
			InvokeRequestSizeBytes:  successMsg.InvokeMetrics.InvokeRequestSizeBytes,
			RuntimeReadyTime:        successMsg.InvokeMetrics.RuntimeReadyTime,

			InvokeCompletionTimeNs:       successMsg.InvokeCompletionTimeNs,
			InvokeReceivedTime:           successMsg.InvokeReceivedTime,
			RuntimeResponseLatencyMs:     successMsg.ResponseMetrics.RuntimeResponseLatencyMs,
			RuntimeTimeThrottledMs:       successMsg.ResponseMetrics.RuntimeTimeThrottledMs,
			RuntimeProducedBytes:         successMsg.ResponseMetrics.RuntimeProducedBytes,
			RuntimeOutboundThroughputBps: successMsg.ResponseMetrics.RuntimeOutboundThroughputBps,
			LogsAPIMetrics:               successMsg.LogsAPIMetrics,
			MetricsDimensions: interop.DoneMetadataMetricsDimensions{
				InvokeResponseMode: successMsg.InvokeResponseMode,
			},
		},
	}
}

func doneFailFromInvokeFailure(failureMsg *interop.InvokeFailure) *interop.DoneFail {
	return &interop.DoneFail{
		ErrorType: failureMsg.ErrorType,
		Meta: interop.DoneMetadata{
			RuntimeRelease:      failureMsg.RuntimeRelease,
			NumActiveExtensions: failureMsg.NumActiveExtensions,
			InvokeReceivedTime:  failureMsg.InvokeReceivedTime,

			RuntimeResponseLatencyMs:     failureMsg.ResponseMetrics.RuntimeResponseLatencyMs,
			RuntimeTimeThrottledMs:       failureMsg.ResponseMetrics.RuntimeTimeThrottledMs,
			RuntimeProducedBytes:         failureMsg.ResponseMetrics.RuntimeProducedBytes,
			RuntimeOutboundThroughputBps: failureMsg.ResponseMetrics.RuntimeOutboundThroughputBps,

			InvokeRequestReadTimeNs: failureMsg.InvokeMetrics.InvokeRequestReadTimeNs,
			InvokeRequestSizeBytes:  failureMsg.InvokeMetrics.InvokeRequestSizeBytes,
			RuntimeReadyTime:        failureMsg.InvokeMetrics.RuntimeReadyTime,

			ExtensionNames: failureMsg.ExtensionNames,
			LogsAPIMetrics: failureMsg.LogsAPIMetrics,

			MetricsDimensions: interop.DoneMetadataMetricsDimensions{
				InvokeResponseMode: failureMsg.InvokeResponseMode,
			},
		},
	}
}
