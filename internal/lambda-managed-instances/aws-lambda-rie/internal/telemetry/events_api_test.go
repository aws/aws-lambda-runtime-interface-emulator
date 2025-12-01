// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/aws-lambda-rie/internal/telemetry/internal"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func TestEventsAPI_SendInitStart(t *testing.T) {
	m := newMockRelay(t)
	defer m.AssertExpectations(t)
	api := &EventsAPI{
		eventRelay: m,
		stdout:     &bytes.Buffer{},
	}

	data := interop.InitStartData{}
	m.On("broadcast", data, internal.CategoryPlatform, internal.TypePlatformInitStart).Once()
	assert.NoError(t, api.SendInitStart(data))
}

func TestEventsAPI_SendInitRuntimeDone(t *testing.T) {
	m := newMockRelay(t)
	defer m.AssertExpectations(t)
	api := &EventsAPI{
		eventRelay: m,
		stdout:     &bytes.Buffer{},
	}

	data := interop.InitRuntimeDoneData{}
	m.On("broadcast", data, internal.CategoryPlatform, internal.TypePlatformInitRuntimeDone).Once()
	assert.NoError(t, api.SendInitRuntimeDone(data))
}

func TestEventsAPI_SendInitReport(t *testing.T) {
	m := newMockRelay(t)
	defer m.AssertExpectations(t)
	api := &EventsAPI{
		eventRelay: m,
		stdout:     &bytes.Buffer{},
	}

	data := interop.InitReportData{}
	m.On("broadcast", data, internal.CategoryPlatform, internal.TypePlatformInitReport).Once()
	assert.NoError(t, api.SendInitReport(data))
}

func TestEventsAPI_SendExtensionInit(t *testing.T) {
	m := newMockRelay(t)
	defer m.AssertExpectations(t)
	api := &EventsAPI{
		eventRelay: m,
		stdout:     &bytes.Buffer{},
	}

	data := interop.ExtensionInitData{}
	m.On("broadcast", data, internal.CategoryPlatform, internal.TypePlatformExtension).Once()
	assert.NoError(t, api.SendExtensionInit(data))
}

func TestEventsAPI_SendInvokeStart(t *testing.T) {
	m := newMockRelay(t)
	defer m.AssertExpectations(t)
	stdout := &bytes.Buffer{}
	api := &EventsAPI{
		eventRelay: m,
		stdout:     stdout,
	}

	data := interop.InvokeStartData{
		InvokeID:    "test-invoke-id-123",
		Version:     "$LATEST",
		FunctionARN: "arn:aws:lambda:us-east-1:123456789012:function:test-function",
	}
	m.On("broadcast", data, internal.CategoryPlatform, internal.TypePlatformStart).Once()
	assert.NoError(t, api.SendInvokeStart(data))

	expectedOutput := "START RequestId: test-invoke-id-123\tVersion: $LATEST\n"
	assert.Equal(t, expectedOutput, stdout.String())
}

func TestEventsAPI_SendReport(t *testing.T) {
	m := newMockRelay(t)
	defer m.AssertExpectations(t)
	stdout := &bytes.Buffer{}
	api := &EventsAPI{
		eventRelay: m,
		stdout:     stdout,
	}

	errorType := rapidmodel.ErrorRuntimeExit
	data := interop.ReportData{
		InvokeID: "test-invoke-id-123",
		Status:   "success",
		Metrics: interop.ReportMetrics{
			DurationMs: 125.456,
		},
		ErrorType: &errorType,
	}
	m.On("broadcast", data, internal.CategoryPlatform, internal.TypePlatformReport).Once()
	assert.NoError(t, api.SendReport(data))

	expectedOutput := "END RequestId: test-invoke-id-123\nREPORT RequestId: test-invoke-id-123\tDuration: 125.46 ms\n"
	assert.Equal(t, expectedOutput, stdout.String())
}

func TestEventsAPI_SendPlatformLogsDropped(t *testing.T) {
	m := newMockRelay(t)
	defer m.AssertExpectations(t)
	api := &EventsAPI{
		eventRelay: m,
		stdout:     &bytes.Buffer{},
	}

	droppedBytes := 1024
	droppedRecords := 5
	reason := "buffer overflow"

	expectedRecord := map[string]any{
		"droppedBytes":   droppedBytes,
		"droppedRecords": droppedRecords,
		"reason":         reason,
	}

	m.On("broadcast", expectedRecord, internal.CategoryPlatform, internal.TypePlatformLogsDropped).Once()
	assert.NoError(t, api.SendPlatformLogsDropped(droppedBytes, droppedRecords, reason))
}

func TestEventsAPI_sendTelemetrySubscription(t *testing.T) {
	m := newMockRelay(t)
	defer m.AssertExpectations(t)
	api := &EventsAPI{
		eventRelay: m,
		stdout:     &bytes.Buffer{},
	}

	agentName := "test-agent"
	state := "subscribed"
	types := []internal.EventCategory{internal.CategoryPlatform, internal.CategoryFunction}

	expectedRecord := map[string]any{
		"name":  agentName,
		"state": state,
		"types": types,
	}

	m.On("broadcast", expectedRecord, internal.CategoryPlatform, internal.TypePlatformTelemetrySubscription).Once()
	assert.NoError(t, api.sendTelemetrySubscription(agentName, state, types))
}
