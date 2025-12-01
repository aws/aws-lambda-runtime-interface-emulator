// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	intmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/ptr"
	rapimodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/servicelogs"
)

var (
	eventInvokeId        = "invoke-id"
	eventFunctionVersion = "function-version"
	eventFunctionArn     = "function-arn"
	eventTraceId         = "Root=12345;Parent=67890;Sampled=1;Lineage=22222"
	eventRuntimeErr      = model.NewCustomerError(model.ErrorRuntimeUnknown)
	eventTimeoutErr      = model.NewCustomerError(model.ErrorSandboxTimedout)
	eventInternalErr     = model.NewPlatformError(nil, model.ErrorReasonRuntimeExecFailed)

	invokeRequestSize int64 = 100

	dummyTracingCtx = &interop.TracingCtx{
		SpanID: "",
		Type:   rapimodel.XRayTracingType,
		Value:  eventTraceId,
	}

	dummyExpectedReportData = interop.ReportData{
		InvokeID: eventInvokeId,
		Metrics: interop.ReportMetrics{
			DurationMs: interop.ReportDurationMs(3000),
		},
		Tracing: &interop.TracingCtx{
			SpanID: "",
			Type:   rapimodel.XRayTracingType,
			Value:  eventTraceId,
		},
		Spans: []interop.Span{
			{
				Name:       "responseLatency",
				Start:      "0001-01-01T00:00:01.000Z",
				DurationMs: 1000.0,
			},
			{
				Name:       "responseDuration",
				Start:      "0001-01-01T00:00:02.000Z",
				DurationMs: 1000.0,
			},
		},
		ErrorType: nil,
	}
)

type invokeMetricsMocks struct {
	eventsApi       interop.MockEventsAPI
	initData        interop.MockInitStaticDataProvider
	invokeReq       interop.MockInvokeRequest
	logger          servicelogs.MockLogger
	counter         MockCounter
	responseMetrics interop.InvokeResponseMetrics
	timeStamp       time.Time
	error           model.AppError
}

func createInvokeEventsMocks(t *testing.T) *invokeMetricsMocks {
	mocks := invokeMetricsMocks{
		eventsApi: interop.MockEventsAPI{},
		initData:  interop.MockInitStaticDataProvider{},
		invokeReq: interop.MockInvokeRequest{},
		logger:    servicelogs.MockLogger{},
		counter:   MockCounter{},
	}

	mocks.invokeReq.On("InvokeID").Return(eventInvokeId)
	mocks.invokeReq.On("ResponseMode").Return("Streaming").Maybe()

	return &mocks
}

func checkMocksExpectations(t *testing.T, mocks *invokeMetricsMocks) {
	mocks.eventsApi.AssertExpectations(t)
	mocks.initData.AssertExpectations(t)
	mocks.invokeReq.AssertExpectations(t)
	mocks.logger.AssertExpectations(t)
}

func createInvokeEventsAndHijackGetTime(mocks *invokeMetricsMocks) *invokeMetrics {
	ev := NewInvokeMetrics(&mocks.logger, &mocks.counter)
	ev.AttachInvokeRequest(&mocks.invokeReq)
	ev.AttachDependencies(&mocks.initData, &mocks.eventsApi)
	ev.getCurrentTime = func() time.Time {
		return mocks.timeStamp
	}

	return ev
}

func Test_invokeMetrics_SendInvokeStart(t *testing.T) {
	mocks := createInvokeEventsMocks(t)
	ev := createInvokeEventsAndHijackGetTime(mocks)

	ev.TriggerStartRequest()

	mocks.initData.On("FunctionVersion").Return(eventFunctionVersion)
	mocks.initData.On("FunctionARN").Return(eventFunctionArn)
	mocks.eventsApi.On("SendInvokeStart", mock.MatchedBy(func(arg interop.InvokeStartData) bool {
		expected := interop.InvokeStartData{
			InvokeID:    eventInvokeId,
			Version:     eventFunctionVersion,
			FunctionARN: eventFunctionArn,
			Tracing: &interop.TracingCtx{
				SpanID: "",
				Type:   rapimodel.XRayTracingType,
				Value:  eventTraceId,
			},
		}

		return assert.Equal(t, expected, arg)
	})).Return(nil)

	err := ev.SendInvokeStartEvent(dummyTracingCtx)
	assert.NoError(t, err)
	checkMocksExpectations(t, mocks)
}

func Test_invokeMetrics_SendReport_FullCycle(t *testing.T) {
	tests := []struct {
		name         string
		runtimeError model.AppError
		status       interop.ResponseStatus
	}{
		{
			name:         "InvokeResponse",
			runtimeError: nil,
			status:       interop.Success,
		},
		{
			name:         "InvokeError",
			runtimeError: eventRuntimeErr,
			status:       interop.Error,
		},
		{
			name:         "InvokeFailure",
			runtimeError: eventInternalErr,
			status:       interop.Failure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := createInvokeEventsMocks(t)
			ev := createInvokeEventsAndHijackGetTime(mocks)

			ev.TriggerStartRequest()
			mocks.timeStamp = mocks.timeStamp.Add(time.Second)
			ev.TriggerSentRequest(invokeRequestSize, 11*time.Microsecond, 12*time.Microsecond)
			mocks.timeStamp = mocks.timeStamp.Add(time.Second)
			ev.TriggerGetResponse()
			mocks.timeStamp = mocks.timeStamp.Add(time.Second)
			ev.TriggerSentResponse(true, tt.runtimeError, nil, 0)

			mocks.eventsApi.On("SendReport", mock.MatchedBy(func(arg interop.ReportData) bool {
				expected := dummyExpectedReportData
				expected.Status = tt.status
				if tt.runtimeError != nil {
					errType := tt.runtimeError.ErrorType()
					expected.ErrorType = &errType
				}

				return assert.Equal(t, expected, arg)
			})).Return(nil)

			err := ev.SendInvokeFinishedEvent(dummyTracingCtx, nil)
			assert.NoError(t, err)
			checkMocksExpectations(t, mocks)
		})
	}
}

func Test_invokeMetrics_SendReport_NoResponse(t *testing.T) {
	mocks := createInvokeEventsMocks(t)
	ev := createInvokeEventsAndHijackGetTime(mocks)

	ev.TriggerStartRequest()
	mocks.timeStamp = mocks.timeStamp.Add(time.Second)
	ev.TriggerSentRequest(invokeRequestSize, 11*time.Microsecond, 12*time.Microsecond)
	mocks.timeStamp = mocks.timeStamp.Add(time.Second)
	ev.TriggerSentResponse(false, eventTimeoutErr, nil, 0)

	mocks.eventsApi.On("SendReport", mock.MatchedBy(func(arg interop.ReportData) bool {
		expected := dummyExpectedReportData
		expected.Status = interop.Timeout
		errType := eventTimeoutErr.ErrorType()
		expected.ErrorType = &errType

		expected.Metrics.DurationMs = interop.ReportDurationMs(2000)
		expected.Spans = []interop.Span{}

		return assert.Equal(t, expected, arg)
	})).Return(nil)

	err := ev.SendInvokeFinishedEvent(dummyTracingCtx, nil)
	assert.NoError(t, err)
	checkMocksExpectations(t, mocks)
}

func Test_invokeMetrics_SendReport_ResponseWithUnfinishedBody(t *testing.T) {
	mocks := createInvokeEventsMocks(t)
	ev := createInvokeEventsAndHijackGetTime(mocks)

	ev.TriggerStartRequest()
	mocks.timeStamp = mocks.timeStamp.Add(time.Second)
	ev.TriggerSentRequest(invokeRequestSize, 11*time.Microsecond, 12*time.Microsecond)
	mocks.timeStamp = mocks.timeStamp.Add(time.Second)
	ev.TriggerGetResponse()
	mocks.timeStamp = mocks.timeStamp.Add(time.Second)
	ev.TriggerSentResponse(false, eventTimeoutErr, nil, 0)

	mocks.eventsApi.On("SendReport", mock.MatchedBy(func(arg interop.ReportData) bool {
		expected := dummyExpectedReportData
		expected.Status = interop.Timeout
		errType := eventTimeoutErr.ErrorType()
		expected.ErrorType = &errType

		expected.Metrics.DurationMs = interop.ReportDurationMs(3000)
		expected.Spans = []interop.Span{
			{
				Name:       "responseLatency",
				Start:      "0001-01-01T00:00:01.000Z",
				DurationMs: 1000.0,
			},
		}

		return assert.Equal(t, expected, arg)
	})).Return(nil)

	err := ev.SendInvokeFinishedEvent(dummyTracingCtx, nil)
	assert.NoError(t, err)
	checkMocksExpectations(t, mocks)
}

func Test_invokeMetrics_SendReport_WithXrayErrorCause(t *testing.T) {
	mocks := createInvokeEventsMocks(t)
	ev := createInvokeEventsAndHijackGetTime(mocks)

	ev.TriggerStartRequest()
	mocks.timeStamp = mocks.timeStamp.Add(time.Second)
	ev.TriggerSentRequest(invokeRequestSize, 11*time.Microsecond, 12*time.Microsecond)
	mocks.timeStamp = mocks.timeStamp.Add(time.Second)
	ev.TriggerGetResponse()
	mocks.timeStamp = mocks.timeStamp.Add(time.Second)
	ev.TriggerSentResponse(true, eventRuntimeErr, nil, 0)

	xrayErrorCause := json.RawMessage(`{"exceptions":[{"message":"Null pointer exception","type":"RuntimeError"}],"working_directory":"","paths":[]}`)
	mocks.eventsApi.On("SendInternalXRayErrorCause", mock.MatchedBy(func(arg interop.InternalXRayErrorCauseData) bool {
		expected := interop.InternalXRayErrorCauseData{
			InvokeID: eventInvokeId,
			Cause:    string(xrayErrorCause),
		}
		return assert.Equal(t, expected, arg)
	})).Return(nil)

	mocks.eventsApi.On("SendReport", mock.MatchedBy(func(arg interop.ReportData) bool {
		expected := dummyExpectedReportData
		expected.Status = interop.Error
		errType := eventRuntimeErr.ErrorType()
		expected.ErrorType = &errType

		return assert.Equal(t, expected, arg)
	})).Return(nil)

	err := ev.SendInvokeFinishedEvent(dummyTracingCtx, xrayErrorCause)
	assert.NoError(t, err)
	checkMocksExpectations(t, mocks)
}

func Test_invokeMetrics_TriggerInvokeDone(t *testing.T) {
	tests := []struct {
		name                string
		setTimeStartRequest bool
		expectedRunMs       *time.Duration
	}{
		{
			name:                "timeStartRequest_empty",
			setTimeStartRequest: false,
			expectedRunMs:       nil,
		},
		{
			name:                "timeStartRequest_non_empty",
			setTimeStartRequest: true,
			expectedRunMs:       ptr.To(3 * time.Second),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			im := NewInvokeMetrics(nil, NewMockCounter(t))
			now := time.Now()
			im.getCurrentTime = func() time.Time {
				return now
			}
			im.TriggerGetRequest()

			now = now.Add(2 * time.Second)
			if tt.setTimeStartRequest {
				im.TriggerStartRequest()
			}

			now = now.Add(3 * time.Second)
			totalMs, runMs, initData := im.TriggerInvokeDone()

			assert.Equal(t, 5*time.Second, totalMs)
			assert.Equal(t, tt.expectedRunMs, runMs)
			assert.Nil(t, initData)
		})
	}
}

func Test_invokeMetrics_ServiceLogs(t *testing.T) {
	tests := []struct {
		name            string
		expectedBytes   uint64
		metricFlow      func(ev *invokeMetrics, mocks *invokeMetricsMocks)
		expectedProps   []servicelogs.Property
		expectedDims    []servicelogs.Dimension
		expectedMetrics []servicelogs.Metric
	}{
		{
			name:          "minimal_invoke_flow",
			expectedBytes: 0,
			metricFlow: func(ev *invokeMetrics, mocks *invokeMetricsMocks) {
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				mocks.error = model.NewClientError(nil, model.ErrorSeverityInvalid, model.ErrorInvalidFunctionVersion)
			},
			expectedProps: []servicelogs.Property{},
			expectedDims:  []servicelogs.Dimension{},
			expectedMetrics: []servicelogs.Metric{
				{Type: servicelogs.TimerType, Key: "TotalDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "InflightRequestCount", Value: 0},
				{Type: servicelogs.CounterType, Key: "IdleRuntimesCount", Value: 0},
				{Type: servicelogs.TimerType, Key: "PlatformOverheadDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "ClientError", Value: 1},
				{Type: servicelogs.CounterType, Key: "CustomerError", Value: 0},
				{Type: servicelogs.CounterType, Key: "PlatformError", Value: 0},
				{Type: servicelogs.CounterType, Key: "ClientErrorReason-ErrInvalidFunctionVersion", Value: 1},
				{Type: servicelogs.CounterType, Key: "NonCustomerError", Value: 1},
			},
		},
		{
			name:          "bad_request_invoke_flow",
			expectedBytes: 0,
			metricFlow: func(ev *invokeMetrics, mocks *invokeMetricsMocks) {
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				ev.AttachInvokeRequest(&mocks.invokeReq)
				mocks.error = model.NewClientError(nil, model.ErrorSeverityError, model.ErrorInitIncomplete)
			},
			expectedProps: []servicelogs.Property{
				{Name: "RequestId", Value: "invoke-id"},
			},
			expectedDims: []servicelogs.Dimension{
				{Name: "RequestMode", Value: "Streaming"},
			},
			expectedMetrics: []servicelogs.Metric{
				{Type: servicelogs.TimerType, Key: "TotalDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "InflightRequestCount", Value: 0},
				{Type: servicelogs.CounterType, Key: "IdleRuntimesCount", Value: 0},
				{Type: servicelogs.TimerType, Key: "PlatformOverheadDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "ClientError", Value: 1},
				{Type: servicelogs.CounterType, Key: "CustomerError", Value: 0},
				{Type: servicelogs.CounterType, Key: "PlatformError", Value: 0},
				{Type: servicelogs.CounterType, Key: "ClientErrorReason-Client.InitIncomplete", Value: 1},
				{Type: servicelogs.CounterType, Key: "NonCustomerError", Value: 1},
			},
		},
		{
			name:          "runtime_unavailable_error",
			expectedBytes: 0,
			metricFlow: func(ev *invokeMetrics, mocks *invokeMetricsMocks) {
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				ev.AttachInvokeRequest(&mocks.invokeReq)
				ev.AttachDependencies(&mocks.initData, &mocks.eventsApi)
				ev.UpdateConcurrencyMetrics(5, 3)
				mocks.error = model.NewClientError(nil, model.ErrorSeverityError, model.ErrorRuntimeUnavailable)
			},
			expectedProps: []servicelogs.Property{
				{Name: "RequestId", Value: "invoke-id"},
			},
			expectedDims: []servicelogs.Dimension{
				{Name: "RequestMode", Value: "Streaming"},
			},
			expectedMetrics: []servicelogs.Metric{
				{Type: servicelogs.TimerType, Key: "TotalDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "InflightRequestCount", Value: 5},
				{Type: servicelogs.CounterType, Key: "IdleRuntimesCount", Value: 3},
				{Type: servicelogs.TimerType, Key: "PlatformOverheadDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "ClientError", Value: 1},
				{Type: servicelogs.CounterType, Key: "CustomerError", Value: 0},
				{Type: servicelogs.CounterType, Key: "PlatformError", Value: 0},
				{Type: servicelogs.CounterType, Key: "ClientErrorReason-Runtime.Unavailable", Value: 1},
				{Type: servicelogs.CounterType, Key: "NonCustomerError", Value: 0},
			},
		},
		{
			name:          "runtime_timeout_flow",
			expectedBytes: 100,
			metricFlow: func(ev *invokeMetrics, mocks *invokeMetricsMocks) {
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				ev.AttachInvokeRequest(&mocks.invokeReq)
				ev.AttachDependencies(&mocks.initData, &mocks.eventsApi)
				ev.UpdateConcurrencyMetrics(5, 3)
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				ev.TriggerStartRequest()
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				ev.TriggerSentRequest(100, 11*time.Microsecond, 12*time.Microsecond)
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				mocks.error = model.NewCustomerError(model.ErrorSandboxTimedout)
				ev.TriggerSentResponse(false, mocks.error, nil, 0)
			},
			expectedProps: []servicelogs.Property{
				{Name: "RequestId", Value: "invoke-id"},
			},
			expectedDims: []servicelogs.Dimension{
				{Name: "RequestMode", Value: "Streaming"},
			},
			expectedMetrics: []servicelogs.Metric{
				{Type: servicelogs.TimerType, Key: "TotalDuration", Value: 4000000},
				{Type: servicelogs.CounterType, Key: "InflightRequestCount", Value: 5},
				{Type: servicelogs.CounterType, Key: "IdleRuntimesCount", Value: 3},
				{Type: servicelogs.TimerType, Key: "PlatformOverheadDuration", Value: 2000000},
				{Type: servicelogs.TimerType, Key: "FunctionDuration", Value: 2000000},
				{Type: servicelogs.TimerType, Key: "RequestSendDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "RequestPayloadSizeBytes", Value: 100},
				{Type: servicelogs.TimerType, Key: "RequestPayloadReadDuration", Value: 11},
				{Type: servicelogs.TimerType, Key: "RequestPayloadWriteDuration", Value: 12},
				{Type: servicelogs.CounterType, Key: "ClientError", Value: 0},
				{Type: servicelogs.CounterType, Key: "CustomerError", Value: 1},
				{Type: servicelogs.CounterType, Key: "PlatformError", Value: 0},
				{Type: servicelogs.CounterType, Key: "CustomerErrorReason-Sandbox.Timedout", Value: 1},
				{Type: servicelogs.CounterType, Key: "NonCustomerError", Value: 0},
			},
		},
		{
			name:          "full_invoke_flow",
			expectedBytes: 200,
			metricFlow: func(ev *invokeMetrics, mocks *invokeMetricsMocks) {
				ev.AttachInvokeRequest(&mocks.invokeReq)
				ev.AttachDependencies(&mocks.initData, &mocks.eventsApi)
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				ev.UpdateConcurrencyMetrics(5, 3)
				ev.TriggerStartRequest()
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				ev.TriggerSentRequest(100, 11*time.Microsecond, 12*time.Microsecond)
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				ev.TriggerGetResponse()
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				ev.TriggerSentResponse(true, nil, &mocks.responseMetrics, 0)
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
			},
			expectedProps: []servicelogs.Property{
				{Name: "RequestId", Value: "invoke-id"},
			},
			expectedDims: []servicelogs.Dimension{
				{Name: "RequestMode", Value: "Streaming"},
				{Name: "ResponseMode", Value: "Streaming"},
			},
			expectedMetrics: []servicelogs.Metric{
				{Type: servicelogs.TimerType, Key: "TotalDuration", Value: 5000000},
				{Type: servicelogs.CounterType, Key: "InflightRequestCount", Value: 5},
				{Type: servicelogs.CounterType, Key: "IdleRuntimesCount", Value: 3},
				{Type: servicelogs.CounterType, Key: "ResponsePayloadSizeBytes", Value: 100},
				{Type: servicelogs.TimerType, Key: "ResponseThrottledDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "ResponseThroughput", Value: 100},
				{Type: servicelogs.TimerType, Key: "ResponsePayloadReadDuration", Value: 13},
				{Type: servicelogs.TimerType, Key: "ResponsePayloadWriteDuration", Value: 14},
				{Type: servicelogs.TimerType, Key: "PlatformOverheadDuration", Value: 2000000},
				{Type: servicelogs.TimerType, Key: "FunctionDuration", Value: 3000000},
				{Type: servicelogs.TimerType, Key: "RequestSendDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "RequestPayloadSizeBytes", Value: 100},
				{Type: servicelogs.TimerType, Key: "RequestPayloadReadDuration", Value: 11},
				{Type: servicelogs.TimerType, Key: "RequestPayloadWriteDuration", Value: 12},
				{Type: servicelogs.TimerType, Key: "ResponseLatency", Value: 1000000},
				{Type: servicelogs.TimerType, Key: "ResponseDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "ClientError", Value: 0},
				{Type: servicelogs.CounterType, Key: "CustomerError", Value: 0},
				{Type: servicelogs.CounterType, Key: "PlatformError", Value: 0},
				{Type: servicelogs.CounterType, Key: "NonCustomerError", Value: 0},
			},
		},
		{
			name:          "full_invoke_error_flow",
			expectedBytes: 300,
			metricFlow: func(ev *invokeMetrics, mocks *invokeMetricsMocks) {
				ev.AttachInvokeRequest(&mocks.invokeReq)
				ev.AttachDependencies(&mocks.initData, &mocks.eventsApi)
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				ev.UpdateConcurrencyMetrics(2, 1)
				ev.TriggerStartRequest()
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				ev.TriggerSentRequest(100, 11*time.Microsecond, 12*time.Microsecond)
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				ev.TriggerGetResponse()
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				mocks.error = model.NewCustomerError(model.ErrorRuntimeUnknown)
				ev.TriggerSentResponse(true, mocks.error, &interop.InvokeResponseMetrics{ProducedBytes: 100, FunctionResponseMode: runtimeResponseModeStreaming}, 100)
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
			},
			expectedProps: []servicelogs.Property{
				{Name: "RequestId", Value: "invoke-id"},
			},
			expectedDims: []servicelogs.Dimension{
				{Name: "RequestMode", Value: "Streaming"},
				{Name: "ResponseMode", Value: "streaming"},
			},
			expectedMetrics: []servicelogs.Metric{
				{Type: servicelogs.TimerType, Key: "TotalDuration", Value: 5000000},
				{Type: servicelogs.CounterType, Key: "InflightRequestCount", Value: 2},
				{Type: servicelogs.CounterType, Key: "IdleRuntimesCount", Value: 1},
				{Type: servicelogs.CounterType, Key: "ResponsePayloadSizeBytes", Value: 100},
				{Type: servicelogs.TimerType, Key: "ResponseThrottledDuration", Value: 0},
				{Type: servicelogs.CounterType, Key: "ResponseThroughput", Value: 0},
				{Type: servicelogs.TimerType, Key: "ResponsePayloadReadDuration", Value: 0},
				{Type: servicelogs.TimerType, Key: "ResponsePayloadWriteDuration", Value: 0},
				{Type: servicelogs.CounterType, Key: "ErrorPayloadSizeBytes", Value: 100},
				{Type: servicelogs.TimerType, Key: "PlatformOverheadDuration", Value: 2000000},
				{Type: servicelogs.TimerType, Key: "FunctionDuration", Value: 3000000},
				{Type: servicelogs.TimerType, Key: "RequestSendDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "RequestPayloadSizeBytes", Value: 100},
				{Type: servicelogs.TimerType, Key: "RequestPayloadReadDuration", Value: 11},
				{Type: servicelogs.TimerType, Key: "RequestPayloadWriteDuration", Value: 12},
				{Type: servicelogs.TimerType, Key: "ResponseLatency", Value: 1000000},
				{Type: servicelogs.TimerType, Key: "ResponseDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "ClientError", Value: 0},
				{Type: servicelogs.CounterType, Key: "CustomerError", Value: 1},
				{Type: servicelogs.CounterType, Key: "PlatformError", Value: 0},
				{Type: servicelogs.CounterType, Key: "CustomerErrorReason-Runtime.Unknown", Value: 1},
				{Type: servicelogs.CounterType, Key: "NonCustomerError", Value: 0},
			},
		},
	}

	tupleSortFunc := func(a, b servicelogs.Tuple) int {
		return cmp.Compare(a.Name, b.Name)
	}

	metricsSortFunc := func(a, b servicelogs.Metric) int {
		return cmp.Compare(a.Key, b.Key)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := &invokeMetricsMocks{
				eventsApi: interop.MockEventsAPI{},
				initData:  interop.MockInitStaticDataProvider{},
				invokeReq: interop.MockInvokeRequest{},
				logger:    servicelogs.MockLogger{},
				counter:   MockCounter{},
			}
			mocks.counter.On("AddInvoke", tt.expectedBytes)

			ev := NewInvokeMetrics(&mocks.logger, &mocks.counter)
			ev.getCurrentTime = func() time.Time {
				return mocks.timeStamp
			}

			mocks.responseMetrics = interop.InvokeResponseMetrics{
				TimeShaped:                   time.Second,
				ProducedBytes:                100,
				OutboundThroughputBps:        100,
				FunctionResponseMode:         "Streaming",
				ResponsePayloadReadDuration:  13 * time.Microsecond,
				ResponsePayloadWriteDuration: 14 * time.Microsecond,
			}

			mocks.invokeReq.On("InvokeID").Return("invoke-id").Maybe()
			mocks.invokeReq.On("ResponseMode").Return("Streaming").Maybe().Maybe()
			mocks.invokeReq.On("TraceId").Return("Root=12345;Parent=67890;Sampled=1;Lineage=22222").Maybe()

			mocks.initData.On("XRayTracingMode").Return(intmodel.XRayTracingModePassThrough).Maybe()
			mocks.initData.On("MemorySizeMB").Return(uint64(128)).Maybe()
			mocks.initData.On("FunctionARN").Return("function-arn").Maybe()
			mocks.initData.On("FunctionVersionID").Return("function-version-id").Maybe()
			mocks.initData.On("FunctionTimeout").Return(time.Second).Maybe()
			mocks.initData.On("RuntimeVersion").Return("python3.9").Maybe()
			mocks.initData.On("ArtefactType").Return(intmodel.ArtefactTypeOCI).Maybe()
			mocks.initData.On("AmiId").Return("ami-1234567").Maybe()
			mocks.initData.On("AvailabilityZoneId").Return("us-west-2").Maybe()

			mocks.logger.On("Log",
				mock.MatchedBy(func(op servicelogs.Operation) bool {
					return assert.Equal(t, servicelogs.InvokeOp, op)
				}),
				mock.AnythingOfType("time.Time"),
				mock.MatchedBy(func(props []servicelogs.Property) bool {
					slices.SortFunc(props, tupleSortFunc)
					slices.SortFunc(tt.expectedProps, tupleSortFunc)
					assert.Equal(t, len(tt.expectedProps), len(props))
					for i := range len(tt.expectedProps) {
						assert.Equal(t, tt.expectedProps[i].Name, props[i].Name)
						assert.Equal(t, tt.expectedProps[i].Value, props[i].Value)
					}

					return true
				}),
				mock.MatchedBy(func(dims []servicelogs.Dimension) bool {
					slices.SortFunc(dims, tupleSortFunc)
					slices.SortFunc(tt.expectedDims, tupleSortFunc)
					assert.Equal(t, len(tt.expectedDims), len(dims))
					for i := range len(tt.expectedDims) {
						assert.Equal(t, tt.expectedDims[i].Name, dims[i].Name)
						assert.Equal(t, tt.expectedDims[i].Value, dims[i].Value)
					}

					return true
				}),
				mock.MatchedBy(func(metrics []servicelogs.Metric) bool {
					slices.SortFunc(metrics, metricsSortFunc)
					slices.SortFunc(tt.expectedMetrics, metricsSortFunc)
					assert.Equal(t, len(tt.expectedMetrics), len(metrics))
					for i := range len(tt.expectedMetrics) {
						require.Equal(t, tt.expectedMetrics[i].Key, metrics[i].Key)
						require.Equal(t, tt.expectedMetrics[i].Type, metrics[i].Type, fmt.Sprintf("wrong format for %s", metrics[i].Key))
						require.Equal(t, tt.expectedMetrics[i].Value, metrics[i].Value, fmt.Sprintf("wrong value for %s", metrics[i].Key))
					}

					return true
				}),
			).Once()

			mocks.timeStamp = time.Now()
			ev.TriggerGetRequest()

			tt.metricFlow(ev, mocks)

			ev.TriggerInvokeDone()
			require.NoError(t, ev.SendMetrics(mocks.error))
			checkMocksExpectations(t, mocks)
		})
	}
}
