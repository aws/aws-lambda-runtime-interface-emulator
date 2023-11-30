// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/telemetry"
)

type EventType = string

const (
	PlatformInitStart          = EventType("platform.initStart")
	PlatformInitRuntimeDone    = EventType("platform.initRuntimeDone")
	PlatformInitReport         = EventType("platform.initReport")
	PlatformRestoreRuntimeDone = EventType("platform.restoreRuntimeDone")
	PlatformStart              = EventType("platform.start")
	PlatformRuntimeDone        = EventType("platform.runtimeDone")
	PlatformExtension          = EventType("platform.extension")
	PlatformEnd                = EventType("platform.end")
	PlatformReport             = EventType("platform.report")
	PlatformFault              = EventType("platform.fault")
)

/*
SandboxEvent represents a generic sandbox event. For example:

	{
		"time": "2021-03-16T13:10:42.358Z",
		"type": "platform.extension",
		"platformEvent": { "name": "foo bar", "state": "Ready", "events": ["INVOKE", "SHUTDOWN"]}
	}

Or:

	{
		"time": "2021-03-16T13:10:42.358Z",
		"type": "extension",
		"logMessage": "raw agent console output"
	}

FluxPump produces entries with a single field 'record', containing either an object or a string.
We make the distinction explicit by providing separate fields for the two cases, 'PlatformEvent' and 'LogMessage'.
Either one of the two would be populated, but not both. This makes code cleaner, but requires test client to merge
two fields back, producing a single 'record' entry again -- to match the FluxPump format that tests actually check.
*/
type SandboxEvent struct {
	Time          string                 `json:"time"`
	Type          EventType              `json:"type"`
	PlatformEvent map[string]interface{} `json:"platformEvent,omitempty"`
	LogMessage    string                 `json:"logMessage,omitempty"`
}

type tailLogs struct {
	Events []SandboxEvent `json:"events,omitempty"`
}

type StandaloneEventsAPI struct {
	lock      sync.Mutex
	requestID interop.RequestID
	eventLog  EventLog
}

func (s *StandaloneEventsAPI) LogTrace(entry TracingEvent) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.eventLog.Traces = append(s.eventLog.Traces, entry)
}

func (s *StandaloneEventsAPI) EventLog() *EventLog {
	return &s.eventLog
}

func (s *StandaloneEventsAPI) SetCurrentRequestID(requestID interop.RequestID) {
	s.requestID = requestID
}

func (s *StandaloneEventsAPI) SendInitStart(data interop.InitStartData) error {
	record := map[string]interface{}{
		"initializationType": data.InitializationType,
		"runtimeVersion":     data.RuntimeVersion,
		"runtimeArn":         data.RuntimeVersionArn,
		"runtimeVersionArn":  data.RuntimeVersionArn,
		"functionArn":        data.FunctionArn,
		"functionName":       data.FunctionName,
		"functionVersion":    data.FunctionVersion,
		"instanceId":         data.InstanceID,
		"instanceMaxMemory":  data.InstanceMaxMemory,
		"phase":              data.Phase,
	}

	s.addTracingToRecord(data.Tracing, record)

	return s.sendPlatformEvent(PlatformInitStart, record)
}

func (s *StandaloneEventsAPI) SendInitRuntimeDone(data interop.InitRuntimeDoneData) error {
	record := map[string]interface{}{
		"initializationType": data.InitializationType,
		"status":             data.Status,
		"phase":              data.Phase,
	}

	s.addTracingToRecord(data.Tracing, record)

	if data.ErrorType != nil {
		record["errorType"] = data.ErrorType
	}

	return s.sendPlatformEvent(PlatformInitRuntimeDone, record)
}

func (s *StandaloneEventsAPI) SendInitReport(data interop.InitReportData) error {
	record := map[string]interface{}{
		"initializationType": data.InitializationType,
		"metrics":            data.Metrics,
		"phase":              data.Phase,
	}

	s.addTracingToRecord(data.Tracing, record)

	return s.sendPlatformEvent(PlatformInitReport, record)
}

func (s *StandaloneEventsAPI) SendRestoreRuntimeDone(data interop.RestoreRuntimeDoneData) error {
	record := map[string]interface{}{"status": data.Status}

	s.addTracingToRecord(data.Tracing, record)

	if data.ErrorType != nil {
		record["errorType"] = data.ErrorType
	}

	return s.sendPlatformEvent(PlatformRestoreRuntimeDone, record)
}

func (s *StandaloneEventsAPI) SendInvokeStart(data interop.InvokeStartData) error {
	record := map[string]interface{}{
		"version":   data.Version,
		"requestId": data.RequestID,
	}

	s.addTracingToRecord(data.Tracing, record)

	return s.sendPlatformEvent(PlatformStart, record)
}

func (s *StandaloneEventsAPI) SendInvokeRuntimeDone(data interop.InvokeRuntimeDoneData) error {
	record := map[string]interface{}{
		"requestId":       s.requestID,
		"status":          data.Status,
		"metrics":         data.Metrics,
		"internalMetrics": data.InternalMetrics,
		"spans":           data.Spans,
	}

	if data.ErrorType != nil {
		record["errorType"] = data.ErrorType
	}

	s.addTracingToRecord(data.Tracing, record)

	return s.sendPlatformEvent(PlatformRuntimeDone, record)
}

func (s *StandaloneEventsAPI) SendExtensionInit(data interop.ExtensionInitData) error {
	sort.Strings(data.Subscriptions)
	record := map[string]interface{}{
		"name":   data.AgentName,
		"state":  data.State,
		"events": data.Subscriptions,
	}
	if len(data.ErrorType) > 0 {
		record["errorType"] = data.ErrorType
	}
	return s.sendPlatformEvent(PlatformExtension, record)
}

func (s *StandaloneEventsAPI) SendImageErrorLog(interop.ImageErrorLogData) {
	// Called on bootstrap exec errors for OCI error modes, e.g. InvalidEntrypoint etc.
}

func (s *StandaloneEventsAPI) SendEnd(data interop.EndData) error {
	record := map[string]interface{}{
		"requestId": data.RequestID,
	}

	return s.sendPlatformEvent(PlatformEnd, record)
}

func (s *StandaloneEventsAPI) SendReportSpan(interop.Span) error {
	return nil
}

func (s *StandaloneEventsAPI) SendReport(data interop.ReportData) error {
	record := map[string]interface{}{
		"requestId": s.requestID,
		"status":    data.Status,
		"metrics":   data.Metrics,
		"spans":     data.Spans,
		"tracing":   data.Tracing,
	}
	if data.ErrorType != nil {
		record["errorType"] = data.ErrorType
	}

	return s.sendPlatformEvent(PlatformReport, record)
}

func (s *StandaloneEventsAPI) SendFault(data interop.FaultData) error {
	record := map[string]interface{}{
		"fault": data.String(),
	}

	return s.sendPlatformEvent(PlatformFault, record)
}

func (s *StandaloneEventsAPI) FetchTailLogs(string) (string, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.eventLog.Events) == 0 {
		return "", nil
	}

	logs := tailLogs{Events: s.eventLog.Events}
	logsBytes, err := json.Marshal(logs)
	if err != nil {
		return "", err
	}

	s.eventLog.Events = nil

	return string(logsBytes), nil
}

func (s *StandaloneEventsAPI) GetRuntimeDoneSpans(
	runtimeStartedTime int64,
	invokeResponseMetrics *interop.InvokeResponseMetrics,
	runtimeOverheadStartedTime int64,
	runtimeReadyTime int64,
) []interop.Span {
	spans := telemetry.GetRuntimeDoneSpans(runtimeStartedTime, invokeResponseMetrics)
	return spans
}

func (s *StandaloneEventsAPI) sendPlatformEvent(eventType string, record map[string]interface{}) error {
	e := SandboxEvent{
		Time:          time.Now().Format(time.RFC3339),
		Type:          eventType,
		PlatformEvent: record,
	}
	s.appendEvent(e)
	s.logEvent(e)
	return nil
}

func (s *StandaloneEventsAPI) sendLogEvent(eventType, logMessage string) error {
	e := SandboxEvent{
		Time:       time.Now().Format(time.RFC3339),
		Type:       eventType,
		LogMessage: logMessage,
	}
	s.appendEvent(e)
	s.logEvent(e)
	return nil
}

func (s *StandaloneEventsAPI) appendEvent(event SandboxEvent) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.eventLog.Events = append(s.eventLog.Events, event)
}

func (s *StandaloneEventsAPI) logEvent(e SandboxEvent) {
	log.WithField("event", e).Info("sandbox event")
}

func (s *StandaloneEventsAPI) addTracingToRecord(tracingData *interop.TracingCtx, record map[string]interface{}) {
	if tracingData != nil {
		record["tracing"] = map[string]string{
			"spanId": tracingData.SpanID,
			"type":   string(tracingData.Type),
			"value":  tracingData.Value,
		}
	}
}
