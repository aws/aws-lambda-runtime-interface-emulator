// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"fmt"

	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/rapi/model"
)

type InitPhase string

// InitializationType describes possible types of INIT phase
type InitType string

type InitStartData struct {
	InitializationType InitType    `json:"initializationType"`
	RuntimeVersion     string      `json:"runtimeVersion"`
	RuntimeVersionArn  string      `json:"runtimeVersionArn"`
	FunctionName       string      `json:"functionName"`
	FunctionArn        string      `json:"functionArn"`
	FunctionVersion    string      `json:"functionVersion"`
	InstanceID         string      `json:"instanceId"`
	InstanceMaxMemory  uint64      `json:"instanceMaxMemory"`
	Phase              InitPhase   `json:"phase"`
	Tracing            *TracingCtx `json:"tracing,omitempty"`
}

func (d *InitStartData) String() string {
	return fmt.Sprintf("INIT START(type: %s, phase: %s)", d.InitializationType, d.Phase)
}

type InitRuntimeDoneData struct {
	InitializationType InitType    `json:"initializationType"`
	Status             string      `json:"status"`
	Phase              InitPhase   `json:"phase"`
	ErrorType          *string     `json:"errorType,omitempty"`
	Tracing            *TracingCtx `json:"tracing,omitempty"`
}

func (d *InitRuntimeDoneData) String() string {
	return fmt.Sprintf("INIT RTDONE(status: %s)", d.Status)
}

type InitReportMetrics struct {
	DurationMs float64 `json:"durationMs"`
}

type InitReportData struct {
	InitializationType InitType          `json:"initializationType"`
	Metrics            InitReportMetrics `json:"metrics"`
	Phase              InitPhase         `json:"phase"`
	Tracing            *TracingCtx       `json:"tracing,omitempty"`
}

func (d *InitReportData) String() string {
	return fmt.Sprintf("INIT REPORT(durationMs: %f)", d.Metrics.DurationMs)
}

type RestoreRuntimeDoneData struct {
	Status    string      `json:"status"`
	ErrorType *string     `json:"errorType,omitempty"`
	Tracing   *TracingCtx `json:"tracing,omitempty"`
}

func (d *RestoreRuntimeDoneData) String() string {
	return fmt.Sprintf("RESTORE RTDONE(status: %s)", d.Status)
}

type TracingCtx struct {
	SpanID string            `json:"spanId,omitempty"`
	Type   model.TracingType `json:"type"`
	Value  string            `json:"value"`
}

type InvokeStartData struct {
	RequestID string      `json:"requestId"`
	Version   string      `json:"version,omitempty"`
	Tracing   *TracingCtx `json:"tracing,omitempty"`
}

func (d *InvokeStartData) String() string {
	return fmt.Sprintf("INVOKE START(requestId: %s)", d.RequestID)
}

type RuntimeDoneInvokeMetrics struct {
	ProducedBytes int64   `json:"producedBytes"`
	DurationMs    float64 `json:"durationMs"`
}

type Span struct {
	Name       string  `json:"name"`
	Start      string  `json:"start"`
	DurationMs float64 `json:"durationMs"`
}

func (s *Span) String() string {
	return fmt.Sprintf("SPAN(name: %s)", s.Name)
}

type InvokeRuntimeDoneData struct {
	RequestID       RequestID                 `json:"requestId"`
	Status          string                    `json:"status"`
	Metrics         *RuntimeDoneInvokeMetrics `json:"metrics,omitempty"`
	Tracing         *TracingCtx               `json:"tracing,omitempty"`
	Spans           []Span                    `json:"spans,omitempty"`
	ErrorType       *string                   `json:"errorType,omitempty"`
	InternalMetrics *InvokeResponseMetrics    `json:"-"`
}

func (d *InvokeRuntimeDoneData) String() string {
	return fmt.Sprintf("INVOKE RTDONE(status: %s, produced bytes: %d, duration: %fms)", d.Status, d.Metrics.ProducedBytes, d.Metrics.DurationMs)
}

type ExtensionInitData struct {
	AgentName     string   `json:"name"`
	State         string   `json:"state"`
	Subscriptions []string `json:"events"`
	ErrorType     string   `json:"errorType,omitempty"`
}

func (d *ExtensionInitData) String() string {
	return fmt.Sprintf("EXTENSION INIT(agent name: %s, state: %s, error type: %s)", d.AgentName, d.State, d.ErrorType)
}

type ReportMetrics struct {
	DurationMs       float64 `json:"durationMs"`
	BilledDurationMs float64 `json:"billedDurationMs"`
	MemorySizeMB     uint64  `json:"memorySizeMB"`
	MaxMemoryUsedMB  uint64  `json:"maxMemoryUsedMB"`
	InitDurationMs   float64 `json:"initDurationMs,omitempty"`
}

type ReportData struct {
	RequestID RequestID     `json:"requestId"`
	Status    string        `json:"status"`
	Metrics   ReportMetrics `json:"metrics"`
	Tracing   *TracingCtx   `json:"tracing,omitempty"`
	Spans     []Span        `json:"spans,omitempty"`
	ErrorType *string       `json:"errorType,omitempty"`
}

func (d *ReportData) String() string {
	return fmt.Sprintf("REPORT(status: %s, durationMs: %f)", d.Status, d.Metrics.DurationMs)
}

type EndData struct {
	RequestID RequestID `json:"requestId"`
}

func (d *EndData) String() string {
	return "END"
}

type RequestID string

type FaultData struct {
	RequestID    RequestID
	ErrorMessage error
	ErrorType    fatalerror.ErrorType
}

func (d *FaultData) String() string {
	return fmt.Sprintf("RequestId: %s Error: %s\n%s\n", d.RequestID, d.ErrorMessage, d.ErrorType)
}

type ImageErrorLogData string

type EventsAPI interface {
	SetCurrentRequestID(RequestID)
	SendInitStart(InitStartData) error
	SendInitRuntimeDone(InitRuntimeDoneData) error
	SendInitReport(InitReportData) error
	SendRestoreRuntimeDone(RestoreRuntimeDoneData) error
	SendInvokeStart(InvokeStartData) error
	SendInvokeRuntimeDone(InvokeRuntimeDoneData) error
	SendExtensionInit(ExtensionInitData) error
	SendReportSpan(Span) error
	SendReport(ReportData) error
	SendEnd(EndData) error
	SendFault(FaultData) error
	SendImageErrorLog(ImageErrorLogData)

	FetchTailLogs(string) (string, error)
	GetRuntimeDoneSpans(
		runtimeStartedTime int64,
		invokeResponseMetrics *InvokeResponseMetrics,
		runtimeOverheadStartedTime int64,
		runtimeReadyTime int64,
	) []Span
}
