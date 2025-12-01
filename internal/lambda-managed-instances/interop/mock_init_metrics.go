// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	time "time"

	mock "github.com/stretchr/testify/mock"
	model "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type MockInitMetrics struct {
	mock.Mock
}

func (_m *MockInitMetrics) RunDuration() time.Duration {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for RunDuration")
	}

	var r0 time.Duration
	if rf, ok := ret.Get(0).(func() time.Duration); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(time.Duration)
	}

	return r0
}

func (_m *MockInitMetrics) SendMetrics() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for SendMetrics")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *MockInitMetrics) SetExtensionsNumber(internal int, external int) {
	_m.Called(internal, external)
}

func (_m *MockInitMetrics) SetLogsAPIMetrics(_a0 TelemetrySubscriptionMetrics) {
	_m.Called(_a0)
}

func (_m *MockInitMetrics) TriggerGetRequest() {
	_m.Called()
}

func (_m *MockInitMetrics) TriggerInitCustomerPhaseDone() {
	_m.Called()
}

func (_m *MockInitMetrics) TriggerInitDone(_a0 model.AppError) {
	_m.Called(_a0)
}

func (_m *MockInitMetrics) TriggerRuntimeDone() {
	_m.Called()
}

func (_m *MockInitMetrics) TriggerStartRequest() {
	_m.Called()
}

func (_m *MockInitMetrics) TriggerStartingRuntime() {
	_m.Called()
}

func NewMockInitMetrics(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockInitMetrics {
	mock := &MockInitMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
