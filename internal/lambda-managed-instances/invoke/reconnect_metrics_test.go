// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/servicelogs"
)

type capturedLog struct {
	op      servicelogs.Operation
	opStart time.Time
	props   []servicelogs.Property
	dims    []servicelogs.Dimension
	metrics []servicelogs.Metric
}

type capturingLogger struct {
	logs []capturedLog
}

func (l *capturingLogger) Log(op servicelogs.Operation, opStart time.Time, props []servicelogs.Property, dims []servicelogs.Dimension, metrics []servicelogs.Metric) {
	l.logs = append(l.logs, capturedLog{op: op, opStart: opStart, props: props, dims: dims, metrics: metrics})
}
func (l *capturingLogger) Close() error { return nil }

func findMetric(metrics []servicelogs.Metric, key string) *servicelogs.Metric {
	for i := range metrics {
		if metrics[i].Key == key {
			return &metrics[i]
		}
	}
	return nil
}

func findProp(props []servicelogs.Property, name string) string {
	for _, p := range props {
		if p.Name == name {
			return p.Value
		}
	}
	return ""
}

func TestReconnectMetrics_SendMetrics_BasicFields(t *testing.T) {
	logger := &capturingLogger{}
	m := NewReconnectMetrics("invoke-1", "reconnect-1", logger)

	m.SendMetrics(interop.ReconnectOutcomeCompleted)

	assert.Len(t, logger.logs, 1)
	log := logger.logs[0]
	assert.Equal(t, servicelogs.ReconnectOp, log.op)
	assert.Equal(t, "invoke-1", findProp(log.props, interop.RequestIdProperty))
	assert.Equal(t, "reconnect-1", findProp(log.props, "reconnectId"))
	assert.NotNil(t, findMetric(log.metrics, interop.TotalDurationMetric))
	assert.Equal(t, "completed", findProp(log.props, "Outcome"))
	assert.NotNil(t, findMetric(log.metrics, "ReconnectOverhead"))

	assert.Equal(t, float64(0), findMetric(log.metrics, interop.PlatformErrorMetric).Value)
	assert.Equal(t, float64(0), findMetric(log.metrics, interop.ClientErrorMetric).Value)
}

func TestReconnectMetrics_SendMetrics_WithConnectionGap(t *testing.T) {
	logger := &capturingLogger{}
	m := NewReconnectMetrics("invoke-1", "", logger)
	past := time.Now().Add(-500 * time.Millisecond)
	m.TriggerConnectionGap(&past)

	m.SendMetrics(interop.ReconnectOutcomeCompleted)

	log := logger.logs[0]
	assert.Equal(t, "", findProp(log.props, "reconnectId"))
	gap := findMetric(log.metrics, "ConnectionGap")
	assert.NotNil(t, gap)
	assert.Greater(t, gap.Value, float64(0))
}

func TestReconnectMetrics_SendMetrics_WithResponseReplay(t *testing.T) {
	logger := &capturingLogger{}
	m := NewReconnectMetrics("invoke-1", "r-1", logger)
	m.TriggerResponseReplayStart()
	time.Sleep(10 * time.Millisecond)
	m.TriggerResponseReplayDone(4096)

	m.SendMetrics(interop.ReconnectOutcomeCompleted)

	log := logger.logs[0]
	assert.NotNil(t, findMetric(log.metrics, "ResponseReplayDuration"))
	size := findMetric(log.metrics, "ResponseReplaySizeBytes")
	assert.NotNil(t, size)
	assert.Equal(t, float64(4096), size.Value)
	speed := findMetric(log.metrics, "ResponseReplaySpeedBytesPerSec")
	assert.NotNil(t, speed)
	assert.Greater(t, speed.Value, float64(0))
}

func TestReconnectMetrics_SendMetrics_PendingAge(t *testing.T) {
	logger := &capturingLogger{}
	m := &ReconnectMetrics{
		startTime: time.Now(),
		invokeID:  "invoke-1",
		logger:    logger,

		functionDoneTime: time.Now().Add(-200 * time.Millisecond),
	}

	m.SendMetrics(interop.ReconnectOutcomeCompleted)

	log := logger.logs[0]
	pending := findMetric(log.metrics, "PendingAge")
	assert.NotNil(t, pending)
	assert.Greater(t, pending.Value, float64(0))
}

func TestReconnectMetrics_SendMetrics_TimeoutOutcome(t *testing.T) {
	logger := &capturingLogger{}
	m := NewReconnectMetrics("invoke-1", "", logger)
	m.TriggerPollStart()
	time.Sleep(1 * time.Millisecond)
	m.TriggerPollEnd()

	m.SendMetrics(interop.ReconnectOutcomeTimeout)

	log := logger.logs[0]
	assert.Equal(t, "timeout", findProp(log.props, "Outcome"))

	overhead := findMetric(log.metrics, "ReconnectOverhead")
	assert.NotNil(t, overhead)
	assert.Less(t, overhead.Value, float64(10000))
}

func TestReconnectMetrics_SendMetrics_ErrorOutcome_PlatformError(t *testing.T) {
	logger := &capturingLogger{}
	m := NewReconnectMetrics("invoke-1", "", logger)

	m.SendMetrics(interop.ReconnectOutcomeError)

	log := logger.logs[0]
	assert.Equal(t, float64(1), findMetric(log.metrics, interop.PlatformErrorMetric).Value)
	assert.Equal(t, float64(0), findMetric(log.metrics, interop.ClientErrorMetric).Value)
}

func TestReconnectMetrics_SendMetrics_NotFoundOutcome_ClientError(t *testing.T) {
	logger := &capturingLogger{}
	m := NewReconnectMetrics("invoke-1", "", logger)

	m.SendMetrics(interop.ReconnectOutcomeNotFound)

	log := logger.logs[0]
	assert.Equal(t, float64(0), findMetric(log.metrics, interop.PlatformErrorMetric).Value)
	assert.Equal(t, float64(1), findMetric(log.metrics, interop.ClientErrorMetric).Value)
}

func TestSendInvokePendingMetrics(t *testing.T) {
	logger := &capturingLogger{}
	invokeStart := time.Now().Add(-10 * time.Second)

	SendInvokePendingMetrics(logger, "invoke-1", invokeStart)

	assert.Len(t, logger.logs, 1)
	log := logger.logs[0]
	assert.Equal(t, servicelogs.InvokePendingOp, log.op)
	assert.Equal(t, "invoke-1", findProp(log.props, interop.RequestIdProperty))
	dur := findMetric(log.metrics, interop.TotalDurationMetric)
	assert.NotNil(t, dur)
	assert.Greater(t, dur.Value, float64(0))
}
