// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/servicelogs"
)

const (
	shutdownReasonTemplate = "ShutdownReason-%s"
)

type durationMetricTimer struct {
	metricName string
	startTime  time.Time
	metrics    *shutdownMetrics
}

type durationMetric struct {
	metricName string
	duration   time.Duration
}

func (t *durationMetricTimer) Done() {
	t.metrics.mutex.Lock()
	defer t.metrics.mutex.Unlock()

	t.metrics.durationMetrics = append(t.metrics.durationMetrics, durationMetric{
		metricName: t.metricName,
		duration:   t.metrics.getCurrentTime().Sub(t.startTime),
	})
}

type shutdownMetrics struct {
	logger servicelogs.Logger

	reason model.AppError
	error  model.AppError

	mutex sync.Mutex

	props           []servicelogs.Property
	dims            []servicelogs.Dimension
	metrics         []servicelogs.Metric
	durationMetrics []durationMetric

	startTime time.Time

	internalExtensionCount int
	externalExtensionCount int

	killProcessDurationRegex *regexp.Regexp

	getCurrentTime func() time.Time
}

func NewShutdownMetrics(logger servicelogs.Logger, reason model.AppError) *shutdownMetrics {
	return &shutdownMetrics{
		logger:         logger,
		reason:         reason,
		startTime:      time.Now(),
		getCurrentTime: time.Now,

		killProcessDurationRegex: regexp.MustCompile(`^Kill.+Duration$`),
	}
}

func (m *shutdownMetrics) CreateDurationMetric(name string) interop.DurationMetricTimer {
	return &durationMetricTimer{
		metricName: name,
		startTime:  m.getCurrentTime(),
		metrics:    m,
	}
}

func (m *shutdownMetrics) AddMetric(metric servicelogs.Metric) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.metrics = append(m.metrics, metric)
}

func (m *shutdownMetrics) SetAgentCount(internal, external int) {
	m.internalExtensionCount = internal
	m.externalExtensionCount = external
}

func (m *shutdownMetrics) SendMetrics(err model.AppError) {
	m.error = err

	m.buildMetrics()

	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logger.Log(servicelogs.ShutdownOp, m.startTime, m.props, m.dims, m.metrics)
}

func (m *shutdownMetrics) buildMetrics() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.metrics = append(m.metrics,
		servicelogs.Counter(interop.InternalExtensionsCountMetric, float64(m.internalExtensionCount)),
		servicelogs.Counter(interop.ExternalExtensionsCountMetric, float64(m.externalExtensionCount)),
		servicelogs.Counter(interop.TotalExtensionsCountMetric, float64(m.internalExtensionCount+m.externalExtensionCount)),
	)

	var totalDuration, sumCustomerDuration time.Duration

	for _, metric := range m.durationMetrics {
		switch key := metric.metricName; {
		case key == interop.TotalDurationMetric:
			totalDuration = metric.duration
		case key == interop.ShutdownRuntimeDuration, key == interop.ShutdownExtensionsDuration, key == interop.ShutdownWaitAllProcessesDuration, m.killProcessDurationRegex.MatchString(key):
			sumCustomerDuration += metric.duration
		}

		m.metrics = append(m.metrics, servicelogs.Timer(metric.metricName, metric.duration))
	}

	shutdodwnOverhead := totalDuration - sumCustomerDuration
	m.metrics = append(m.metrics, servicelogs.Timer(interop.PlatformOverheadDurationMetric, shutdodwnOverhead))

	if m.reason != nil {
		m.metrics = append(
			m.metrics,
			servicelogs.Counter(fmt.Sprintf(shutdownReasonTemplate, m.reason), 1.0),
		)
	}

	var platformErrCnt float64
	if m.error != nil {
		platformErrCnt = 1
		m.metrics = append(m.metrics,
			servicelogs.Counter(fmt.Sprintf(interop.PlatformErrorReasonTemplate, m.error.ErrorType()), 1.0),
		)
	}
	m.metrics = append(m.metrics,
		servicelogs.Counter(interop.PlatformErrorMetric, platformErrCnt),
	)
}
