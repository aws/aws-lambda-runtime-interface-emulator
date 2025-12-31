// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	mock "github.com/stretchr/testify/mock"
	model "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	servicelogs "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/servicelogs"
)

type MockShutdownMetrics struct {
	mock.Mock
}

func (_m *MockShutdownMetrics) AddMetric(metric servicelogs.Metric) {
	_m.Called(metric)
}

func (_m *MockShutdownMetrics) CreateDurationMetric(name string) DurationMetricTimer {
	ret := _m.Called(name)

	if len(ret) == 0 {
		panic("no return value specified for CreateDurationMetric")
	}

	var r0 DurationMetricTimer
	if rf, ok := ret.Get(0).(func(string) DurationMetricTimer); ok {
		r0 = rf(name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(DurationMetricTimer)
		}
	}

	return r0
}

func (_m *MockShutdownMetrics) SendMetrics(error model.AppError) {
	_m.Called(error)
}

func (_m *MockShutdownMetrics) SetAgentCount(internal int, external int) {
	_m.Called(internal, external)
}

func NewMockShutdownMetrics(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockShutdownMetrics {
	mock := &MockShutdownMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
