// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"sort"
	"time"

	"go.amzn.com/lambda/telemetry"
)

// EventType indicates the type of SandboxEvent. See full list:
type EventType = string

const (
	PlatformInitRuntimeDone    = EventType("platform.initRuntimeDone")
	PlatformRestoreRuntimeDone = EventType("platform.restoreRuntimeDone")
	PlatformRuntimeDone        = EventType("platform.runtimeDone")
	PlatformExtension          = EventType("platform.extension")
)

/*
	 SandboxEvent represents a generic sandbox event. For example:
	   {'time': '2021-03-16T13:10:42.358Z',
	    'type': 'platform.extension',
		'record': { "name": "foo bar", "state": "Ready", "events": ["INVOKE", "SHUTDOWN"]}}
*/
type SandboxEvent struct {
	Time   string                 `json:"time"`
	Type   EventType              `json:"type"`
	Record map[string]interface{} `json:"record"`
}

type StandaloneEventLog struct {
	requestID string
	eventLog  *EventLog
}

func (s *StandaloneEventLog) SetCurrentRequestID(requestID string) {
	s.requestID = requestID
}

func (s *StandaloneEventLog) SendInitRuntimeDone(data *telemetry.InitRuntimeDoneData) error {
	record := map[string]interface{}{"initializationType": data.InitSource, "status": data.Status}
	s.eventLog.Events = append(s.eventLog.Events, SandboxEvent{time.Now().Format(time.RFC3339), PlatformInitRuntimeDone, record})
	return nil
}

func (s *StandaloneEventLog) SendRestoreRuntimeDone(status string) error {
	record := map[string]interface{}{"status": status}
	s.eventLog.Events = append(s.eventLog.Events, SandboxEvent{time.Now().Format(time.RFC3339), PlatformRestoreRuntimeDone, record})
	return nil
}

func (s *StandaloneEventLog) SendRuntimeDone(data telemetry.InvokeRuntimeDoneData) error {
	// e.g. 'record': {'requestId': '1506eb3053d148f3bb7ec0fabe6f8d91','status': 'success', 'metrics': {...}, 'tracing': {...}}
	record := map[string]interface{}{
		"requestId":       s.requestID,
		"status":          data.Status,
		"metrics":         data.Metrics,
		"internalMetrics": data.InternalMetrics,
		"spans":           data.Spans,
	}

	if data.Tracing != nil {
		record["tracing"] = map[string]string{
			"spanId": data.Tracing.SpanID,
			"type":   string(data.Tracing.Type),
			"value":  data.Tracing.Value,
		}
	}

	s.eventLog.Events = append(s.eventLog.Events, SandboxEvent{time.Now().Format(time.RFC3339), PlatformRuntimeDone, record})
	return nil
}

func (s *StandaloneEventLog) SendExtensionInit(agentName, state, errorType string, subscriptions []string) error {
	// e.g. 'record': { "name": "", "state": "", errorType: "", events: [""] }
	sort.Strings(subscriptions)
	record := map[string]interface{}{"name": agentName, "state": state, "events": subscriptions}
	if len(errorType) > 0 {
		record["errorType"] = errorType
	}
	s.eventLog.Events = append(s.eventLog.Events, SandboxEvent{time.Now().Format(time.RFC3339), PlatformExtension, record})
	return nil
}

func (s *StandaloneEventLog) SendImageErrorLog(logline string) {
	// Called on bootstrap exec errors for OCI error modes, e.g. InvalidEntrypoint etc.
}

func NewStandaloneEventLog(eventLog *EventLog) *StandaloneEventLog {
	return &StandaloneEventLog{
		eventLog: eventLog,
	}
}
