// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core/statejson"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils/invariant"
)

const (
	MaxPayloadSize = 6*1024*1024 + 100

	ResponseBandwidthRate      = 2 * 1024 * 1024
	ResponseBandwidthBurstSize = 6 * 1024 * 1024

	MinResponseBandwidthRate = 32 * 1024
	MaxResponseBandwidthRate = 64 * 1024 * 1024

	MinResponseBandwidthBurstSize = 32 * 1024
	MaxResponseBandwidthBurstSize = 64 * 1024 * 1024

	InitializationType = "lambda-managed-instances"
)

type ResponseMode string

const (
	ResponseModeBuffered  = "Buffered"
	ResponseModeStreaming = "Streaming"
)

type InvokeResponseMode string

const (
	InvokeResponseModeBuffered  InvokeResponseMode = ResponseModeBuffered
	InvokeResponseModeStreaming InvokeResponseMode = ResponseModeStreaming
)

var AllInvokeResponseModes = []string{
	string(InvokeResponseModeBuffered), string(InvokeResponseModeStreaming),
}

func ConvertInvokeResponseModeToString(invokeResponseMode InvokeResponseMode) string {
	if invokeResponseMode == "" {
		return ""
	}
	return "invoke_response_mode=" + strings.ToLower(string(invokeResponseMode))
}

type FunctionResponseMode string

const (
	FunctionResponseModeBuffered  FunctionResponseMode = ResponseModeBuffered
	FunctionResponseModeStreaming FunctionResponseMode = ResponseModeStreaming
)

var AllFunctionResponseModes = []string{
	string(FunctionResponseModeBuffered), string(FunctionResponseModeStreaming),
}

func ConvertToFunctionResponseMode(value string) (FunctionResponseMode, error) {

	if strings.EqualFold(value, string(FunctionResponseModeBuffered)) {
		return FunctionResponseModeBuffered, nil
	}

	if strings.EqualFold(value, string(FunctionResponseModeStreaming)) {
		return FunctionResponseModeStreaming, nil
	}

	allowedValues := strings.Join(AllFunctionResponseModes, ", ")
	slog.Error("Unable to map value to allowed values", "value", value, "allowedValues", allowedValues)
	return "", ErrInvalidFunctionResponseMode
}

type Message interface{}

type InvokeMetadataHeader string

type Invoke struct {
	TraceID               string
	LambdaSegmentID       string
	ID                    string
	InvokedFunctionArn    string
	CognitoIdentityID     string
	CognitoIdentityPoolID string
	Deadline              time.Time
	FunctionTimeout       time.Duration
	ClientContext         string
	ContentType           string
	Payload               io.Reader
	VersionID             string
	InvokeReceivedTime    time.Time
	InvokeResponseMetrics *InvokeResponseMetrics
	InvokeResponseMode    InvokeResponseMode
}

func (i Invoke) GetDeadlineMs(ctx context.Context) int64 {
	if deadline, hasDeadline := ctx.Deadline(); hasDeadline {

		return deadline.UnixMilli()
	}

	if !i.Deadline.IsZero() {

		return i.Deadline.UnixMilli()
	}

	return 0
}

type InvokeErrorTraceData struct {
	InvokeID InvokeID `json:"requestId,omitempty"`

	ErrorCause json.RawMessage `json:"ErrorCause,omitempty"`
}

func GetErrorResponseWithFormattedErrorMessage(errorType model.ErrorType, err error, invokeID InvokeID) *ErrorInvokeResponse {
	var errorMessage string
	if invokeID != "" {
		errorMessage = fmt.Sprintf("RequestId: %s Error: %v", invokeID, err)
	} else {
		errorMessage = fmt.Sprintf("Error: %v", err)
	}

	jsonPayload, err := json.Marshal(model.FunctionError{
		Type:    errorType,
		Message: errorMessage,
	})
	if err != nil {
		return &ErrorInvokeResponse{
			Headers: InvokeResponseHeaders{},
			FunctionError: model.FunctionError{
				Type:    model.ErrorSandboxFailure,
				Message: errorMessage,
			},
			Payload: []byte{},
		}
	}

	headers := InvokeResponseHeaders{}
	functionError := model.FunctionError{
		Type:    errorType,
		Message: errorMessage,
	}

	return &ErrorInvokeResponse{Headers: headers, FunctionError: functionError, Payload: jsonPayload}
}

type Shutdown struct {
	DeadlineNs int64
}

type TelemetrySubscriptionMetrics map[string]int

type InvokeResponseMetrics struct {
	StartReadingResponseTime time.Time

	FinishReadingResponseTime time.Time
	TimeShaped                time.Duration
	ProducedBytes             int64
	OutboundThroughputBps     int64
	FunctionResponseMode      FunctionResponseMode
	RuntimeCalledResponse     bool

	TransferError error

	Interrupted bool

	ResponsePayloadReadDuration time.Duration

	ResponsePayloadWriteDuration time.Duration
}

func IsResponseStreamingMetrics(metrics *InvokeResponseMetrics) bool {
	if metrics == nil {
		return false
	}
	return metrics.FunctionResponseMode == FunctionResponseModeStreaming
}

type DoneMetadataMetricsDimensions struct {
	InvokeResponseMode InvokeResponseMode
}

func (dimensions DoneMetadataMetricsDimensions) String() string {
	return ConvertInvokeResponseModeToString(dimensions.InvokeResponseMode)
}

type DoneMetadata struct {
	NumActiveExtensions int
	ExtensionNames      string
	RuntimeRelease      string

	InvokeRequestReadTimeNs      int64
	InvokeRequestSizeBytes       int64
	InvokeCompletionTimeNs       int64
	InvokeReceivedTime           int64
	RuntimeReadyTime             int64
	RuntimeResponseLatencyMs     float64
	RuntimeTimeThrottledMs       int64
	RuntimeProducedBytes         int64
	RuntimeOutboundThroughputBps int64
	MetricsDimensions            DoneMetadataMetricsDimensions
}

type Done struct {
	WaitForExit bool
	ErrorType   model.ErrorType
	Meta        DoneMetadata
}

type DoneFail struct {
	ErrorType model.ErrorType
	Meta      DoneMetadata
}

var ErrInvalidFunctionResponseMode = fmt.Errorf("ErrInvalidFunctionResponseMode")

var ErrInvalidInvokeResponseMode = fmt.Errorf("ErrInvalidInvokeResponseMode")

var ErrInvalidMaxPayloadSize = fmt.Errorf("ErrInvalidMaxPayloadSize")

var ErrInvalidResponseBandwidthRate = fmt.Errorf("ErrInvalidResponseBandwidthRate")

var ErrInvalidResponseBandwidthBurstSize = fmt.Errorf("ErrInvalidResponseBandwidthBurstSize")

var ErrMalformedCustomerHeaders = fmt.Errorf("ErrMalformedCustomerHeaders")

var ErrResponseSent = fmt.Errorf("ErrResponseSent")

var ErrReservationExpired = fmt.Errorf("ErrReservationExpired")

type ErrInternalPlatformError struct{}

func (s *ErrInternalPlatformError) Error() string {
	return "ErrInternalPlatformError"
}

type ErrTruncatedResponse struct{}

func (s *ErrTruncatedResponse) Error() string {
	return "ErrTruncatedResponse"
}

type ErrorResponseTooLarge struct {
	MaxResponseSize int
	ResponseSize    int
}

type ErrorResponseTooLargeDI struct {
	ErrorResponseTooLarge
}

func (s *ErrorResponseTooLarge) Error() string {
	return fmt.Sprintf("Response payload size (%d bytes) exceeded maximum allowed payload size (%d bytes).", s.ResponseSize, s.MaxResponseSize)
}

func (s *ErrorResponseTooLarge) AsErrorResponse() *ErrorInvokeResponse {
	functionError := model.FunctionError{
		Type:    model.ErrorFunctionOversizedResponse,
		Message: s.Error(),
	}
	jsonPayload, err := json.Marshal(functionError)
	invariant.Check(err == nil, "Failed to marshal interop.FunctionError")

	headers := InvokeResponseHeaders{ContentType: "application/json"}
	return &ErrorInvokeResponse{Headers: headers, FunctionError: functionError, Payload: jsonPayload}
}

type Server interface {
	SendInitErrorResponse(response *ErrorInvokeResponse) (*InvokeResponseMetrics, error)
}

type InternalStateGetter func() statejson.InternalStateDescription

var (
	ErrTimeout       = errors.New("errTimeout")
	ErrPlatformError = errors.New("ErrPlatformError")
)
