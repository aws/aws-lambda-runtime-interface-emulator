// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package raptor

import (
	mock "github.com/stretchr/testify/mock"
	interop "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"

	servicelogs "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/servicelogs"

	time "time"
)

type mockRaptorLogger struct {
	mock.Mock
}

func (_m *mockRaptorLogger) Close() error {
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

func (_m *mockRaptorLogger) Log(op servicelogs.Operation, opStart time.Time, props []servicelogs.Tuple, dims []servicelogs.Tuple, metrics []servicelogs.Metric) {
	_m.Called(op, opStart, props, dims, metrics)
}

func (_m *mockRaptorLogger) SetInitData(initData interop.InitStaticDataProvider) {
	_m.Called(initData)
}

func newMockRaptorLogger(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockRaptorLogger {
	mock := &mockRaptorLogger{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
