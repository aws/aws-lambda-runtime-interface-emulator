// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"fmt"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type InitPhase string

type InitStartData struct {
	InitializationType string      `json:"initializationType"`
	RuntimeVersion     string      `json:"runtimeVersion"`
	RuntimeVersionArn  string      `json:"runtimeVersionArn"`
	FunctionName       string      `json:"functionName"`
	FunctionVersion    string      `json:"functionVersion"`
	InstanceID         string      `json:"instanceId"`
	InstanceMaxMemory  uint64      `json:"instanceMaxMemory"`
	Phase              InitPhase   `json:"phase"`
	Tracing            *TracingCtx `json:"tracing,omitempty"`
}

func (d *InitStartData) String() string {
	return fmt.Sprintf("INIT START(initType: %s, phase: %s)", d.InitializationType, d.Phase)
}

type InitRuntimeDoneData struct {
	InitializationType string         `json:"initializationType"`
	Status             ResponseStatus `json:"status"`
	Phase              InitPhase      `json:"phase"`
	ErrorType          *string        `json:"errorType,omitempty"`
	Tracing            *TracingCtx    `json:"tracing,omitempty"`
}

func (d *InitRuntimeDoneData) String() string {
	errorType := "nil"
	if d.ErrorType != nil {
		errorType = *d.ErrorType
	}
	return fmt.Sprintf("INIT RTDONE(initType: %s, status: %s, phase: %s, errorType: %s)", d.InitializationType, d.Status, d.Phase, errorType)
}

type InitReportMetrics struct {
	DurationMs float64 `json:"durationMs"`
}

type InitReportData struct {
	InitializationType string            `json:"initializationType"`
	Metrics            InitReportMetrics `json:"metrics"`
	Phase              InitPhase         `json:"phase"`
	Tracing            *TracingCtx       `json:"tracing,omitempty"`
	Status             ResponseStatus    `json:"status"`
	ErrorType          *string           `json:"errorType,omitempty"`
}

func (d *InitReportData) String() string {
	errorType := "nil"
	if d.ErrorType != nil {
		errorType = *d.ErrorType
	}

	return fmt.Sprintf("INIT REPORT(initType: %s, durationMs: %.2f, status: %s, phase: %s, errorType: %s)", d.InitializationType, d.Metrics.DurationMs, d.Status, d.Phase, errorType)
}

type TracingCtx struct {
	SpanID string            `json:"spanId,omitempty"`
	Type   model.TracingType `json:"type"`
	Value  string            `json:"value"`
}

type InvokeStartData struct {
	InvokeID    InvokeID    `json:"requestId"`
	Version     string      `json:"version,omitempty"`
	FunctionARN string      `json:"functionArn,omitempty"`
	Tracing     *TracingCtx `json:"tracing,omitempty"`
}

func (d *InvokeStartData) String() string {
	return fmt.Sprintf("INVOKE START(requestId: %s)", d.InvokeID)
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
	InvokeID        InvokeID                  `json:"requestId"`
	Status          ResponseStatus            `json:"status"`
	Metrics         *RuntimeDoneInvokeMetrics `json:"metrics,omitempty"`
	Tracing         *TracingCtx               `json:"tracing,omitempty"`
	Spans           []Span                    `json:"spans,omitempty"`
	ErrorType       *string                   `json:"errorType,omitempty"`
	InternalMetrics *InvokeResponseMetrics    `json:"-"`
}

func (d *InvokeRuntimeDoneData) String() string {
	errorType := "nil"
	if d.ErrorType != nil {
		errorType = *d.ErrorType
	}
	return fmt.Sprintf("INVOKE RTDONE(status: %s, producedBytes: %d, durationMs: %.2f, spans: %d, errorType: %s)", d.Status, d.Metrics.ProducedBytes, d.Metrics.DurationMs, len(d.Spans), errorType)
}

type ExtensionInitData struct {
	AgentName     string   `json:"name"`
	State         string   `json:"state"`
	Subscriptions []string `json:"events"`
	ErrorType     string   `json:"errorType,omitempty"`
}

func (d *ExtensionInitData) String() string {
	return fmt.Sprintf("EXTENSION INIT(agentName: %s, state: %s, errorType: %s)", d.AgentName, d.State, d.ErrorType)
}

type ReportDurationMs float64

func (d ReportDurationMs) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%.3f", d)), nil
}

type ReportMetrics struct {
	DurationMs ReportDurationMs `json:"durationMs"`
}

type ReportData struct {
	InvokeID  InvokeID              `json:"requestId"`
	Status    ResponseStatus        `json:"status"`
	Metrics   ReportMetrics         `json:"metrics"`
	Tracing   *TracingCtx           `json:"tracing,omitempty"`
	Spans     []Span                `json:"spans,omitempty"`
	ErrorType *rapidmodel.ErrorType `json:"errorType,omitempty"`
}

func (d *ReportData) String() string {
	errorType := "nil"
	if d.ErrorType != nil {
		errorType = string(*d.ErrorType)
	}
	return fmt.Sprintf("REPORT(status: %s, durationMs: %.2f, errorType: %s)", d.Status, d.Metrics.DurationMs, errorType)
}

type EndData struct {
	InvokeID InvokeID `json:"requestId"`
}

func (d *EndData) String() string {
	return "END"
}

type InternalXRayErrorCauseData struct {
	InvokeID InvokeID `json:"requestId"`
	Cause    string   `json:"cause"`
}

func (d *InternalXRayErrorCauseData) String() string {
	return fmt.Sprintf("XRAY_ERROR_CAUSE(len: %d)", len(d.Cause))
}

type InvokeID = string

type FaultData struct {
	InvokeID  InvokeID
	Status    ResponseStatus
	ErrorType *rapidmodel.ErrorType
}

func (d *FaultData) RenderFluxpumpMsg() string {
	var errtype string
	if d.ErrorType != nil {
		errtype = fmt.Sprintf("\tErrorType: %s", *d.ErrorType)
	}
	return fmt.Sprintf("RequestId: %s\tStatus: %s%s\n", d.InvokeID, d.Status, errtype)
}

type ImageErrorLogData struct {
	ExecError  rapidmodel.RuntimeExecError
	ExecConfig rapidmodel.RuntimeExec
}

type EventsAPI interface {
	SendInitStart(InitStartData) error
	SendInitRuntimeDone(InitRuntimeDoneData) error
	SendInitReport(InitReportData) error
	SendExtensionInit(ExtensionInitData) error
	SendImageError(ImageErrorLogData)
	SendInternalXRayErrorCause(InternalXRayErrorCauseData) error
	SendInvokeStart(InvokeStartData) error
	SendReport(ReportData) error
	Flush()
}
