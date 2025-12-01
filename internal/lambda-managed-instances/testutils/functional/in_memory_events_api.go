// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build test

package functional

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/invoke"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
)

type EventType string

const (
	PlatformInitStart       EventType = "platform.initStart"
	PlatformInitRuntimeDone EventType = "platform.initRuntimeDone"
	PlatformInitReport      EventType = "platform.initReport"
	PlatformRuntimeStart    EventType = "platform.runtimeStart"
	PlatformReport          EventType = "platform.report"
)

type ExpectedInitEvent struct {
	EventType EventType
	Status    string
	ErrorType string
}

type ExpectedExtensionEvents struct {
	ExtensionName string
	State         string
	ErrorType     string
}

type ExpectedInvokeEvents struct {
	EventType EventType

	Status string

	Spans []string
}

type ExpectedGlobalData struct {
	FunctionARN     string
	FunctionVersion string
}

type InMemoryEventsApi struct {
	testState *testing.T
	mu        sync.Mutex

	initStart       *interop.InitStartData
	initRuntimeDone *interop.InitRuntimeDoneData
	initReport      *interop.InitReportData
	imageError      *interop.ImageErrorLogData

	initEventsCount int

	extensionInit map[string]interop.ExtensionInitData

	invokeStart          map[interop.InvokeID]interop.InvokeStartData
	invokeReport         map[interop.InvokeID]interop.ReportData
	InvokeXRAYErrorCause map[interop.InvokeID]interop.InternalXRayErrorCauseData

	logLines []telemetry.Event
}

func NewInMemoryEventsApi(t *testing.T) *InMemoryEventsApi {
	return &InMemoryEventsApi{
		testState:            t,
		extensionInit:        make(map[string]interop.ExtensionInitData),
		invokeStart:          make(map[interop.InvokeID]interop.InvokeStartData),
		invokeReport:         make(map[interop.InvokeID]interop.ReportData),
		InvokeXRAYErrorCause: make(map[interop.InvokeID]interop.InternalXRayErrorCauseData),
	}
}

func (e *InMemoryEventsApi) SendInitStart(data interop.InitStartData) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.initEventsCount++

	if e.initStart != nil {
		assert.FailNow(e.testState, "InitStart already exists")
	}

	e.initStart = &data
	return nil
}

func (e *InMemoryEventsApi) SendInitRuntimeDone(data interop.InitRuntimeDoneData) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.initEventsCount++

	if e.initRuntimeDone != nil {
		assert.FailNow(e.testState, "InitRuntimeDone already exists")
	}

	e.initRuntimeDone = &data
	return nil
}

func (e *InMemoryEventsApi) SendInitReport(data interop.InitReportData) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.initEventsCount++

	if e.initReport != nil {
		assert.FailNow(e.testState, "InitReport already exists")
	}

	e.initReport = &data
	return nil
}

func (e *InMemoryEventsApi) SendExtensionInit(data interop.ExtensionInitData) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.extensionInit[data.AgentName]; ok {
		assert.FailNow(e.testState, "ExtensionInit for %s already exists", data.AgentName)
	}

	e.extensionInit[data.AgentName] = data
	return nil
}

func (e *InMemoryEventsApi) SendImageError(errLog interop.ImageErrorLogData) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.imageError != nil {
		assert.FailNow(e.testState, "SendImageError already exists")
	}

	e.imageError = &errLog
}

func (e *InMemoryEventsApi) SendInvokeStart(data interop.InvokeStartData) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.invokeStart[data.InvokeID]; ok {
		assert.FailNow(e.testState, "InvokeStart for %s already exists", data.InvokeID)
	}

	e.invokeStart[data.InvokeID] = data
	return nil
}

func (e *InMemoryEventsApi) SendReport(data interop.ReportData) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.invokeReport[data.InvokeID]; ok {
		assert.FailNow(e.testState, "InvokeReport for %s already exists", data.InvokeID)
	}

	e.invokeReport[data.InvokeID] = data
	return nil
}

func (e *InMemoryEventsApi) SendInternalXRayErrorCause(data interop.InternalXRayErrorCauseData) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.InvokeXRAYErrorCause[data.InvokeID]; ok {
		assert.FailNow(e.testState, "SendInternalXRayErrorCause for %s already exists", data.InvokeID)
	}

	e.InvokeXRAYErrorCause[data.InvokeID] = data
	return nil
}

func (e *InMemoryEventsApi) CheckSimpleInitExpectations(startTimestamp time.Time, finishTimestamp time.Time, expectedInitEvents []ExpectedInitEvent, initReq model.InitRequestMessage) {

	e.CheckComprehensiveInitExpectations(startTimestamp, finishTimestamp, 0, expectedInitEvents, initReq)
}

func (e *InMemoryEventsApi) CheckComprehensiveInitExpectations(startTimestamp time.Time, finishTimestamp time.Time, expectedMinimalInitDuration time.Duration, expectedInitEvents []ExpectedInitEvent, initReq model.InitRequestMessage) {
	if expectedInitEvents == nil {
		return
	}

	require.Equal(e.testState, len(expectedInitEvents), e.initEventsCount)

	for _, initEvent := range expectedInitEvents {
		switch initEvent.EventType {
		case PlatformInitStart:
			assert.NotNil(e.testState, e.initStart)
			assert.Equal(e.testState, interop.InitializationType, e.initStart.InitializationType)
			assert.Equal(e.testState, initReq.RuntimeVersion, e.initStart.RuntimeVersion)
			assert.Equal(e.testState, initReq.RuntimeArn, e.initStart.RuntimeVersionArn)
			assert.Equal(e.testState, initReq.TaskName, e.initStart.FunctionName)
			assert.Equal(e.testState, initReq.FunctionVersion, e.initStart.FunctionVersion)
			assert.Equal(e.testState, initReq.LogStreamName, e.initStart.InstanceID)
			assert.Equal(e.testState, uint64(initReq.MemorySizeBytes), e.initStart.InstanceMaxMemory)
			assert.Equal(e.testState, telemetry.InitInsideInitPhase, e.initStart.Phase)
			assert.Nil(e.testState, e.initStart.Tracing)
		case PlatformInitRuntimeDone:
			assert.NotNil(e.testState, e.initRuntimeDone)
			assert.Equal(e.testState, interop.InitializationType, e.initRuntimeDone.InitializationType)
			assert.Equal(e.testState, initEvent.Status, e.initRuntimeDone.Status)
			assert.Equal(e.testState, telemetry.InitInsideInitPhase, e.initRuntimeDone.Phase)
			checkErrorTypePtr(e.testState, initEvent.ErrorType, e.initRuntimeDone.ErrorType)
			assert.Nil(e.testState, e.initRuntimeDone.Tracing)
		case PlatformInitReport:
			assert.NotNil(e.testState, e.initReport)
			assert.Equal(e.testState, interop.InitializationType, e.initReport.InitializationType)
			assert.Equal(e.testState, telemetry.InitInsideInitPhase, e.initReport.Phase)
			assert.Nil(e.testState, e.initReport.Tracing)
			assert.Equal(e.testState, initEvent.Status, e.initReport.Status)
			checkErrorTypePtr(e.testState, initEvent.ErrorType, e.initReport.ErrorType)
			checkDuration(e.testState, expectedMinimalInitDuration, finishTimestamp.Sub(startTimestamp), e.initReport.Metrics.DurationMs)
		}
	}
}

func checkErrorTypePtr(t *testing.T, expectedErr string, realErrPtr *string) {
	if expectedErr == "" {
		assert.Nil(t, realErrPtr)
	} else {
		assert.Equal(t, expectedErr, *realErrPtr)
	}
}

func (e *InMemoryEventsApi) CheckSimpleExtensionExpectations(expectedExtensionsEvents []ExpectedExtensionEvents) {
	if expectedExtensionsEvents == nil {
		return
	}

	require.Equal(e.testState, len(expectedExtensionsEvents), len(e.extensionInit))

	for _, expectedEvent := range expectedExtensionsEvents {
		event, ok := e.extensionInit[expectedEvent.ExtensionName]
		assert.True(e.testState, ok)
		assert.Equal(e.testState, expectedEvent.ErrorType, event.ErrorType)
		assert.Equal(e.testState, expectedEvent.State, event.State)
	}
}

func (e *InMemoryEventsApi) CheckSimpleInvokeExpectations(startTimestamp time.Time, finishTimestamp time.Time, invokeID interop.InvokeID, expectedInvokeEvents []ExpectedInvokeEvents, initReq model.InitRequestMessage) {

	e.CheckComprehensiveInvokeExpectations(startTimestamp, finishTimestamp, invokeID, expectedInvokeEvents, initReq, 0, 0)
}

func (e *InMemoryEventsApi) CheckComprehensiveInvokeExpectations(startTimestamp time.Time, finishTimestamp time.Time, invokeID interop.InvokeID, expectedInvokeEvents []ExpectedInvokeEvents, initReq model.InitRequestMessage, expectedInvokeLatency time.Duration, expectedInvokeRespDuration time.Duration) {
	if expectedInvokeEvents == nil {
		return
	}

	maxDuration := finishTimestamp.Sub(startTimestamp)
	for _, expectedInvokeEvent := range expectedInvokeEvents {
		switch expectedInvokeEvent.EventType {
		case PlatformRuntimeStart:
			event := e.invokeStart[invokeID]
			require.NotEmpty(e.testState, event)
			assert.Equal(e.testState, initReq.FunctionARN, event.FunctionARN)
			assert.Equal(e.testState, initReq.FunctionVersion, event.Version)
		case PlatformReport:
			event := e.invokeReport[invokeID]
			require.NotEmpty(e.testState, event)
			assert.Equal(e.testState, expectedInvokeEvent.Status, event.Status)
			checkDuration(e.testState, expectedInvokeLatency+expectedInvokeRespDuration, maxDuration, float64(event.Metrics.DurationMs))
			require.Equal(e.testState, len(expectedInvokeEvent.Spans), len(event.Spans))

			for i := range len(expectedInvokeEvent.Spans) {
				assert.Equal(e.testState, expectedInvokeEvent.Spans[i], event.Spans[i].Name)
				spanStartTime, err := time.ParseInLocation("2006-01-02T15:04:05.000Z", event.Spans[i].Start, time.UTC)
				assert.NoError(e.testState, err)
				checkTimestamp(e.testState, spanStartTime, startTimestamp, finishTimestamp)

				var minDuration time.Duration
				switch expectedInvokeEvent.Spans[i] {
				case invoke.ResponseLatencySpanName:
					minDuration = expectedInvokeLatency
				case invoke.ResponseDurationSpanName:
					minDuration = expectedInvokeRespDuration
				default:

				}
				checkDuration(e.testState, minDuration, maxDuration, event.Spans[i].DurationMs)
			}
		}
	}
}

func (e *InMemoryEventsApi) CheckXRayErrorCauseExpectations(invokeID interop.InvokeID, expectedErrorCause string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if expectedErrorCause == "" {
		_, exists := e.InvokeXRAYErrorCause[invokeID]
		assert.False(e.testState, exists, "Expected no XRay error cause for invoke %s, but one was recorded", invokeID)
		return
	}

	errorCauseData, exists := e.InvokeXRAYErrorCause[invokeID]
	assert.True(e.testState, exists, "Expected XRay error cause for invoke %s, but none was recorded", invokeID)
	if exists {
		assert.Equal(e.testState, expectedErrorCause, errorCauseData.Cause, "XRay error cause mismatch for invoke %s", invokeID)
	}
}

func (e *InMemoryEventsApi) Flush() {

}

func (e *InMemoryEventsApi) RecordLogLine(ev telemetry.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.logLines = append(e.logLines, ev)
}

func (e *InMemoryEventsApi) LogLines() []telemetry.Event {
	e.mu.Lock()
	defer e.mu.Unlock()

	lines := make([]telemetry.Event, len(e.logLines))
	copy(lines, e.logLines)

	return lines
}

func checkDuration(t *testing.T, minDuration, maxDuration time.Duration, realDuration float64) {
	assert.GreaterOrEqual(t, realDuration, getDurationMs(minDuration))
	assert.LessOrEqual(t, realDuration, getDurationMs(maxDuration))
}

func getDurationMs(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

func checkTimestamp(t *testing.T, realTimestamp, leftBound, rightBound time.Time) {
	assert.WithinRange(t, realTimestamp, leftBound.Truncate(time.Millisecond), rightBound.Truncate(time.Millisecond))
}
