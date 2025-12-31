// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/ptr"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/servicelogs"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils/invariant"
)

const (
	ResponseLatencySpanName  = "responseLatency"
	ResponseDurationSpanName = "responseDuration"
)

const (
	InvokeTimeoutProperty = "InvokeTimeoutSeconds"

	RequestResponseModeDimension = "RequestMode"
	ResponseModeDimension        = "ResponseMode"

	RequestSendDurationMetric = "RequestSendDuration"

	RequestPayloadReadDurationMetric = "RequestPayloadReadDuration"

	RequestPayloadWriteDurationMetric = "RequestPayloadWriteDuration"
	ResponseLatencyMetric             = "ResponseLatency"
	ResponseDurationMetric            = "ResponseDuration"
	FunctionDurationMetric            = "FunctionDuration"
	RequestPayloadSizeBytesMetric     = "RequestPayloadSizeBytes"
	ResponsePayloadSizeBytesMetric    = "ResponsePayloadSizeBytes"

	ResponsePayloadReadDurationMetric = "ResponsePayloadReadDuration"

	ResponsePayloadWriteDurationMetric = "ResponsePayloadWriteDuration"
	ErrorPayloadSizeBytesMetric        = "ErrorPayloadSizeBytes"
	ResponseThrottledDurationMetric    = "ResponseThrottledDuration"
	ResponseThroughputMetric           = "ResponseThroughput"
	InflightRequestCountMetric         = "InflightRequestCount"
	IdleRuntimesCountMetric            = "IdleRuntimesCount"
)

var invokeMetricsMissDepError = "Invoke metrics miss dependencies"

type Counter interface {
	AddInvoke(proxiedBytes uint64)
}

type invokeMetrics struct {
	telemetryEventsAPI interop.EventsAPI
	logger             servicelogs.Logger

	initData  interop.InitStaticDataProvider
	invokeReq interop.InvokeRequest

	runtimeResponseSent bool
	error               model.AppError

	timeGetRequest   time.Time
	timeStartRequest time.Time
	timeSentRequest  time.Time
	timeGetResponse  time.Time
	timeSentResponse time.Time
	timeInvokeDone   time.Time

	responseMetrics *interop.InvokeResponseMetrics

	requestPayloadBytes         int64
	requestPayloadReadDuration  time.Duration
	requestPayloadWriteDuration time.Duration
	errorPayloadSizeBytes       int

	inflightInvokes   int
	idleRuntimesCount int

	counter Counter

	getCurrentTime func() time.Time
}

func NewInvokeMetrics(logger servicelogs.Logger, counter Counter) *invokeMetrics {
	return &invokeMetrics{
		getCurrentTime: time.Now,
		logger:         logger,
		counter:        counter,
	}
}

func (e *invokeMetrics) AttachInvokeRequest(req interop.InvokeRequest) {
	e.invokeReq = req
}

func (e *invokeMetrics) AttachDependencies(initData interop.InitStaticDataProvider, telemetryEventsAPI interop.EventsAPI) {
	invariant.Check(e.invokeReq != nil, invokeMetricsMissDepError)
	e.initData = initData
	e.telemetryEventsAPI = telemetryEventsAPI
}

func (e *invokeMetrics) TriggerGetRequest() {
	e.timeGetRequest = e.getCurrentTime()
}

func (e *invokeMetrics) UpdateConcurrencyMetrics(inflightInvokes, idleRuntimesCount int) {
	e.inflightInvokes = inflightInvokes
	e.idleRuntimesCount = idleRuntimesCount
}

func (e *invokeMetrics) TriggerStartRequest() {
	e.timeStartRequest = e.getCurrentTime()
}

func (e *invokeMetrics) TriggerSentRequest(requestBytes int64, requestPayloadReadDuration, requestPayloadWriteDuration time.Duration) {
	e.timeSentRequest = e.getCurrentTime()
	e.requestPayloadBytes = requestBytes
	e.requestPayloadReadDuration = requestPayloadReadDuration
	e.requestPayloadWriteDuration = requestPayloadWriteDuration
}

func (e *invokeMetrics) TriggerGetResponse() {
	e.timeGetResponse = e.getCurrentTime()
}

func (e *invokeMetrics) TriggerSentResponse(runtimeResponseSent bool, responseErr model.AppError, streamingMetrics *interop.InvokeResponseMetrics, errorPayloadSizeBytes int) {
	e.timeSentResponse = e.getCurrentTime()
	e.runtimeResponseSent = runtimeResponseSent
	e.error = responseErr
	e.responseMetrics = streamingMetrics
	e.errorPayloadSizeBytes = errorPayloadSizeBytes
}

func (e *invokeMetrics) TriggerInvokeDone() (totalMs time.Duration, runMs *time.Duration, initData interop.InitStaticDataProvider) {
	e.timeInvokeDone = e.getCurrentTime()

	totalMs = e.timeInvokeDone.Sub(e.timeGetRequest)

	if !e.timeStartRequest.IsZero() {
		runMs = ptr.To(e.timeInvokeDone.Sub(e.timeStartRequest))
	}
	return totalMs, runMs, e.initData
}

func (e *invokeMetrics) SendInvokeStartEvent(tracing *interop.TracingCtx) error {
	invariant.Check(e.telemetryEventsAPI != nil, invokeMetricsMissDepError)

	return e.telemetryEventsAPI.SendInvokeStart(interop.InvokeStartData{
		InvokeID:    e.invokeReq.InvokeID(),
		Version:     e.initData.FunctionVersion(),
		FunctionARN: e.initData.FunctionARN(),
		Tracing:     tracing,
	})
}

func (e *invokeMetrics) SendInvokeFinishedEvent(tracing *interop.TracingCtx, xrayErrorCause json.RawMessage) error {
	invariant.Check(e.telemetryEventsAPI != nil, invokeMetricsMissDepError)

	if xrayErrorCause != nil {

		err := e.telemetryEventsAPI.SendInternalXRayErrorCause(interop.InternalXRayErrorCauseData{InvokeID: e.invokeReq.InvokeID(), Cause: string(xrayErrorCause)})
		if err != nil {
			slog.Error("Failed to send xray error cause", "err", err, "invokeId", e.invokeReq.InvokeID())
		}
	}

	spans := []interop.Span{}
	spans = e.addLatencySpan(spans)
	spans = e.addDurationSpan(spans)

	return e.telemetryEventsAPI.SendReport(interop.ReportData{
		InvokeID: e.invokeReq.InvokeID(),
		Status:   interop.BuildStatusFromError(e.error),
		Metrics: interop.ReportMetrics{
			DurationMs: interop.ReportDurationMs(buildDuration(e.timeStartRequest, e.timeSentResponse)),
		},
		Tracing:   tracing,
		Spans:     spans,
		ErrorType: buildErrorTypePointer(e.error),
	})
}

func (e *invokeMetrics) addLatencySpan(spans []interop.Span) []interop.Span {
	if e.timeSentRequest.IsZero() || e.timeGetResponse.IsZero() {
		return spans
	}

	latencySpan := interop.Span{
		Name:       ResponseLatencySpanName,
		Start:      e.timeSentRequest.UTC().Format(telemetry.TimeFormat),
		DurationMs: buildDuration(e.timeSentRequest, e.timeGetResponse),
	}

	return append(spans, latencySpan)
}

func (e *invokeMetrics) addDurationSpan(spans []interop.Span) []interop.Span {
	if !e.runtimeResponseSent || e.timeGetResponse.IsZero() || e.timeSentResponse.IsZero() {
		return spans
	}

	durationSpan := interop.Span{
		Name:       ResponseDurationSpanName,
		Start:      e.timeGetResponse.UTC().Format(telemetry.TimeFormat),
		DurationMs: buildDuration(e.timeGetResponse, e.timeSentResponse),
	}

	return append(spans, durationSpan)
}

func buildDuration(start time.Time, finish time.Time) float64 {
	return float64(finish.Sub(start).Microseconds()) / 1000.0
}

func buildErrorTypePointer(err model.AppError) *model.ErrorType {
	if err == nil {
		return nil
	}

	errorType := err.ErrorType()
	return &errorType
}

func (e *invokeMetrics) SendMetrics(invokeErr model.AppError) error {
	invariant.Check(e.logger != nil, invokeMetricsMissDepError)

	if e.error != nil && invokeErr == nil {
		invariant.Violate("Empty error in SendMetrics after previous error isn't nil")
	}

	e.error = invokeErr

	props := e.buildProperties()
	dims := e.buildDimensions()
	metrics := e.buildMetrics()

	e.logger.Log(servicelogs.InvokeOp, e.timeGetRequest, props, dims, metrics)

	e.updateCounter()

	return nil
}

func (e *invokeMetrics) updateCounter() {
	var proxiedBytes uint64
	proxiedBytes += uint64(e.requestPayloadBytes)
	proxiedBytes += uint64(e.errorPayloadSizeBytes)
	if e.responseMetrics != nil {
		proxiedBytes += uint64(e.responseMetrics.ProducedBytes)
	}

	e.counter.AddInvoke(proxiedBytes)
}

func (e *invokeMetrics) buildProperties() []servicelogs.Property {
	var props []servicelogs.Property

	if e.invokeReq != nil {
		props = append(props,
			servicelogs.Property{
				Name:  interop.RequestIdProperty,
				Value: e.invokeReq.InvokeID(),
			},
		)
	}

	return props
}

func (e *invokeMetrics) buildDimensions() []servicelogs.Dimension {
	var dim []servicelogs.Dimension

	if e.invokeReq != nil {
		dim = append(dim,
			servicelogs.Dimension{
				Name:  RequestResponseModeDimension,
				Value: e.invokeReq.ResponseMode(),
			},
		)
	}

	if e.responseMetrics != nil {
		dim = append(dim, servicelogs.Dimension{
			Name:  ResponseModeDimension,
			Value: string(e.responseMetrics.FunctionResponseMode),
		})
	}

	return dim
}

func (e *invokeMetrics) buildMetrics() []servicelogs.Metric {
	totalDuration := e.timeInvokeDone.Sub(e.timeGetRequest)
	runDuration := time.Duration(0)
	if !e.timeSentResponse.IsZero() {
		runDuration = e.timeSentResponse.Sub(e.timeStartRequest)
	}
	platformOverhead := totalDuration - runDuration

	metrics := []servicelogs.Metric{
		servicelogs.Timer(interop.TotalDurationMetric, totalDuration),
		servicelogs.Timer(interop.PlatformOverheadDurationMetric, platformOverhead),
		servicelogs.Counter(InflightRequestCountMetric, float64(e.inflightInvokes)),
		servicelogs.Counter(IdleRuntimesCountMetric, float64(e.idleRuntimesCount)),
	}

	if e.responseMetrics != nil {
		metrics = append(metrics,
			servicelogs.Counter(ResponsePayloadSizeBytesMetric, float64(e.responseMetrics.ProducedBytes)),
			servicelogs.Timer(ResponseThrottledDurationMetric, e.responseMetrics.TimeShaped),
			servicelogs.Counter(ResponseThroughputMetric, float64(e.responseMetrics.OutboundThroughputBps)),
			servicelogs.Timer(ResponsePayloadReadDurationMetric, e.responseMetrics.ResponsePayloadReadDuration),
			servicelogs.Timer(ResponsePayloadWriteDurationMetric, e.responseMetrics.ResponsePayloadWriteDuration),
		)
	}

	if e.errorPayloadSizeBytes != 0 {
		metrics = append(metrics,
			servicelogs.Counter(ErrorPayloadSizeBytesMetric, float64(e.errorPayloadSizeBytes)),
		)
	}

	if runDuration > 0 {
		metrics = append(metrics, servicelogs.Timer(FunctionDurationMetric, runDuration))
	}

	if !e.timeSentRequest.IsZero() {
		metrics = append(metrics,
			servicelogs.Timer(RequestSendDurationMetric, e.timeSentRequest.Sub(e.timeStartRequest)),
			servicelogs.Counter(RequestPayloadSizeBytesMetric, float64(e.requestPayloadBytes)),
			servicelogs.Timer(RequestPayloadReadDurationMetric, e.requestPayloadReadDuration),
			servicelogs.Timer(RequestPayloadWriteDurationMetric, e.requestPayloadWriteDuration),
		)
	}

	if !e.timeGetResponse.IsZero() {
		metrics = append(metrics, servicelogs.Timer(ResponseLatencyMetric, e.timeGetResponse.Sub(e.timeSentRequest)))
	}

	if e.runtimeResponseSent {
		metrics = append(metrics, servicelogs.Timer(ResponseDurationMetric, e.timeSentResponse.Sub(e.timeGetResponse)))
	}

	var clientErrCnt, customerErrCnt, platformErrCnt, nonCustomerErrCnt float64

	switch e.error.(type) {
	case model.ClientError:
		clientErrCnt = 1
		if e.error.ErrorType() != model.ErrorRuntimeUnavailable {

			nonCustomerErrCnt = 1
		}
		metrics = append(metrics,
			servicelogs.Counter(fmt.Sprintf(interop.ClientErrorReasonTemplate, e.error.ErrorType()), 1.0),
		)
	case model.CustomerError:
		customerErrCnt = 1
		metrics = append(metrics,
			servicelogs.Counter(fmt.Sprintf(interop.CustomerErrorReasonTemplate, e.error.ErrorType()), 1.0),
		)
	case model.PlatformError:
		platformErrCnt = 1
		nonCustomerErrCnt = 1
		metrics = append(metrics,
			servicelogs.Counter(fmt.Sprintf(interop.PlatformErrorReasonTemplate, e.error.ErrorType()), 1.0),
		)
	}

	metrics = append(metrics,
		servicelogs.Counter(interop.ClientErrorMetric, clientErrCnt),
		servicelogs.Counter(interop.CustomerErrorMetric, customerErrCnt),
		servicelogs.Counter(interop.PlatformErrorMetric, platformErrCnt),
		servicelogs.Counter(interop.NonCustomerErrorMetric, nonCustomerErrCnt),
	)
	return metrics
}
