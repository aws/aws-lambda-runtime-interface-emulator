// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.amzn.com/lambda/core/statejson"
	"go.amzn.com/lambda/fatalerror"
)

// MaxPayloadSize max event body size declared as LAMBDA_EVENT_BODY_SIZE
const MaxPayloadSize = 6*1024*1024 + 100 // 6 MiB + 100 bytes

const functionResponseSizeTooLargeType = "Function.ResponseSizeTooLarge"

// Message is a generic interop message.
type Message interface{}

// Invoke is an invocation request received from the slicer.
type Invoke struct {
	// Tracing header.
	// https://docs.aws.amazon.com/xray/latest/devguide/xray-concepts.html#xray-concepts-tracingheader
	TraceID               string
	LambdaSegmentID       string
	ID                    string
	InvokedFunctionArn    string
	CognitoIdentityID     string
	CognitoIdentityPoolID string
	DeadlineNs            string
	ClientContext         string
	ContentType           string
	Payload               io.Reader
	NeedDebugLogs         bool
	CorrelationID         string // internal use only
	ReservationToken      string
	VersionID             string
	InvokeReceivedTime    int64
}

type Token struct {
	ReservationToken string
	InvokeID         string
	VersionID        string
	FunctionTimeout  time.Duration
	InvackDeadlineNs int64
	TraceID          string
	LambdaSegmentID  string
	InvokeMetadata   string
	NeedDebugLogs    bool
}

type ErrorResponse struct {
	// Payload sent via shared memory.
	Payload []byte `json:"Payload,omitempty"`

	// When error response body (Payload) is not provided, e.g.
	// not retrievable, error type and error message will be
	// used by the Slicer to construct a response json, e.g:
	//
	// default error response produced by the Slicer:
	// '{"errorMessage":"Unknown application error occurred"}',
	//
	// when error type is provided, error response becomes:
	// '{"errorMessage":"Unknown application error occurred","errorType":"ErrorType"}'
	ErrorType    string `json:"errorType,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`

	// Attached to invoke segment
	ErrorCause json.RawMessage `json:"ErrorCause,omitempty"`
}

// SandboxType identifies sandbox type (PreWarmed vs Classic)
type SandboxType string

const SandboxPreWarmed SandboxType = "PreWarmed"
const SandboxClassic SandboxType = "Classic"

// Start message received from the slicer, part of the protocol.
type Start struct {
	InvokeID          string
	Handler           string
	AwsKey            string
	AwsSecret         string
	AwsSession        string
	SuppressInit      bool
	XRayDaemonAddress string // only in standalone; not used by slicer
	FunctionName      string // only in standalone; not used by slicer
	FunctionVersion   string // only in standalone; not used by slicer
	CorrelationID     string // internal use only
	// TODO: define new Init type that has the Start fields as well as env vars below.
	// In standalone mode, these env vars come from test/init but from environment otherwise.
	CustomerEnvironmentVariables map[string]string
	SandboxType                  SandboxType
}

// Running message is sent to the slicer, part of the protocol.
type Running struct {
	WaitStartTimeNs   int64
	WaitEndTimeNs     int64
	PreLoadTimeNs     int64
	PostLoadTimeNs    int64
	ExtensionsEnabled bool
}

// Reset message is sent to rapid to initiate reset sequence
type Reset struct {
	Reason        string
	DeadlineNs    int64
	CorrelationID string // internal use only
}

// Shutdown message is sent to rapid to initiate graceful shutdown
type Shutdown struct {
	DeadlineNs    int64
	CorrelationID string // internal use only
}

// Metrics for response status of LogsAPI `/subscribe` calls
type LogsAPIMetrics map[string]int

type DoneMetadata struct {
	NumActiveExtensions int
	ExtensionsResetMs   int64
	RuntimeRelease      string
	// Metrics for response status of LogsAPI `/subscribe` calls
	LogsAPIMetrics          LogsAPIMetrics
	InvokeRequestReadTimeNs int64
	InvokeRequestSizeBytes  int64
	InvokeCompletionTimeNs  int64
	InvokeReceivedTime      int64
}

type Done struct {
	WaitForExit   bool
	ErrorType     fatalerror.ErrorType
	CorrelationID string // internal use only
	Meta          DoneMetadata
}

type DoneFail struct {
	ErrorType     fatalerror.ErrorType
	CorrelationID string // internal use only
	Meta          DoneMetadata
}

// ErrInvalidInvokeID is returned when invokeID provided in Invoke2 does not match one provided in Token
var ErrInvalidInvokeID = fmt.Errorf("ErrInvalidInvokeID")

// ErrInvalidReservationToken is returned when reservationToken provided in Invoke2 does not match one provided in Token
var ErrInvalidReservationToken = fmt.Errorf("ErrInvalidReservationToken")

// ErrInvalidFunctionVersion is returned when functionVersion provided in Invoke2 does not match one provided in Token
var ErrInvalidFunctionVersion = fmt.Errorf("ErrInvalidFunctionVersion")

// ErrMalformedCustomerHeaders is returned when customer headers format is invalid
var ErrMalformedCustomerHeaders = fmt.Errorf("ErrMalformedCustomerHeaders")

// ErrResponseSent is returned when response with given invokeID was already sent.
var ErrResponseSent = fmt.Errorf("ErrResponseSent")

// ErrReservationExpired is returned when invoke arrived after InvackDeadline
var ErrReservationExpired = fmt.Errorf("ErrReservationExpired")

// ErrorResponseTooLarge is returned when response Payload exceeds shared memory buffer size
type ErrorResponseTooLarge struct {
	MaxResponseSize int
	ResponseSize    int
}

// ErrorResponseTooLarge is returned when response provided by Runtime does not fit into shared memory buffer
func (s *ErrorResponseTooLarge) Error() string {
	return fmt.Sprintf("Response payload size (%d bytes) exceeded maximum allowed payload size (%d bytes).", s.ResponseSize, s.MaxResponseSize)
}

// AsErrorResponse generates ErrorResponse from ErrorResponseTooLarge
func (s *ErrorResponseTooLarge) AsInteropError() *ErrorResponse {
	resp := ErrorResponse{
		ErrorType:    functionResponseSizeTooLargeType,
		ErrorMessage: s.Error(),
	}
	respJSON, err := json.Marshal(resp)
	if err != nil {
		panic("Failed to marshal interop.ErrorResponse")
	}
	resp.Payload = respJSON
	return &resp
}

// Server implements Slicer communication protocol.
type Server interface {
	// StartAcceptingDirectInvokes starts accepting on direct invoke socket (if one is available)
	StartAcceptingDirectInvokes() error

	// SendErrorResponse sends response.
	// Errors returned:
	//   ErrInvalidInvokeID - validation error indicating that provided invokeID doesn't match current invokeID
	//   ErrResponseSent    - validation error indicating that response with given invokeID was already sent
	//   Non-nil error      - non-nil error indicating transport failure
	SendResponse(invokeID string, response io.Reader) error

	// SendErrorResponse sends error response.
	// Errors returned:
	//   ErrInvalidInvokeID - validation error indicating that provided invokeID doesn't match current invokeID
	//   ErrResponseSent    - validation error indicating that response with given invokeID was already sent
	//   Non-nil error      - non-nil error indicating transport failure
	SendErrorResponse(invokeID string, response *ErrorResponse) error

	// GetCurrentInvokeID returns current invokeID.
	// NOTE, in case of INIT, when invokeID is not known in advance (e.g. provisioned concurrency),
	// returned invokeID will contain empty value.
	GetCurrentInvokeID() string

	// CommitMessage confirms that the message written through SendResponse and SendErrorResponse is complete.
	CommitResponse() error

	// SendRunning sends GIRD RUNNING.
	// Returns error on transport failure.
	SendRunning(*Running) error

	// SendRuntimeReady sends GIRD RTREADY
	SendRuntimeReady() error

	// SendDone sends GIRD DONE.
	// Returns error on transport failure.
	SendDone(*Done) error

	// SendDone sends GIRD DONEFAIL.
	// Returns error on transport failure.
	SendDoneFail(*DoneFail) error

	// StartChan returns Start emitter
	StartChan() <-chan *Start

	// InvokeChan returns Invoke emitter
	InvokeChan() <-chan *Invoke

	// ResetChan returns Reset emitter
	ResetChan() <-chan *Reset

	// ShutdownChan returns Shutdown emitter
	ShutdownChan() <-chan *Shutdown

	// TransportErrorChan emits errors if there was parsing/connection issue
	TransportErrorChan() <-chan error

	// Clear is called on rapid reset. It should leave server prepared for new invocations
	Clear()

	// IsResponseSent exposes is response sent flag
	IsResponseSent() bool

	// The following are used by standalone rapid only
	// TODO refactor to decouple the interfaces

	SetInternalStateGetter(cb InternalStateGetter)

	Init(i *Start, invokeTimeoutMs int64)

	Invoke(responseWriter http.ResponseWriter, invoke *Invoke) error

	Shutdown(shutdown *Shutdown) *statejson.InternalStateDescription
}

type InternalStateGetter func() statejson.InternalStateDescription
