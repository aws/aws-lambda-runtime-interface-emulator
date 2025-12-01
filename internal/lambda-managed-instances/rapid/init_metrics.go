// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"fmt"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/servicelogs"
)

const (
	InitTimeoutProperty = "InitTimeoutSeconds"

	runtimeDurationMetric = "RuntimeDuration"
	runDurationMetric     = "RunDuration"
)

type initMetrics struct {
	logger servicelogs.Logger

	error model.AppError

	timeGetRequest time.Time

	timeStartRequest      time.Time
	timeStartedRuntime    time.Time
	timeRuntimeDone       time.Time
	timeCustomerPhaseDone time.Time
	timeInitDone          time.Time

	internalExtensionCount int
	externalExtensionCount int

	logsAPIMetrics interop.TelemetrySubscriptionMetrics

	getCurrentTime func() time.Time
}

func NewInitMetrics(logger servicelogs.Logger) *initMetrics {
	return &initMetrics{
		getCurrentTime: time.Now,
		logger:         logger,
	}
}

func (m *initMetrics) TriggerGetRequest() {
	m.timeGetRequest = m.getCurrentTime()
}

func (m *initMetrics) SetLogsAPIMetrics(metrics interop.TelemetrySubscriptionMetrics) {
	m.logsAPIMetrics = metrics
}

func (m *initMetrics) SetExtensionsNumber(internal, external int) {
	m.internalExtensionCount = internal
	m.externalExtensionCount = external
}

func (m *initMetrics) TriggerStartRequest() {
	m.timeStartRequest = m.getCurrentTime()
}

func (m *initMetrics) TriggerStartingRuntime() {
	m.timeStartedRuntime = m.getCurrentTime()
}

func (m *initMetrics) TriggerRuntimeDone() {
	m.timeRuntimeDone = m.getCurrentTime()
}

func (m *initMetrics) TriggerInitCustomerPhaseDone() {
	m.timeCustomerPhaseDone = m.getCurrentTime()
}

func (m *initMetrics) TriggerInitDone(err model.AppError) {
	m.error = err
	m.timeInitDone = m.getCurrentTime()
}

func (m *initMetrics) RunDuration() time.Duration {
	if m.timeCustomerPhaseDone.IsZero() {
		return time.Duration(0)
	}

	return m.timeCustomerPhaseDone.Sub(m.timeStartRequest)
}

func (m *initMetrics) SendMetrics() error {
	metrics := m.buildMetrics()

	m.logger.Log(servicelogs.InitOp, m.timeGetRequest, nil, nil, metrics)
	return nil
}

func (m *initMetrics) buildMetrics() []servicelogs.Metric {
	totalDuration := m.timeInitDone.Sub(m.timeGetRequest)
	runDuration := m.RunDuration()
	overheadDuration := totalDuration - runDuration

	metrics := []servicelogs.Metric{
		servicelogs.Timer(interop.TotalDurationMetric, totalDuration),
		servicelogs.Timer(interop.PlatformOverheadDurationMetric, overheadDuration),
		servicelogs.Counter(interop.InternalExtensionsCountMetric, float64(m.internalExtensionCount)),
		servicelogs.Counter(interop.ExternalExtensionsCountMetric, float64(m.externalExtensionCount)),
		servicelogs.Counter(interop.TotalExtensionsCountMetric, float64(m.internalExtensionCount+m.externalExtensionCount)),
	}

	if !m.timeRuntimeDone.IsZero() {
		metrics = append(metrics,
			servicelogs.Timer(runtimeDurationMetric, m.timeRuntimeDone.Sub(m.timeStartedRuntime)),
		)
	}

	if runDuration > 0 {
		metrics = append(metrics,
			servicelogs.Timer(runDurationMetric, runDuration),
		)
	}

	var clientErrCnt, customerErrCnt, platformErrCnt, nonCustomerErrCnt float64

	switch m.error.(type) {
	case model.ClientError:
		clientErrCnt = 1
		nonCustomerErrCnt = 1
		metrics = append(metrics,
			servicelogs.Counter(fmt.Sprintf(interop.ClientErrorReasonTemplate, m.error.ErrorType()), 1.0),
		)
	case model.CustomerError:
		customerErrCnt = 1
		metrics = append(metrics,
			servicelogs.Counter(fmt.Sprintf(interop.CustomerErrorReasonTemplate, m.error.ErrorType()), 1.0),
		)
	case model.PlatformError:
		platformErrCnt = 1
		nonCustomerErrCnt = 1
		metrics = append(metrics,
			servicelogs.Counter(fmt.Sprintf(interop.PlatformErrorReasonTemplate, m.error.ErrorType()), 1.0),
		)
	}

	metrics = append(metrics,
		servicelogs.Counter(interop.ClientErrorMetric, clientErrCnt),
		servicelogs.Counter(interop.CustomerErrorMetric, customerErrCnt),
		servicelogs.Counter(interop.PlatformErrorMetric, platformErrCnt),
		servicelogs.Counter(interop.NonCustomerErrorMetric, nonCustomerErrCnt),
	)

	for metricName, value := range m.logsAPIMetrics {
		metrics = append(metrics, servicelogs.Counter(metricName, float64(value)))
	}

	return metrics
}
