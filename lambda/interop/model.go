// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.amzn.com/lambda/core/statejson"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/supervisor/model"

	log "github.com/sirupsen/logrus"
)

// MaxPayloadSize max event body size declared as LAMBDA_EVENT_BODY_SIZE
const (
	MaxPayloadSize = 6*1024*1024 + 100 // 6 MiB + 100 bytes

	ResponseBandwidthRate      = 2 * 1024 * 1024 // default average rate of 2 MiB/s
	ResponseBandwidthBurstSize = 6 * 1024 * 1024 // default burst size of 6 MiB

	MinResponseBandwidthRate = 32 * 1024        // 32 KiB/s
	MaxResponseBandwidthRate = 64 * 1024 * 1024 // 64 MiB/s

	MinResponseBandwidthBurstSize = 32 * 1024        // 32 KiB
	MaxResponseBandwidthBurstSize = 64 * 1024 * 1024 // 64 MiB
)

const functionResponseSizeTooLargeType = "Function.ResponseSizeTooLarge"

// ResponseMode are top-level constants used in combination with the various types of
// modes we have for responses, such as invoke's response mode and function's response mode.
// In the future we might have invoke's request mode or similar, so these help set the ground
// for consistency.
type ResponseMode string

const ResponseModeBuffered = "Buffered"
const ResponseModeStreaming = "Streaming"

type InvokeResponseMode string

const InvokeResponseModeBuffered InvokeResponseMode = ResponseModeBuffered
const InvokeResponseModeStreaming InvokeResponseMode = ResponseModeStreaming

var AllInvokeResponseModes = []string{
	string(InvokeResponseModeBuffered), string(InvokeResponseModeStreaming),
}

// ConvertToInvokeResponseMode converts the given string to a InvokeResponseMode
// It is case insensitive and if there is no match, an error is thrown.
func ConvertToInvokeResponseMode(value string) (InvokeResponseMode, error) {
	// buffered
	if strings.EqualFold(value, string(InvokeResponseModeBuffered)) {
		return InvokeResponseModeBuffered, nil
	}

	// streaming
	if strings.EqualFold(value, string(InvokeResponseModeStreaming)) {
		return InvokeResponseModeStreaming, nil
	}

	// unknown
	allowedValues := strings.Join(AllInvokeResponseModes, ", ")
	log.Errorf("Unlable to map %s to %s.", value, allowedValues)
	return "", ErrInvalidInvokeResponseMode
}

// FunctionResponseMode is passed by Runtime to tell whether the response should be
// streamed or not.
type FunctionResponseMode string

const FunctionResponseModeBuffered FunctionResponseMode = ResponseModeBuffered
const FunctionResponseModeStreaming FunctionResponseMode = ResponseModeStreaming

var AllFunctionResponseModes = []string{
	string(FunctionResponseModeBuffered), string(FunctionResponseModeStreaming),
}

// ConvertToFunctionResponseMode converts the given string to a FunctionResponseMode
// It is case insensitive and if there is no match, an error is thrown.
func ConvertToFunctionResponseMode(value string) (FunctionResponseMode, error) {
	// buffered
	if strings.EqualFold(value, string(FunctionResponseModeBuffered)) {
		return FunctionResponseModeBuffered, nil
	}

	// streaming
	if strings.EqualFold(value, string(FunctionResponseModeStreaming)) {
		return FunctionResponseModeStreaming, nil
	}

	// unknown
	allowedValues := strings.Join(AllFunctionResponseModes, ", ")
	log.Errorf("Unlable to map %s to %s.", value, allowedValues)
	return "", ErrInvalidFunctionResponseMode
}

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
	ReservationToken      string
	VersionID             string
	InvokeReceivedTime    int64
	InvokeResponseMetrics *InvokeResponseMetrics
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
	Payload              []byte `json:"Payload,omitempty"`
	ContentType          string `json:"-"`
	FunctionResponseMode string `json:"-"`

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

// RuntimeInfo contains metadata about the runtime used by the Sandbox
type RuntimeInfo struct {
	ImageJSON string // image config, e.g {\"layers\":[]}
	Arn       string // runtime ARN, e.g. arn:awstest:lambda:us-west-2::runtime:python3.8::alpha
	Version   string // human-readable runtime arn equivalent, e.g. python3.8.v999
}

// Captures configuration of the operator and runtime domain
// that are only known after INIT is received
type DynamicDomainConfig struct {
	// extra hooks to execute at domain start. Currently used for filesystem and network hooks.
	// It can be empty.
	AdditionalStartHooks []model.Hook
	Mounts               []model.DriveMount
	//TODO: other dynamic configurations for the domain go here
}

// Reset message is sent to rapid to initiate reset sequence
type Reset struct {
	Reason                string
	DeadlineNs            int64
	InvokeResponseMetrics *InvokeResponseMetrics
	TraceID               string
	LambdaSegmentID       string
}

// Restore message is sent to rapid to restore runtime to make it ready for consecutive invokes
type Restore struct {
	AwsKey            string
	AwsSecret         string
	AwsSession        string
	CredentialsExpiry time.Time
}

type Resync struct {
}

// Shutdown message is sent to rapid to initiate graceful shutdown
type Shutdown struct {
	DeadlineNs int64
}

// Metrics for response status of LogsAPI/TelemetryAPI `/subscribe` calls
type TelemetrySubscriptionMetrics map[string]int

func MergeSubscriptionMetrics(logsAPIMetrics TelemetrySubscriptionMetrics, telemetryAPIMetrics TelemetrySubscriptionMetrics) TelemetrySubscriptionMetrics {
	metrics := make(map[string]int)
	for metric, value := range logsAPIMetrics {
		metrics[metric] = value
	}

	for metric, value := range telemetryAPIMetrics {
		metrics[metric] += value
	}
	return metrics
}

// InvokeResponseMetrics are produced while sending streaming invoke response to WP
type InvokeResponseMetrics struct {
	StartReadingResponseMonoTimeMs  int64
	FinishReadingResponseMonoTimeMs int64
	TimeShapedNs                    int64
	ProducedBytes                   int64
	OutboundThroughputBps           int64 // in bytes per second
	FunctionResponseMode            FunctionResponseMode
	RuntimeCalledResponse           bool
}

func IsResponseStreamingMetrics(metrics *InvokeResponseMetrics) bool {
	if metrics == nil {
		return false
	}
	return metrics.FunctionResponseMode == FunctionResponseModeStreaming
}

type DoneMetadata struct {
	NumActiveExtensions int
	ExtensionsResetMs   int64
	ExtensionNames      string
	RuntimeRelease      string
	// Metrics for response status of LogsAPI `/subscribe` calls
	LogsAPIMetrics               TelemetrySubscriptionMetrics
	InvokeRequestReadTimeNs      int64
	InvokeRequestSizeBytes       int64
	InvokeCompletionTimeNs       int64
	InvokeReceivedTime           int64
	RuntimeReadyTime             int64
	RuntimeTimeThrottledMs       int64
	RuntimeProducedBytes         int64
	RuntimeOutboundThroughputBps int64
}

type Done struct {
	WaitForExit bool
	ErrorType   fatalerror.ErrorType
	Meta        DoneMetadata
}

type DoneFail struct {
	ErrorType fatalerror.ErrorType
	Meta      DoneMetadata
}

// ErrInvalidInvokeID is returned when invokeID provided in Invoke2 does not match one provided in Token
var ErrInvalidInvokeID = fmt.Errorf("ErrInvalidInvokeID")

// ErrInvalidReservationToken is returned when reservationToken provided in Invoke2 does not match one provided in Token
var ErrInvalidReservationToken = fmt.Errorf("ErrInvalidReservationToken")

// ErrInvalidFunctionVersion is returned when functionVersion provided in Invoke2 does not match one provided in Token
var ErrInvalidFunctionVersion = fmt.Errorf("ErrInvalidFunctionVersion")

// ErrInvalidFunctionResponseMode is returned when the value sent by runtime during Invoke2
// is not a constant of type interop.FunctionResponseMode
var ErrInvalidFunctionResponseMode = fmt.Errorf("ErrInvalidFunctionResponseMode")

// ErrInvalidInvokeResponseMode is returned when optional InvokeResponseMode header provided in Invoke2 is not a constant of type interop.InvokeResponseMode
var ErrInvalidInvokeResponseMode = fmt.Errorf("ErrInvalidInvokeResponseMode")

// ErrInvalidMaxPayloadSize is returned when optional MaxPayloadSize header provided in Invoke2 is invalid
var ErrInvalidMaxPayloadSize = fmt.Errorf("ErrInvalidMaxPayloadSize")

// ErrInvalidResponseBandwidthRate is returned when optional ResponseBandwidthRate header provided in Invoke2 is invalid
var ErrInvalidResponseBandwidthRate = fmt.Errorf("ErrInvalidResponseBandwidthRate")

// ErrInvalidResponseBandwidthBurstSize is returned when optional ResponseBandwidthBurstSize header provided in Invoke2 is invalid
var ErrInvalidResponseBandwidthBurstSize = fmt.Errorf("ErrInvalidResponseBandwidthBurstSize")

// ErrMalformedCustomerHeaders is returned when customer headers format is invalid
var ErrMalformedCustomerHeaders = fmt.Errorf("ErrMalformedCustomerHeaders")

// ErrResponseSent is returned when response with given invokeID was already sent.
var ErrResponseSent = fmt.Errorf("ErrResponseSent")

// ErrReservationExpired is returned when invoke arrived after InvackDeadline
var ErrReservationExpired = fmt.Errorf("ErrReservationExpired")

// ErrInternalPlatformError is returned when internal platform error occurred
type ErrInternalPlatformError struct{}

func (s *ErrInternalPlatformError) Error() string {
	return "ErrInternalPlatformError"
}

// ErrTruncatedResponse is returned when response is truncated
type ErrTruncatedResponse struct{}

func (s *ErrTruncatedResponse) Error() string {
	return "ErrTruncatedResponse"
}

// ErrorResponseTooLarge is returned when response Payload exceeds shared memory buffer size
type ErrorResponseTooLarge struct {
	MaxResponseSize int
	ResponseSize    int
}

// ErrorResponseTooLargeDI is used to reproduce ErrorResponseTooLarge behavior for Direct Invoke mode
type ErrorResponseTooLargeDI struct {
	ErrorResponseTooLarge
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
	resp.ContentType = "application/json"
	return &resp
}

// Server used for sending messages and sharing data between the Runtime API handlers and the
// internal platform facing servers. For example,
//
//	responseCtx.SendResponse(...)
//
// will send the response payload and metadata provided by the runtime to the platform, through the internal
// protocol used by the specific implementation
// TODO: rename this to InvokeResponseContext, used to send responses from handlers to platform-facing server
type Server interface {
	// SendResponse sends response.
	// Errors returned:
	//   ErrInvalidInvokeID - validation error indicating that provided invokeID doesn't match current invokeID
	//   ErrResponseSent    - validation error indicating that response with given invokeID was already sent
	//   Non-nil error      - non-nil error indicating transport failure
	SendResponse(invokeID string, headers map[string]string, response io.Reader, trailers http.Header, request *CancellableRequest) error

	// SendErrorResponse sends error response.
	// Errors returned:
	//   ErrInvalidInvokeID - validation error indicating that provided invokeID doesn't match current invokeID
	//   ErrResponseSent    - validation error indicating that response with given invokeID was already sent
	//   Non-nil error      - non-nil error indicating transport failure
	SendErrorResponse(invokeID string, response *ErrorResponse) error
	SendInitErrorResponse(invokeID string, response *ErrorResponse) error

	// GetCurrentInvokeID returns current invokeID.
	// NOTE, in case of INIT, when invokeID is not known in advance (e.g. provisioned concurrency),
	// returned invokeID will contain empty value.
	GetCurrentInvokeID() string

	// SendRuntimeReady sends a message indicating the runtime has called /invocation/next.
	// The checkpoint allows us to compute the overhead due to Extensions by substracting it
	// from the time when all extensions have called /next.
	// TODO: this method is a lifecycle event used only for metrics, and doesn't belong here
	SendRuntimeReady() error
}

type InternalStateGetter func() statejson.InternalStateDescription

const OnDemandInitTelemetrySource string = "on-demand"
const ProvisionedConcurrencyInitTelemetrySource string = "provisioned-concurrency"
const InitCachingInitTelemetrySource string = "snap-start"

func InferTelemetryInitSource(initCachingEnabled bool, sandboxType SandboxType) string {
	initSource := OnDemandInitTelemetrySource

	// ToDo: Unify this selection of SandboxType by using the START message
	// after having a roadmap on the combination of INIT modes
	if initCachingEnabled {
		initSource = InitCachingInitTelemetrySource
	} else if sandboxType == SandboxPreWarmed {
		initSource = ProvisionedConcurrencyInitTelemetrySource
	}

	return initSource
}
