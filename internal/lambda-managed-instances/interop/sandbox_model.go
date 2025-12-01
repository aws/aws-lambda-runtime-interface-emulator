// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/netip"
	"time"

	intmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/servicelogs"
	supvmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/supervisor/model"
)

type InitSuccess struct {
	NumActiveExtensions   int
	ExtensionNames        string
	RuntimeRelease        string
	Ack                   chan struct{}
	NumInternalExtensions int
	NumExternalExtensions int
	RuntimeInitDuration   time.Duration
}

type InitFailure struct {
	Error         model.AppError
	ErrorType     model.ErrorType
	ErrorCategory model.ErrorCategory
	ErrorMessage  error

	NumActiveExtensions   int
	RuntimeRelease        string
	Ack                   chan struct{}
	NumInternalExtensions int
	NumExternalExtensions int
	RuntimeInitDuration   time.Duration
}

type InitResponse interface {
	initResponse()
}

func (s InitSuccess) initResponse() {}
func (f InitFailure) initResponse() {}

type PlatformError struct {
	model.PlatformError
	RuntimeRelease string
}

type ClientError struct {
	model.ClientError
}

func (PlatformError) initResponse() {}
func (ClientError) initResponse()   {}

var (
	_ InitResponse = PlatformError{}
	_ InitResponse = ClientError{}
)

type ErrorInvokeResponse struct {
	Headers       InvokeResponseHeaders
	Payload       []byte
	FunctionError model.FunctionError
}

type StreamableInvokeResponse struct {
	Headers  map[string]string
	Payload  io.Reader
	Trailers http.Header
	Request  *CancellableRequest
}

type InvokeResponseHeaders struct {
	ContentType          string
	FunctionResponseMode string
}

type InvokeResponseSender interface {
	SendResponse(invokeID InvokeID, response *StreamableInvokeResponse) (*InvokeResponseMetrics, error)

	SendErrorResponse(invokeID InvokeID, response *ErrorInvokeResponse) (*InvokeResponseMetrics, error)
}

type ResponseMetrics struct {
	RuntimeOutboundThroughputBps int64
	RuntimeProducedBytes         int64
	RuntimeResponseLatency       time.Duration
	RuntimeTimeThrottled         time.Duration
}

type RuntimeInitMetrics struct {
	Duration time.Duration
	Error    error
}

type InvokeSuccess struct {
	RuntimeRelease       string
	NumActiveExtensions  int
	ExtensionNames       string
	ExtensionsOverhead   time.Duration
	InvokeCompletionTime time.Duration
	InvokeReceivedTime   time.Time
	ResponseMetrics      ResponseMetrics
	InvokeResponseMode   InvokeResponseMode
}

type InvokeFailure struct {
	ErrorType            model.ErrorType
	ErrorMessage         error
	RuntimeRelease       string
	NumActiveExtensions  int
	ExtensionsOverhead   time.Duration
	InvokeReceivedTime   time.Time
	ResponseMetrics      ResponseMetrics
	ExtensionNames       string
	DefaultErrorResponse *ErrorInvokeResponse
	InvokeResponseMode   InvokeResponseMode
}

type InvokeResponse interface{ invokeResponse() }

func (InvokeSuccess) invokeResponse() {}
func (InvokeFailure) invokeResponse() {}
func (PlatformError) invokeResponse() {}

type ShutdownSuccess struct{}

type ShutdownResponse interface{ shutdownResponse() }

func (ShutdownSuccess) shutdownResponse() {}
func (PlatformError) shutdownResponse()   {}

type HealthyContainerResponse struct{}

type UnhealthyContainerResponse struct {
	ErrorType model.ErrorType
}

type HealthCheckResponse interface {
	healthCheckResponse()
}

func (HealthyContainerResponse) healthCheckResponse()   {}
func (UnhealthyContainerResponse) healthCheckResponse() {}

type InitExecutionData struct {
	ExtensionEnv                 intmodel.KVMap
	Runtime                      model.Runtime
	Credentials                  model.Credentials
	LogGroupName                 string
	LogStreamName                string
	FunctionMetadata             model.FunctionMetadata
	RuntimeManagedLoggingFormats []supvmodel.ManagedLoggingFormat
	StaticData                   EEStaticData
	TelemetrySubscriptionConfig  TelemetrySubscriptionConfig
}

func (i *InitExecutionData) FunctionARN() string {
	return i.StaticData.FunctionARN
}

func (i *InitExecutionData) FunctionVersion() string {
	return i.FunctionMetadata.FunctionVersion
}

func (i *InitExecutionData) MemorySizeMB() uint64 {
	return i.FunctionMetadata.MemorySizeBytes / (1024 * 1024)
}

func (i *InitExecutionData) FunctionVersionID() string {
	return i.StaticData.FunctionVersionID
}

func (i *InitExecutionData) FunctionTimeout() time.Duration {
	return i.StaticData.FunctionTimeout
}

func (i *InitExecutionData) InitTimeout() time.Duration {
	return i.StaticData.InitTimeout
}

func (i *InitExecutionData) LogGroup() string {
	return i.StaticData.LogGroupName
}

func (i *InitExecutionData) LogStream() string {
	return i.StaticData.LogStreamName
}

func (i *InitExecutionData) XRayTracingMode() intmodel.XrayTracingMode {
	return i.StaticData.XRayTracingMode
}

func (i *InitExecutionData) TelemetryPassphrase() string {
	return i.TelemetrySubscriptionConfig.Passphrase
}

func (i *InitExecutionData) TelemetryAPIAddr() netip.AddrPort {
	return i.TelemetrySubscriptionConfig.APIAddr
}

func (i *InitExecutionData) ArtefactType() intmodel.ArtefactType {
	return i.StaticData.ArtefactType
}

func (i *InitExecutionData) AmiId() string {
	return i.StaticData.AmiId
}

func (i *InitExecutionData) RuntimeVersion() string {
	return i.StaticData.RuntimeVersion
}

func (i *InitExecutionData) AvailabilityZoneId() string {
	return i.StaticData.AvailabilityZoneId
}

type EEStaticData struct {
	FunctionARN        string
	FunctionVersionID  string
	InitTimeout        time.Duration
	FunctionTimeout    time.Duration
	LogGroupName       string
	LogStreamName      string
	XRayTracingMode    intmodel.XrayTracingMode
	ArtefactType       intmodel.ArtefactType
	RuntimeVersion     string
	AmiId              string
	AvailabilityZoneId string
}

type TelemetrySubscriptionConfig struct {
	Passphrase string
	APIAddr    netip.AddrPort
}

type RapidContext interface {
	HandleInit(ctx context.Context, initData InitExecutionData, initMetrics InitMetrics) (err model.AppError)

	HandleShutdown(shutdownCause model.AppError, metrics ShutdownMetrics) model.AppError
	HandleInvoke(ctx context.Context, invokeRequest InvokeRequest, invokeMetrics InvokeMetrics) (err model.AppError, wasResponseSent bool)
	RuntimeAPIAddrPort() netip.AddrPort

	ProcessTerminationNotifier() <-chan model.AppError
}

type LifecyclePhase int

const (
	LifecyclePhaseInit LifecyclePhase = iota + 1
	LifecyclePhaseInvoke
)

type InvokeRequest interface {
	ContentType() string
	InvokeID() InvokeID
	Deadline() time.Time
	TraceId() string
	ClientContext() string
	CognitoId() string
	CognitoPoolId() string
	ResponseBandwidthRate() int64
	ResponseBandwidthBurstRate() int64
	MaxPayloadSize() int64
	ResponseMode() string

	BodyReader() io.Reader
	ResponseWriter() http.ResponseWriter

	SetResponseHeader(string, string)
	AddResponseHeader(string, string)
	WriteResponseHeaders(int)

	UpdateFromInitData(InitStaticDataProvider) model.AppError
	FunctionVersionID() string
}

type InitStaticDataProvider interface {
	FunctionARN() string
	FunctionVersion() string
	FunctionVersionID() string
	InitTimeout() time.Duration
	FunctionTimeout() time.Duration
	LogGroup() string
	LogStream() string
	XRayTracingMode() intmodel.XrayTracingMode
	MemorySizeMB() uint64
	ArtefactType() intmodel.ArtefactType
	AmiId() string
	RuntimeVersion() string
	AvailabilityZoneId() string
}

type InvokeMetrics interface {
	TriggerGetRequest()
	AttachInvokeRequest(InvokeRequest)
	AttachDependencies(InitStaticDataProvider, EventsAPI)
	UpdateConcurrencyMetrics(inflightInvokes, idleRuntimesCount int)
	TriggerStartRequest()
	TriggerSentRequest(bytes int64, requestPayloadReadDuration, requestPayloadWriteDuration time.Duration)
	TriggerGetResponse()
	TriggerSentResponse(runtimeResponseSent bool, responseErr model.AppError, streamingMetrics *InvokeResponseMetrics, errorPayloadSizeBytes int)

	TriggerInvokeDone() (totalMs time.Duration, runMs *time.Duration, initData InitStaticDataProvider)

	SendInvokeStartEvent(*TracingCtx) error
	SendInvokeFinishedEvent(tracingCtx *TracingCtx, xrayErrorCause json.RawMessage) error
	SendMetrics(model.AppError) error
}

type InitMetrics interface {
	TriggerGetRequest()
	SetLogsAPIMetrics(TelemetrySubscriptionMetrics)
	SetExtensionsNumber(internal, external int)
	TriggerStartRequest()
	TriggerStartingRuntime()
	TriggerRuntimeDone()
	TriggerInitCustomerPhaseDone()
	TriggerInitDone(model.AppError)

	RunDuration() time.Duration
	SendMetrics() error
}

type ShutdownMetrics interface {
	CreateDurationMetric(name string) DurationMetricTimer
	AddMetric(metric servicelogs.Metric)

	SetAgentCount(internal, external int)

	SendMetrics(error model.AppError)
}

type DurationMetricTimer interface {
	Done()
}
