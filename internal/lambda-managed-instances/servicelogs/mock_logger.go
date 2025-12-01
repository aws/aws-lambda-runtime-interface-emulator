// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package servicelogs

import (
	time "time"

	mock "github.com/stretchr/testify/mock"
)

type MockLogger struct {
	mock.Mock
}

func (_m *MockLogger) Close() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Close")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *MockLogger) Log(op Operation, opStart time.Time, props []Tuple, dims []Tuple, metrics []Metric) {
	_m.Called(op, opStart, props, dims, metrics)
}

func NewMockLogger(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockLogger {
	mock := &MockLogger{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
