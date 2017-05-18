// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
	"sync"
	"time"
)

type XrayEvent struct {
	Msg         string `json:"msg"`
	TraceID     string `json:"traceID"`
	SegmentName string `json:"segmentName"`
	SegmentID   string `json:"segmentID"`
	Timestamp   int64  `json:"timestamp"`
}

// PlatformLogEvent represents a platform-generated customer log entry
type PlatformLogEvent struct {
	Name          string   `json:"name"`
	State         string   `json:"state"`
	ErrorType     string   `json:"errorType"`
	Subscriptions []string `json:"subscriptions"`
}

// FunctionLogEvent represents a runtime-generated customer log entry
type FunctionLogEvent struct{}

// ExtensionLogEvent represents an agent-generated customer log entry
type ExtensionLogEvent struct{}

type EventLog struct {
	Xray        []XrayEvent        `json:"xray,omitempty"`
	PlatformLog []PlatformLogEvent `json:"platformLogs,omitempty"`
	Logs        []string           `json:"rawLogs,omitempty"`
	mutex       sync.Mutex
}

func (p *EventLog) LogXrayEvent(msg string, traceID string, segmentName string, segmentID string) {
	p.Xray = append(p.Xray, XrayEvent{Msg: msg, TraceID: traceID, SegmentName: segmentName, SegmentID: segmentID, Timestamp: time.Now().UnixNano() / int64(time.Millisecond)})
}

func (p *EventLog) LogExtensionInitEvent(agentName string, state string, subscriptions string, errorType string) {
	p.PlatformLog = append(p.PlatformLog, PlatformLogEvent{agentName, state, errorType, strings.Split(subscriptions, ",")})
}

func parseLogString(s string) []string {
	elems := strings.Split(s, "\t")[1:]
	for i, e := range elems {
		elems[i] = strings.Split(e, ": ")[1]
		elems[i] = strings.TrimSuffix(elems[i], "\n")
		elems[i] = strings.TrimPrefix(elems[i], "[")
		elems[i] = strings.TrimSuffix(elems[i], "]")
	}
	return elems
}

func (p *EventLog) dispatchLogEvent(logStr string) {
	elems := parseLogString(logStr)
	if strings.HasPrefix(logStr, "XRAY") {
		// format: 'XRAY\tMessage: %s\tTraceID: %s\tSegmentName: %s\tSegmentID: %s'
		msg, traceID, segmentName, segmentID := elems[0], elems[1], elems[2], elems[3]
		p.LogXrayEvent(msg, traceID, segmentName, segmentID)
	}

	if strings.HasPrefix(logStr, "EXTENSION") && strings.Contains(logStr, "Error Type") {
		// format: 'EXTENSION\tName: %s\tState: %s\tEvents: [%s]\tError Type: %s'
		agentName, state, subscriptions, errorType := elems[0], elems[1], elems[2], elems[3]
		p.LogExtensionInitEvent(agentName, state, subscriptions, errorType)
	}

	if strings.HasPrefix(logStr, "EXTENSION") && !strings.Contains(logStr, "Error Type") {
		// format: 'EXTENSION\tName: %s\tState: %s\tEvents: [%s]'
		agentName, state, subscriptions, errorType := elems[0], elems[1], elems[2], ""
		p.LogExtensionInitEvent(agentName, state, subscriptions, errorType)
	}
}

func (p *EventLog) Write(logline []byte) (int, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	logStr := string(logline)
	p.Logs = append(p.Logs, logStr)

	p.dispatchLogEvent(logStr)

	return len(logline), nil
}

func NewEventLog() *EventLog {
	return &EventLog{}
}
