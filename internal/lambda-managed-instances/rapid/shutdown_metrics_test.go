// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"cmp"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	intmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/servicelogs"
)

type shutdownMetricsMocks struct {
	timeStamp time.Time
	err       model.AppError
	initData  interop.MockInitStaticDataProvider
	logger    servicelogs.MockLogger
}

func checkShutdownMocksExpectations(t *testing.T, mocks *shutdownMetricsMocks) {
	mocks.initData.AssertExpectations(t)
	mocks.logger.AssertExpectations(t)
}

func Test_shutdownMetrics(t *testing.T) {
	tests := []struct {
		name            string
		shutdownReason  model.AppError
		metricFlow      func(m *shutdownMetrics, mocks *shutdownMetricsMocks)
		expectedMetrics []servicelogs.Metric
	}{
		{
			name: "shutdown_full_flow_no_extensions",
			metricFlow: func(m *shutdownMetrics, mocks *shutdownMetricsMocks) {
				m.SetAgentCount(0, 0)

				timer := m.CreateDurationMetric("TotalDuration")
				mocks.timeStamp = mocks.timeStamp.Add(5 * time.Second)
				timer.Done()

				timer = m.CreateDurationMetric("AbortInvokeDuration")
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				timer.Done()

				timer = m.CreateDurationMetric("KillruntimeDuration")
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				timer.Done()

				timer = m.CreateDurationMetric("WaitCustomerProcessesExitDuration")
				mocks.timeStamp = mocks.timeStamp.Add(2 * time.Second)
				timer.Done()

				timer = m.CreateDurationMetric("ShutdownRuntimeServerDuration")
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				timer.Done()
			},
			expectedMetrics: []servicelogs.Metric{
				{Type: servicelogs.TimerType, Key: "TotalDuration", Value: 5000000},
				{Type: servicelogs.TimerType, Key: "AbortInvokeDuration", Value: 1000000},
				{Type: servicelogs.TimerType, Key: "KillruntimeDuration", Value: 1000000},
				{Type: servicelogs.TimerType, Key: "WaitCustomerProcessesExitDuration", Value: 2000000},
				{Type: servicelogs.TimerType, Key: "ShutdownRuntimeServerDuration", Value: 1000000},
				{Type: servicelogs.TimerType, Key: "PlatformOverheadDuration", Value: 2000000},
				{Type: servicelogs.CounterType, Key: "TotalExtensionsCount", Value: 0},
				{Type: servicelogs.CounterType, Key: "InternalExtensionsCount", Value: 0},
				{Type: servicelogs.CounterType, Key: "ExternalExtensionsCount", Value: 0},
				{Type: servicelogs.CounterType, Key: "PlatformError", Value: 0},
			},
		},
		{
			name:           "shutdown_full_flow_with_extensions",
			shutdownReason: model.NewPlatformError(nil, model.ErrorAgentCrash),
			metricFlow: func(m *shutdownMetrics, mocks *shutdownMetricsMocks) {
				m.SetAgentCount(2, 3)

				timer := m.CreateDurationMetric("TotalDuration")
				mocks.timeStamp = mocks.timeStamp.Add(5 * time.Second)
				timer.Done()

				timer = m.CreateDurationMetric("AbortInvokeDuration")
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				timer.Done()

				timer = m.CreateDurationMetric("StopRuntimeDuration")
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				timer.Done()

				timer = m.CreateDurationMetric("WaitCustomerProcessesExitDuration")
				mocks.timeStamp = mocks.timeStamp.Add(2 * time.Second)
				timer.Done()

				timer = m.CreateDurationMetric("ShutdownRuntimeServerDuration")
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				timer.Done()
			},
			expectedMetrics: []servicelogs.Metric{
				{Type: servicelogs.TimerType, Key: "TotalDuration", Value: 5000000},
				{Type: servicelogs.TimerType, Key: "AbortInvokeDuration", Value: 1000000},
				{Type: servicelogs.TimerType, Key: "StopRuntimeDuration", Value: 1000000},
				{Type: servicelogs.TimerType, Key: "WaitCustomerProcessesExitDuration", Value: 2000000},
				{Type: servicelogs.TimerType, Key: "ShutdownRuntimeServerDuration", Value: 1000000},
				{Type: servicelogs.TimerType, Key: "PlatformOverheadDuration", Value: 2000000},
				{Type: servicelogs.CounterType, Key: "TotalExtensionsCount", Value: 5},
				{Type: servicelogs.CounterType, Key: "InternalExtensionsCount", Value: 2},
				{Type: servicelogs.CounterType, Key: "ExternalExtensionsCount", Value: 3},
				{Type: servicelogs.CounterType, Key: "ShutdownReason-Extension.Crash", Value: 1},
				{Type: servicelogs.CounterType, Key: "PlatformError", Value: 0},
			},
		},
		{
			name: "shutdown_no_init_data",
			metricFlow: func(m *shutdownMetrics, mocks *shutdownMetricsMocks) {
				timer := m.CreateDurationMetric("TotalDuration")
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				timer.Done()
			},
			expectedMetrics: []servicelogs.Metric{
				{Type: servicelogs.TimerType, Key: "TotalDuration", Value: 1000000},
				{Type: servicelogs.TimerType, Key: "PlatformOverheadDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "TotalExtensionsCount", Value: 0},
				{Type: servicelogs.CounterType, Key: "InternalExtensionsCount", Value: 0},
				{Type: servicelogs.CounterType, Key: "ExternalExtensionsCount", Value: 0},
				{Type: servicelogs.CounterType, Key: "PlatformError", Value: 0},
			},
		},
		{
			name: "shutdown_failed",
			metricFlow: func(m *shutdownMetrics, mocks *shutdownMetricsMocks) {
				m.SetAgentCount(2, 3)

				timer := m.CreateDurationMetric("TotalDuration")
				mocks.timeStamp = mocks.timeStamp.Add(2 * time.Second)
				timer.Done()

				timer = m.CreateDurationMetric("AbortInvokeDuration")
				mocks.timeStamp = mocks.timeStamp.Add(1 * time.Second)
				timer.Done()

				timer = m.CreateDurationMetric("ShutdownRuntimeServerDuration")
				mocks.timeStamp = mocks.timeStamp.Add(1 * time.Second)
				timer.Done()

				mocks.err = model.NewPlatformError(nil, model.ErrorAgentCrash)
			},
			expectedMetrics: []servicelogs.Metric{
				{Type: servicelogs.TimerType, Key: "TotalDuration", Value: 2000000},
				{Type: servicelogs.TimerType, Key: "AbortInvokeDuration", Value: 1000000},
				{Type: servicelogs.TimerType, Key: "ShutdownRuntimeServerDuration", Value: 1000000},
				{Type: servicelogs.TimerType, Key: "PlatformOverheadDuration", Value: 2000000},
				{Type: servicelogs.CounterType, Key: "TotalExtensionsCount", Value: 5},
				{Type: servicelogs.CounterType, Key: "InternalExtensionsCount", Value: 2},
				{Type: servicelogs.CounterType, Key: "ExternalExtensionsCount", Value: 3},
				{Type: servicelogs.CounterType, Key: "PlatformError", Value: 1},
				{Type: servicelogs.CounterType, Key: "PlatformErrorReason-Extension.Crash", Value: 1},
			},
		},
	}

	metricsSortFunc := func(a, b servicelogs.Metric) int {
		return cmp.Compare(a.Key, b.Key)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := &shutdownMetricsMocks{
				initData: interop.MockInitStaticDataProvider{},
				logger:   servicelogs.MockLogger{},
			}

			m := NewShutdownMetrics(&mocks.logger, tt.shutdownReason)
			m.getCurrentTime = func() time.Time {
				return mocks.timeStamp
			}

			mocks.initData.On("InitTimeout").Return(time.Second).Maybe()
			mocks.initData.On("MemorySizeMB").Return(uint64(128)).Maybe()
			mocks.initData.On("FunctionARN").Return("function-arn").Maybe()
			mocks.initData.On("FunctionVersionID").Return("function-version-id").Maybe()
			mocks.initData.On("RuntimeVersion").Return("python3.9").Maybe()
			mocks.initData.On("ArtefactType").Return(intmodel.ArtefactTypeOCI).Maybe()
			mocks.initData.On("AmiId").Return("ami-1234567").Maybe()
			mocks.initData.On("AvailabilityZoneId").Return("us-west-2").Maybe()

			mocks.logger.On("Log",
				mock.MatchedBy(func(op servicelogs.Operation) bool {
					return assert.Equal(t, servicelogs.ShutdownOp, op)
				}),
				mock.AnythingOfType("time.Time"),
				[]servicelogs.Tuple(nil),
				[]servicelogs.Tuple(nil),
				mock.MatchedBy(func(metrics []servicelogs.Metric) bool {
					slices.SortFunc(metrics, metricsSortFunc)
					slices.SortFunc(tt.expectedMetrics, metricsSortFunc)
					assert.Equal(t, len(tt.expectedMetrics), len(metrics))
					for i := range len(tt.expectedMetrics) {
						require.Equal(t, tt.expectedMetrics[i].Key, metrics[i].Key)
						require.Equal(t, tt.expectedMetrics[i].Type, metrics[i].Type, fmt.Sprintf("wrong format for %s", metrics[i].Key))
						require.Equal(t, tt.expectedMetrics[i].Value, metrics[i].Value, fmt.Sprintf("wrong value for %s", metrics[i].Key))
					}

					return true
				}),
			).Once()

			mocks.timeStamp = time.Now()
			tt.metricFlow(m, mocks)

			m.SendMetrics(mocks.err)
			checkShutdownMocksExpectations(t, mocks)
		})
	}
}
