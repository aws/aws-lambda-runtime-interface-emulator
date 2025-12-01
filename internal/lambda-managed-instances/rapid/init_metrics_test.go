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

type initMetricsMocks struct {
	timeStamp time.Time
	error     model.AppError
	initData  interop.MockInitStaticDataProvider
	logger    servicelogs.MockLogger
}

func checkMocksExpectations(t *testing.T, mocks *initMetricsMocks) {
	mocks.initData.AssertExpectations(t)
	mocks.logger.AssertExpectations(t)
}

func Test_initMetrics(t *testing.T) {
	tests := []struct {
		name                string
		metricFlow          func(m *initMetrics, mocks *initMetricsMocks)
		expectedMetrics     []servicelogs.Metric
		expectedRunDuration time.Duration
	}{
		{
			name: "malformed_request_flow",
			metricFlow: func(m *initMetrics, mocks *initMetricsMocks) {
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				mocks.error = model.NewClientError(nil, model.ErrorSeverityInvalid, model.ErrorMalformedRequest)
			},
			expectedMetrics: []servicelogs.Metric{
				{Type: servicelogs.TimerType, Key: "TotalDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "TotalExtensionsCount", Value: 0},
				{Type: servicelogs.CounterType, Key: "InternalExtensionsCount", Value: 0},
				{Type: servicelogs.CounterType, Key: "ExternalExtensionsCount", Value: 0},
				{Type: servicelogs.TimerType, Key: "PlatformOverheadDuration", Value: 1000000},
				{Type: servicelogs.CounterType, Key: "ClientError", Value: 1},
				{Type: servicelogs.CounterType, Key: "CustomerError", Value: 0},
				{Type: servicelogs.CounterType, Key: "PlatformError", Value: 0},
				{Type: servicelogs.CounterType, Key: "ClientErrorReason-ErrMalformedRequest", Value: 1},
				{Type: servicelogs.CounterType, Key: "NonCustomerError", Value: 1},
			},
		},
		{
			name: "init_platform_error",
			metricFlow: func(m *initMetrics, mocks *initMetricsMocks) {
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				m.TriggerStartRequest()
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				m.TriggerInitCustomerPhaseDone()
				m.SetExtensionsNumber(2, 3)
				mocks.error = model.NewPlatformError(nil, model.ErrorAgentCountRegistrationFailed)
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
			},
			expectedMetrics: []servicelogs.Metric{
				{Type: servicelogs.TimerType, Key: "TotalDuration", Value: 3000000},
				{Type: servicelogs.CounterType, Key: "TotalExtensionsCount", Value: 5},
				{Type: servicelogs.CounterType, Key: "InternalExtensionsCount", Value: 2},
				{Type: servicelogs.CounterType, Key: "ExternalExtensionsCount", Value: 3},
				{Type: servicelogs.TimerType, Key: "RunDuration", Value: 1000000},
				{Type: servicelogs.TimerType, Key: "PlatformOverheadDuration", Value: 2000000},
				{Type: servicelogs.CounterType, Key: "ClientError", Value: 0},
				{Type: servicelogs.CounterType, Key: "CustomerError", Value: 0},
				{Type: servicelogs.CounterType, Key: "PlatformError", Value: 1},
				{Type: servicelogs.CounterType, Key: "PlatformErrorReason-Extension.CountRegistrationFailed", Value: 1},
				{Type: servicelogs.CounterType, Key: "NonCustomerError", Value: 1},
			},
			expectedRunDuration: time.Second,
		},
		{
			name: "init_runtime_failed",
			metricFlow: func(m *initMetrics, mocks *initMetricsMocks) {
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				m.TriggerStartRequest()
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				m.TriggerStartingRuntime()
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				m.TriggerInitCustomerPhaseDone()
				m.SetExtensionsNumber(2, 3)
				mocks.error = model.NewCustomerError(model.ErrorRuntimeInvalidWorkingDir, model.WithSeverity(model.ErrorSeverityInvalid))
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
			},
			expectedMetrics: []servicelogs.Metric{
				{Type: servicelogs.TimerType, Key: "TotalDuration", Value: 4000000},
				{Type: servicelogs.CounterType, Key: "TotalExtensionsCount", Value: 5},
				{Type: servicelogs.CounterType, Key: "InternalExtensionsCount", Value: 2},
				{Type: servicelogs.CounterType, Key: "ExternalExtensionsCount", Value: 3},
				{Type: servicelogs.TimerType, Key: "RunDuration", Value: 2000000},
				{Type: servicelogs.TimerType, Key: "PlatformOverheadDuration", Value: 2000000},
				{Type: servicelogs.CounterType, Key: "ClientError", Value: 0},
				{Type: servicelogs.CounterType, Key: "CustomerError", Value: 1},
				{Type: servicelogs.CounterType, Key: "PlatformError", Value: 0},
				{Type: servicelogs.CounterType, Key: "CustomerErrorReason-Runtime.InvalidWorkingDir", Value: 1},
				{Type: servicelogs.CounterType, Key: "NonCustomerError", Value: 0},
			},
			expectedRunDuration: 2 * time.Second,
		},
		{
			name: "init_full_flow",
			metricFlow: func(m *initMetrics, mocks *initMetricsMocks) {
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				m.TriggerStartRequest()
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				m.TriggerStartingRuntime()
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				m.TriggerRuntimeDone()
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
				m.TriggerInitCustomerPhaseDone()
				m.SetExtensionsNumber(2, 3)
				m.SetLogsAPIMetrics(map[string]int{
					"logs_api_subscribe_success":    2,
					"logs_api_subscribe_client_err": 1,
					"logs_api_subscribe_server_err": 0,
					"logs_api_num_subscribers":      2,
				})
				mocks.timeStamp = mocks.timeStamp.Add(time.Second)
			},
			expectedMetrics: []servicelogs.Metric{
				{Type: servicelogs.TimerType, Key: "TotalDuration", Value: 5000000},
				{Type: servicelogs.CounterType, Key: "TotalExtensionsCount", Value: 5},
				{Type: servicelogs.CounterType, Key: "InternalExtensionsCount", Value: 2},
				{Type: servicelogs.CounterType, Key: "ExternalExtensionsCount", Value: 3},
				{Type: servicelogs.TimerType, Key: "RuntimeDuration", Value: 1000000},
				{Type: servicelogs.TimerType, Key: "RunDuration", Value: 3000000},
				{Type: servicelogs.TimerType, Key: "PlatformOverheadDuration", Value: 2000000},
				{Type: servicelogs.CounterType, Key: "ClientError", Value: 0},
				{Type: servicelogs.CounterType, Key: "CustomerError", Value: 0},
				{Type: servicelogs.CounterType, Key: "PlatformError", Value: 0},
				{Type: servicelogs.CounterType, Key: "NonCustomerError", Value: 0},
				{Type: servicelogs.CounterType, Key: "logs_api_subscribe_success", Value: 2},
				{Type: servicelogs.CounterType, Key: "logs_api_subscribe_client_err", Value: 1},
				{Type: servicelogs.CounterType, Key: "logs_api_subscribe_server_err", Value: 0},
				{Type: servicelogs.CounterType, Key: "logs_api_num_subscribers", Value: 2},
			},
			expectedRunDuration: 3 * time.Second,
		},
	}

	metricsSortFunc := func(a, b servicelogs.Metric) int {
		return cmp.Compare(a.Key, b.Key)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := &initMetricsMocks{
				initData: interop.MockInitStaticDataProvider{},
				logger:   servicelogs.MockLogger{},
			}

			m := NewInitMetrics(&mocks.logger)
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
					return assert.Equal(t, servicelogs.InitOp, op)
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

			m.TriggerGetRequest()
			tt.metricFlow(m, mocks)
			m.TriggerInitDone(mocks.error)

			require.NoError(t, m.SendMetrics())
			assert.Equal(t, tt.expectedRunDuration, m.RunDuration())
			checkMocksExpectations(t, mocks)
		})
	}
}
