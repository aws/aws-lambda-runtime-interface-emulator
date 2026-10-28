// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/servicelogs"
)

type ReconnectMetrics struct {
	startTime              time.Time
	invokeID               interop.InvokeID
	reconnectRequestID     string
	logger                 servicelogs.Logger
	functionDoneTime       time.Time
	connectionGap          time.Duration
	responseReplayStart    time.Time
	responseReplayDuration time.Duration
	responseReplaySize     int
	pollStartTime          time.Time
	pollEndTime            time.Time
}

func NewReconnectMetrics(invokeID interop.InvokeID, reconnectID string, logger servicelogs.Logger) *ReconnectMetrics {
	return &ReconnectMetrics{startTime: time.Now(), invokeID: invokeID, reconnectRequestID: reconnectID, logger: logger}
}

func (m *ReconnectMetrics) TriggerConnectionGap(lastDisconnectTime *time.Time) {
	if lastDisconnectTime != nil {
		m.connectionGap = time.Since(*lastDisconnectTime)
	}
}

func (m *ReconnectMetrics) TriggerResponseReplayStart() {
	m.responseReplayStart = time.Now()
}

func (m *ReconnectMetrics) TriggerResponseReplayDone(size int) {
	m.responseReplayDuration = time.Since(m.responseReplayStart)
	m.responseReplaySize = size
}

func (m *ReconnectMetrics) SetFunctionDoneTime(t time.Time) { m.functionDoneTime = t }

func (m *ReconnectMetrics) TriggerPollStart() { m.pollStartTime = time.Now() }

func (m *ReconnectMetrics) TriggerPollEnd() { m.pollEndTime = time.Now() }

type noopReconnectMetrics struct{}

func (noopReconnectMetrics) TriggerConnectionGap(*time.Time) {}
func (noopReconnectMetrics) TriggerPollStart()               {}
func (noopReconnectMetrics) TriggerPollEnd()                 {}
func (noopReconnectMetrics) TriggerResponseReplayStart()     {}
func (noopReconnectMetrics) TriggerResponseReplayDone(int)   {}
func (noopReconnectMetrics) SetFunctionDoneTime(time.Time)   {}

func NoopReconnectMetrics() interop.ReconnectMetrics { return noopReconnectMetrics{} }

func (m *ReconnectMetrics) SendMetrics(outcome interop.ReconnectOutcome) {
	totalDuration := time.Since(m.startTime)

	props := []servicelogs.Property{
		{Name: interop.RequestIdProperty, Value: string(m.invokeID)},
		{Name: "Outcome", Value: string(outcome)},
	}
	if m.reconnectRequestID != "" {
		props = append(props, servicelogs.Property{Name: "reconnectId", Value: m.reconnectRequestID})
	}

	metrics := []servicelogs.Metric{
		servicelogs.Timer(interop.TotalDurationMetric, totalDuration),
	}
	if m.connectionGap > 0 {
		metrics = append(metrics, servicelogs.Timer("ConnectionGap", m.connectionGap))
	}

	if !m.functionDoneTime.IsZero() {
		pendingAge := m.startTime.Sub(m.functionDoneTime)
		if pendingAge > 0 {
			metrics = append(metrics, servicelogs.Timer("PendingAge", pendingAge))
		}
	}
	if m.responseReplayDuration > 0 {
		metrics = append(metrics, servicelogs.Timer("ResponseReplayDuration", m.responseReplayDuration))
		metrics = append(metrics, servicelogs.Counter("ResponseReplaySizeBytes", float64(m.responseReplaySize)))
		bytesPerSec := float64(m.responseReplaySize) / m.responseReplayDuration.Seconds()
		metrics = append(metrics, servicelogs.Counter("ResponseReplaySpeedBytesPerSec", bytesPerSec))
	}

	var overhead time.Duration
	if !m.pollStartTime.IsZero() && !m.pollEndTime.IsZero() {
		pollDuration := m.pollEndTime.Sub(m.pollStartTime)
		overhead = totalDuration - pollDuration
	} else {

		overhead = totalDuration
	}
	metrics = append(metrics, servicelogs.Timer("ReconnectOverhead", overhead))

	var platformErrCnt, clientErrCnt float64
	switch outcome {
	case interop.ReconnectOutcomeError:
		platformErrCnt = 1
	case interop.ReconnectOutcomeNotFound:
		clientErrCnt = 1
	}
	metrics = append(metrics,
		servicelogs.Counter(interop.PlatformErrorMetric, platformErrCnt),
		servicelogs.Counter(interop.ClientErrorMetric, clientErrCnt),
	)

	m.logger.Log(servicelogs.ReconnectOp, m.startTime, props, nil, metrics)
}

func SendInvokePendingMetrics(logger servicelogs.Logger, invokeID interop.InvokeID, invokeStart time.Time) {
	logger.Log(servicelogs.InvokePendingOp, invokeStart,
		[]servicelogs.Property{{Name: interop.RequestIdProperty, Value: string(invokeID)}},
		nil,
		[]servicelogs.Metric{servicelogs.Timer(interop.TotalDurationMetric, time.Since(invokeStart))},
	)
}
