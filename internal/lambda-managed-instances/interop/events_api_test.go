// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

const (
	initializationType          = "lambda-managed-instances"
	invokeID           InvokeID = "REQUEST_ID"
)

func TestJsonMarshalInvokeRuntimeDone(t *testing.T) {
	data := InvokeRuntimeDoneData{
		InvokeID: invokeID,
		Status:   "success",
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
		InvokeID: invokeID,
		Status:   "success",
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
		InvokeID: invokeID,
		Status:   "success",
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
		InvokeID: invokeID,
		Status:   "success",
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
		InvokeID: invokeID,
		Status:   "success",
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
		InvokeID: invokeID,
		Status:   "timeout",
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
		InvokeID:  invokeID,
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
		InvokeID:  invokeID,
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
		InitializationType: initializationType,
		Phase:              "init",
		Status:             "success",
		ErrorType:          errorType,
	}

	expected := `
		{
			"initializationType": "lambda-managed-instances",
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
		InitializationType: initializationType,
		Phase:              "init",
		Status:             "error",
		ErrorType:          &errorType,
	}

	expected := `
		{
			"initializationType": "lambda-managed-instances",
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
		InitializationType: initializationType,
		Phase:              "init",
		Status:             "error",
		ErrorType:          &errorType,
	}

	expected := `
		{
			"initializationType": "lambda-managed-instances",
			"phase": "init",
			"status": "error",
			"errorType": ""
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalInitReportSuccess(t *testing.T) {
	var errorType *string
	data := InitReportData{
		InitializationType: initializationType,
		Phase:              "init",
		Status:             "success",
		ErrorType:          errorType,
		Metrics:            InitReportMetrics{DurationMs: 5},
	}

	expected := `
		{
			"initializationType": "lambda-managed-instances",
			"metrics": {"durationMs": 5},
			"phase": "init",
			"status": "success"
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)

	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalInitReportError(t *testing.T) {
	errorType := "Runtime.ExitError"
	data := InitReportData{
		InitializationType: initializationType,
		Phase:              "init",
		Status:             "error",
		ErrorType:          &errorType,
		Metrics:            InitReportMetrics{DurationMs: 18},
	}

	expected := `
		{
			"initializationType": "lambda-managed-instances",
			"metrics": {"durationMs": 18},
			"phase": "init",
			"status": "error",
			"errorType": "Runtime.ExitError"
		}
	`

	actual, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalInitReportTimeout(t *testing.T) {
	var errorType *string

	data := InitReportData{
		InitializationType: initializationType,
		Phase:              "init",
		Status:             "timeout",
		ErrorType:          errorType,
		Metrics:            InitReportMetrics{DurationMs: 17},
	}

	expected := `
		{
			"initializationType": "lambda-managed-instances",
			"metrics": {"durationMs": 17},
			"phase": "init",
			"status": "timeout"
		}
	`

	actual, err := json.Marshal(data)

	t.Log(string(actual))

	assert.NoError(t, err)
	assert.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalInitReportErrorWithEmptyErrorType(t *testing.T) {
	errorType := ""
	data := InitReportData{
		InitializationType: initializationType,
		Phase:              "init",
		Status:             "error",
		ErrorType:          &errorType,
		Metrics:            InitReportMetrics{DurationMs: 23},
	}

	expected := `
		{
			"initializationType": "lambda-managed-instances",
			"metrics": {"durationMs": 23},
			"phase": "init",
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
	errorType := rapidmodel.ErrorRuntimeExit
	data := ReportData{
		InvokeID:  invokeID,
		Status:    "error",
		ErrorType: &errorType,
		Metrics: ReportMetrics{
			DurationMs: 52.56,
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
				"durationMs": 52.56
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
		InvokeID: invokeID,
		Status:   "timeout",
		Metrics: ReportMetrics{
			DurationMs: 52.56,
		},
	}

	expected := `
		{
			"requestId": "REQUEST_ID",
			"status": "timeout",
			"metrics": {
				"durationMs": 52.56
			}
		}
	`

	actual, err := json.Marshal(data)
	require.NoError(t, err)
	require.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalReportWithInit(t *testing.T) {
	data := ReportData{
		InvokeID: invokeID,
		Status:   "success",
		Metrics: ReportMetrics{
			DurationMs: 52.56,
		},
	}

	expected := `
		{
			"requestId": "REQUEST_ID",
			"status": "success",
			"metrics": {
				"durationMs": 52.56
			}
		}
	`

	actual, err := json.Marshal(data)
	require.NoError(t, err)
	require.JSONEq(t, expected, string(actual))
}

func TestJsonMarshalReportMetrics(t *testing.T) {
	testCases := []struct {
		name     string
		actual   ReportData
		expected string
	}{
		{
			"Report metrics with lower precision than reqd.",
			ReportData{
				InvokeID: invokeID,
				Status:   "success",
				Metrics: ReportMetrics{
					DurationMs: 12.3,
				},
			},
			`{"requestId":"REQUEST_ID","status":"success","metrics":{"durationMs":12.300}}`,
		},
		{
			"Report metrics with enough or higher precision than reqd.",
			ReportData{
				InvokeID: invokeID,
				Status:   "success",
				Metrics: ReportMetrics{
					DurationMs: 1.234567,
				},
			},
			`{"requestId":"REQUEST_ID","status":"success","metrics":{"durationMs":1.235}}`,
		},
		{
			"`DurationMs` of integer type, `InitDuration` absent",
			ReportData{
				InvokeID: invokeID,
				Status:   "success",
				Metrics: ReportMetrics{
					DurationMs: 10,
				},
			},
			`{"requestId":"REQUEST_ID","status":"success","metrics":{"durationMs":10.000}}`,
		},
		{
			"Report metrics with zero value",
			ReportData{
				InvokeID: invokeID,
				Status:   "success",
				Metrics: ReportMetrics{
					DurationMs: 0,
				},
			},
			`{"requestId":"REQUEST_ID","status":"success","metrics":{"durationMs":0.000}}`,
		},
		{
			"Report metrics not explicitly provided",
			ReportData{
				InvokeID: invokeID,
				Status:   "success",
				Metrics:  ReportMetrics{},
			},
			`{"requestId":"REQUEST_ID","status":"success","metrics":{"durationMs":0.000}}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := json.Marshal(tc.actual)
			require.NoError(t, err)
			require.Equal(t, tc.expected, string(actual))
		})
	}
}

func TestFaultDataRenderFluxpumpMsg(t *testing.T) {
	getPointerFromErrorType := func(e rapidmodel.ErrorType) *rapidmodel.ErrorType {
		return &e
	}

	testCases := []struct {
		name      string
		expectLog string
		actualLog *FaultData
	}{
		{
			name:      "TimeoutDataString",
			expectLog: "RequestId: dbe05a36-624e-4924-84d9-1c196aa21733\tStatus: timeout\n",
			actualLog: &FaultData{
				"dbe05a36-624e-4924-84d9-1c196aa21733",
				Timeout,
				nil,
			},
		},
		{
			name:      "ErrorDataString",
			expectLog: "RequestId: 34359edf-cda1-4088-a74d-74f37ef686b6\tStatus: error\tErrorType: Runtime.InvalidEntrypoint\n",
			actualLog: &FaultData{
				"34359edf-cda1-4088-a74d-74f37ef686b6",
				Error,
				getPointerFromErrorType(rapidmodel.ErrorRuntimeInvalidEntryPoint),
			},
		},
		{
			name:      "FailureDataString",
			expectLog: "RequestId: ff612654-10be-4311-8576-ab5af830d402\tStatus: failure\tErrorType: Sandbox.Failure\n",
			actualLog: &FaultData{
				"ff612654-10be-4311-8576-ab5af830d402",
				Failure,
				getPointerFromErrorType(rapidmodel.ErrorSandboxFailure),
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expectLog, tt.actualLog.RenderFluxpumpMsg())
		})
	}
}

func TestTelemetryEventDataString(t *testing.T) {
	runtimeExit := rapidmodel.ErrorRuntimeExit
	runtimeUnknown := string(rapidmodel.ErrorRuntimeUnknown)

	testCases := []struct {
		name      string
		expectLog string
		actualLog fmt.Stringer
	}{
		{
			name:      "InitStartDataString",
			expectLog: "INIT START(initType: lambda-managed-instances, phase: invoke)",
			actualLog: &InitStartData{
				InitializationType: initializationType,
				Phase:              InitPhase("invoke"),
			},
		},
		{
			name:      "InitRuntimeDoneDataString",
			expectLog: "INIT RTDONE(initType: lambda-managed-instances, status: error, phase: init, errorType: Runtime.Unknown)",
			actualLog: &InitRuntimeDoneData{
				InitializationType: initializationType,
				Status:             "error",
				Phase:              InitPhase("init"),
				ErrorType:          &runtimeUnknown,
			},
		},
		{
			name:      "InitReportDataString",
			expectLog: "INIT REPORT(initType: lambda-managed-instances, durationMs: 40.00, status: error, phase: init, errorType: Runtime.Unknown)",
			actualLog: &InitReportData{
				InitializationType: initializationType,
				Metrics:            InitReportMetrics{DurationMs: 40},
				Phase:              InitPhase("init"),
				Status:             "error",
				ErrorType:          &runtimeUnknown,
			},
		},
		{
			name:      "InvokeRuntimeDoneDataString",
			expectLog: "INVOKE RTDONE(status: success, producedBytes: 100, durationMs: 52.56, spans: 0, errorType: nil)",
			actualLog: &InvokeRuntimeDoneData{
				Status: "success",
				Metrics: &RuntimeDoneInvokeMetrics{
					ProducedBytes: int64(100),
					DurationMs:    float64(52.56),
				},
				Spans: []Span{},
			},
		},
		{
			name:      "ExtensionInitDataString",
			expectLog: "EXTENSION INIT(agentName: Amazon Cloudfront, state: Registered, errorType: )",
			actualLog: &ExtensionInitData{
				AgentName: "Amazon Cloudfront",
				State:     "Registered",
			},
		},
		{
			name:      "ReportDataString",
			expectLog: "REPORT(status: error, durationMs: 27.80, errorType: Runtime.ExitError)",
			actualLog: &ReportData{
				InvokeID: "75c6a56e-385d-4686-8114-ae6fe457e397",
				Status:   "error",
				Metrics: ReportMetrics{
					DurationMs: 27.799,
				},
				ErrorType: &runtimeExit,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expectLog, tt.actualLog.String())
		})
	}
}
