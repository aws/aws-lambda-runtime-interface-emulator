// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"encoding/json"
	"fmt"
	"io"

	"go.amzn.com/lambda/core/statejson"
)

// MaxPayloadSize max event body size declared as LAMBDA_EVENT_BODY_SIZE
const MaxPayloadSize = 6*1024*1024 + 100 // 6 MiB + 100 bytes

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
	Payload               []byte
	NeedDebugLogs         bool
	CorrelationID         string // internal use only
}

// Response is a response to an invoke that is sent to the slicer.
type Response struct {
	Payload []byte
}

// ErrorResponse is an error response that is sent to the slicer.
//
// Note, this struct is implementation-specific to how Slicer
// processes errors.
type ErrorResponse struct {
	// Payload sent via shared memory.
	Payload []byte

	// When error response body (Payload) is not provided, e.g.
	// not retrievable, error type and error message will be
	// used by the Slicer to construct a response json, e.g:
	//
	// default error response produced by the Slicer:
	// '{"errorMessage":"Unknown application error occurred"}',
	//
	// when error type is provided, error response becomes:
	// '{"errorMessage":"Unknown application error occurred","errorType":"ErrorType"}'
	ErrorType    string
	ErrorMessage string

	// Attached to invoke segment
	ErrorCause json.RawMessage
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

// Done message is sent to the slicer, part of the protocol.
type Done struct {
	WaitForExit         bool
	NumActiveExtensions int
	RuntimeRelease      string
	ErrorType           string // internal use only, still in use by standalone
	CorrelationID       string // internal use only
	// Metrics for response status of LogsAPI `/subscribe` calls
	LogsAPIMetrics LogsAPIMetrics
}

// DoneFail message is sent to the slicer to report error and request reset.
type DoneFail struct {
	RuntimeRelease      string
	NumActiveExtensions int
	ErrorType           string
	CorrelationID       string // internal use only
	// Metrics for response status of LogsAPI `/subscribe` calls
	LogsAPIMetrics LogsAPIMetrics
}

// ErrInvalidInvokeID is returned when provided invokeID doesn't match current invokeID
var ErrInvalidInvokeID = fmt.Errorf("ErrInvalidInvokeID")

// ErrResponseSent is returned when response with given invokeID was already sent.
var ErrResponseSent = fmt.Errorf("ErrResponseSent")

// Server implements Slicer communication protocol.
type Server interface {
	// SendErrorResponse sends response.
	// Errors returned:
	//   ErrInvalidInvokeID - validation error indicating that provided invokeID doesn't match current invokeID
	//   ErrResponseSent    - validation error indicating that response with given invokeID was already sent
	//   Non-nil error      - non-nil error indicating transport failure
	SendResponse(invokeID string, response *Response) error

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

	Invoke(responseWriter io.Writer, invoke *Invoke) error

	Shutdown(shutdown *Shutdown) *statejson.InternalStateDescription
}

type InternalStateGetter func() statejson.InternalStateDescription
