// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.amzn.com/lambda/rapi/model"
)

const requestID RequestID = "REQUEST_ID"

func TestJsonMarshalInvokeRuntimeDone(t *testing.T) {
	data := InvokeRuntimeDoneData{
		RequestID: requestID,
		Status:    "success",
		Metrics: &RuntimeDoneInvokeMetrics{
			ProducedBytes: int64(100),
			DurationMs:    float64(52.56),
		},
		Spans: []Span{
			{
				Name:       "responseLatency",
				Start:      "2022-04-11T15:01:28.543Z",
				DurationMs: float64(23.02),
			},
			{
				Name:       "responseDuration",
				Start:      "2022-04-11T15:00:00.000Z",
				DurationMs: float64(20),
			},
		},
		Tracing: &TracingCtx{
			SpanID: "spanid",
			Type:   model.XRayTracingType,
			Value:  "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1",
		},
	}

	expected := `
		{
			"requestId": "REQUEST_ID",
			"status": "success",
			"tracing": {
				"spanId": "spanid",
				"type": "X-Amzn-Trace-Id",
				"value": "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1"
			},
			"spans": [
				{
					"name": "responseLatency",
					"start": "2022-04-11T15:01:28.543Z",
					"durationMs": 23.02
				},
				{
					"name": "responseDuration",
					"start": "2022-04-11T15:00:00.000Z",
					"durationMs": 20
				}
			],
			"metrics": {
				"producedBytes": 100,
				"durationMs": 52.56
			}
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalInvokeRuntimeDoneNoTracing(t *testing.T) {
	data := InvokeRuntimeDoneData{
		RequestID: requestID,
		Status:    "success",
		Metrics: &RuntimeDoneInvokeMetrics{
			ProducedBytes: int64(100),
			DurationMs:    float64(52.56),
		},
		Spans: []Span{
			{
				Name:       "responseLatency",
				Start:      "2022-04-11T15:01:28.543Z",
				DurationMs: float64(23.02),
			},
			{
				Name:       "responseDuration",
				Start:      "2022-04-11T15:00:00.000Z",
				DurationMs: float64(20),
			},
		},
	}

	expected := `
		{
			"requestId": "REQUEST_ID",
			"status": "success",
			"spans": [
				{
					"name": "responseLatency",
					"start": "2022-04-11T15:01:28.543Z",
					"durationMs": 23.02
				},
				{
					"name": "responseDuration",
					"start": "2022-04-11T15:00:00.000Z",
					"durationMs": 20
				}
			],
			"metrics": {
				"producedBytes": 100,
				"durationMs": 52.56
			}
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalInvokeRuntimeDoneNoMetrics(t *testing.T) {
	data := InvokeRuntimeDoneData{
		RequestID: requestID,
		Status:    "success",
		Spans: []Span{
			{
				Name:       "responseLatency",
				Start:      "2022-04-11T15:01:28.543Z",
				DurationMs: float64(23.02),
			},
			{
				Name:       "responseDuration",
				Start:      "2022-04-11T15:00:00.000Z",
				DurationMs: float64(20),
			},
		},
		Tracing: &TracingCtx{
			SpanID: "spanid",
			Type:   model.XRayTracingType,
			Value:  "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1",
		},
	}

	expected := `
		{
			"requestId": "REQUEST_ID",
			"status": "success",
			"tracing": {
				"spanId": "spanid",
				"type": "X-Amzn-Trace-Id",
				"value": "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1"
			},
			"spans": [
				{
					"name": "responseLatency",
					"start": "2022-04-11T15:01:28.543Z",
					"durationMs": 23.02
				},
				{
					"name": "responseDuration",
					"start": "2022-04-11T15:00:00.000Z",
					"durationMs": 20
				}
			]
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalInvokeRuntimeDoneWithProducedBytesEqualToZero(t *testing.T) {
	data := InvokeRuntimeDoneData{
		RequestID: requestID,
		Status:    "success",
		Metrics: &RuntimeDoneInvokeMetrics{
			ProducedBytes: int64(0),
			DurationMs:    float64(52.56),
		},
		Spans: []Span{
			{
				Name:       "responseLatency",
				Start:      "2022-04-11T15:01:28.543Z",
				DurationMs: float64(23.02),
			},
			{
				Name:       "responseDuration",
				Start:      "2022-04-11T15:00:00.000Z",
				DurationMs: float64(20),
			},
		},
		Tracing: &TracingCtx{
			SpanID: "spanid",
			Type:   model.XRayTracingType,
			Value:  "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1",
		},
	}

	expected := `
		{
			"requestId": "REQUEST_ID",
			"status": "success",
			"tracing": {
				"spanId": "spanid",
				"type": "X-Amzn-Trace-Id",
				"value": "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1"
			},
			"spans": [
				{
					"name": "responseLatency",
					"start": "2022-04-11T15:01:28.543Z",
					"durationMs": 23.02
				},
				{
					"name": "responseDuration",
					"start": "2022-04-11T15:00:00.000Z",
					"durationMs": 20
				}
			],
			"metrics": {
				"producedBytes": 0,
				"durationMs": 52.56
			}
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalInvokeRuntimeDoneWithNoSpans(t *testing.T) {
	data := InvokeRuntimeDoneData{
		RequestID: requestID,
		Status:    "success",
		Metrics: &RuntimeDoneInvokeMetrics{
			ProducedBytes: int64(100),
			DurationMs:    float64(52.56),
		},
		Spans: []Span{},
		Tracing: &TracingCtx{
			SpanID: "spanid",
			Type:   model.XRayTracingType,
			Value:  "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1",
		},
	}

	expected := `
		{
			"requestId": "REQUEST_ID",
			"status": "success",
			"tracing": {
				"spanId": "spanid",
				"type": "X-Amzn-Trace-Id",
				"value": "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1"
			},
			"metrics": {
				"producedBytes": 100,
				"durationMs": 52.56
			}
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalInvokeRuntimeDoneTimeout(t *testing.T) {
	data := InvokeRuntimeDoneData{
		RequestID: requestID,
		Status:    "timeout",
		Metrics: &RuntimeDoneInvokeMetrics{
			DurationMs: float64(52.56),
		},
		Spans: []Span{},
		Tracing: &TracingCtx{
			SpanID: "spanid",
			Type:   model.XRayTracingType,
			Value:  "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1",
		},
	}

	expected := `
		{
			"requestId": "REQUEST_ID",
			"status": "timeout",
			"tracing": {
				"spanId": "spanid",
				"type": "X-Amzn-Trace-Id",
				"value": "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1"
			},
			"metrics": {
				"producedBytes": 0,
				"durationMs": 52.56
			}
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalInvokeRuntimeDoneFailure(t *testing.T) {
	errorType := "Runtime.ExitError"
	data := InvokeRuntimeDoneData{
		RequestID: requestID,
		Status:    "failure",
		ErrorType: &errorType,
		Metrics: &RuntimeDoneInvokeMetrics{
			DurationMs: float64(52.56),
		},
		Spans: []Span{},
		Tracing: &TracingCtx{
			SpanID: "spanid",
			Type:   model.XRayTracingType,
			Value:  "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1",
		},
	}

	expected := `
		{
			"requestId": "REQUEST_ID",
			"status": "failure",
			"tracing": {
				"spanId": "spanid",
				"type": "X-Amzn-Trace-Id",
				"value": "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1"
			},
			"metrics": {
				"producedBytes": 0,
				"durationMs": 52.56
			},
			"errorType": "Runtime.ExitError"
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalInvokeRuntimeDoneWithEmptyErrorType(t *testing.T) {
	errorType := ""
	data := InvokeRuntimeDoneData{
		RequestID: requestID,
		Status:    "failure",
		ErrorType: &errorType,
		Metrics: &RuntimeDoneInvokeMetrics{
			DurationMs: float64(52.56),
		},
		Spans: []Span{},
		Tracing: &TracingCtx{
			SpanID: "spanid",
			Type:   model.XRayTracingType,
			Value:  "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1",
		},
	}

	expected := `
		{
			"requestId": "REQUEST_ID",
			"status": "failure",
			"tracing": {
				"spanId": "spanid",
				"type": "X-Amzn-Trace-Id",
				"value": "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1"
			},
			"metrics": {
				"producedBytes": 0,
				"durationMs": 52.56
			},
			"errorType": ""
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalInitRuntimeDoneSuccess(t *testing.T) {
	var errorType *string
	data := InitRuntimeDoneData{
		InitializationType: "snap-start",
		Phase:              "init",
		Status:             "success",
		ErrorType:          errorType,
	}

	expected := `
		{
			"initializationType": "snap-start",
			"phase": "init",
			"status": "success"
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalInitRuntimeDoneError(t *testing.T) {
	errorType := "Runtime.ExitError"
	data := InitRuntimeDoneData{
		InitializationType: "snap-start",
		Phase:              "init",
		Status:             "error",
		ErrorType:          &errorType,
	}

	expected := `
		{
			"initializationType": "snap-start",
			"phase": "init",
			"status": "error",
			"errorType": "Runtime.ExitError"
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalInitRuntimeDoneFailureWithEmptyErrorType(t *testing.T) {
	errorType := ""
	data := InitRuntimeDoneData{
		InitializationType: "snap-start",
		Phase:              "init",
		Status:             "error",
		ErrorType:          &errorType,
	}

	expected := `
		{
			"initializationType": "snap-start",
			"phase": "init",
			"status": "error",
			"errorType": ""
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalRestoreRuntimeDoneSuccess(t *testing.T) {
	var errorType *string
	data := RestoreRuntimeDoneData{
		Status:    "success",
		ErrorType: errorType,
	}

	expected := `
		{
			"status": "success"
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalRestoreRuntimeDoneError(t *testing.T) {
	errorType := "Runtime.ExitError"
	data := RestoreRuntimeDoneData{
		Status:    "error",
		ErrorType: &errorType,
	}

	expected := `
		{
			"status": "error",
			"errorType": "Runtime.ExitError"
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalRestoreRuntimeDoneErrorWithEmptyErrorType(t *testing.T) {
	errorType := ""
	data := RestoreRuntimeDoneData{
		Status:    "error",
		ErrorType: &errorType,
	}

	expected := `
		{
			"status": "error",
			"errorType": ""
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalExtensionInit(t *testing.T) {
	data := ExtensionInitData{
		AgentName:     "agentName",
		State:         "Registered",
		ErrorType:     "",
		Subscriptions: []string{"INVOKE", "SHUTDOWN"},
	}

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"name":"agentName","state":"Registered","events":["INVOKE","SHUTDOWN"]}`, string(actual))
}

func TestJsonMarshalExtensionInitWithError(t *testing.T) {
	data := ExtensionInitData{
		AgentName:     "agentName",
		State:         "Registered",
		ErrorType:     "Extension.FooBar",
		Subscriptions: []string{"INVOKE", "SHUTDOWN"},
	}

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"name":"agentName","state":"Registered","events":["INVOKE","SHUTDOWN"],"errorType":"Extension.FooBar"}`, string(actual))
}

func TestJsonMarshalExtensionInitEmptyEvents(t *testing.T) {
	data := ExtensionInitData{
		AgentName:     "agentName",
		State:         "Registered",
		ErrorType:     "Extension.FooBar",
		Subscriptions: []string{},
	}

	actual, err := json.Marshal(data)
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"agentName","state":"Registered","events":[],"errorType":"Extension.FooBar"}`, string(actual))
}

func TestJsonMarshalReportWithTracing(t *testing.T) {
	errorType := "Runtime.ExitError"
	data := ReportData{
		RequestID: requestID,
		Status:    "error",
		ErrorType: &errorType,
		Metrics: ReportMetrics{
			DurationMs:       float64(52.56),
			BilledDurationMs: float64(52.40),
			MemorySizeMB:     uint64(1024),
			MaxMemoryUsedMB:  uint64(512),
		},
		Tracing: &TracingCtx{
			SpanID: "spanid",
			Type:   model.XRayTracingType,
			Value:  "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1",
		},
	}

	expected := `
		{
			"requestId": "REQUEST_ID",
			"status": "error",
			"errorType": "Runtime.ExitError",
			"metrics": {
				"durationMs": 52.56,
				"billedDurationMs": 52.40,
				"memorySizeMB": 1024,
				"maxMemoryUsedMB": 512
			},
			"tracing": {
				"spanId": "spanid",
				"type": "X-Amzn-Trace-Id",
				"value": "Root=1-5759e988-bd862e3fe1be46a994272793;Parent=53995c3f42cd8ad8;Sampled=1"
			}
		}
	`

	actual, err := json.Marshal(data)
	require.NoError(t, err)
	require.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalReportWithoutErrorSpansAndTracing(t *testing.T) {
	data := ReportData{
		RequestID: requestID,
		Status:    "timeout",
		Metrics: ReportMetrics{
			DurationMs:       float64(52.56),
			BilledDurationMs: float64(52.40),
			MemorySizeMB:     uint64(1024),
			MaxMemoryUsedMB:  uint64(512),
		},
	}

	expected := `
		{
			"requestId": "REQUEST_ID",
			"status": "timeout",
			"metrics": {
				"durationMs": 52.56,
				"billedDurationMs": 52.40,
				"memorySizeMB": 1024,
				"maxMemoryUsedMB": 512
			}
		}
	`

	actual, err := json.Marshal(data)
	require.NoError(t, err)
	require.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalReportWithInit(t *testing.T) {
	data := ReportData{
		RequestID: requestID,
		Status:    "success",
		Metrics: ReportMetrics{
			DurationMs:       float64(52.56),
			BilledDurationMs: float64(52.40),
			MemorySizeMB:     uint64(1024),
			MaxMemoryUsedMB:  uint64(512),
			InitDurationMs:   float64(3.15),
		},
	}

	expected := `
		{
			"requestId": "REQUEST_ID",
			"status": "success",
			"metrics": {
				"durationMs": 52.56,
				"billedDurationMs": 52.40,
				"memorySizeMB": 1024,
				"maxMemoryUsedMB": 512,
				"initDurationMs": 3.15
			}
		}
	`

	actual, err := json.Marshal(data)
	require.NoError(t, err)
	require.JSONEq(t, expected, string(actual))
}
