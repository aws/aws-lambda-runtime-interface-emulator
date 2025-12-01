// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	interop "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"

	model "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type mockRunningInvoke struct {
	mock.Mock
}

func (_m *mockRunningInvoke) CancelAsync(_a0 model.AppError) {
	_m.Called(_a0)
}

func (_m *mockRunningInvoke) RunInvokeAndSendResult(_a0 context.Context, _a1 interop.InitStaticDataProvider, _a2 interop.InvokeRequest, _a3 interop.InvokeMetrics) model.AppError {
	ret := _m.Called(_a0, _a1, _a2, _a3)

	if len(ret) == 0 {
		panic("no return value specified for RunInvokeAndSendResult")
	}

	var r0 model.AppError
	if rf, ok := ret.Get(0).(func(context.Context, interop.InitStaticDataProvider, interop.InvokeRequest, interop.InvokeMetrics) model.AppError); ok {
		r0 = rf(_a0, _a1, _a2, _a3)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(model.AppError)
		}
	}

	return r0
}

func (_m *mockRunningInvoke) RuntimeError(_a0 context.Context, _a1 RuntimeErrorRequest) model.AppError {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for RuntimeError")
	}

	var r0 model.AppError
	if rf, ok := ret.Get(0).(func(context.Context, RuntimeErrorRequest) model.AppError); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(model.AppError)
		}
	}

	return r0
}

func (_m *mockRunningInvoke) RuntimeNextWait(_a0 context.Context) model.AppError {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for RuntimeNextWait")
	}

	var r0 model.AppError
	if rf, ok := ret.Get(0).(func(context.Context) model.AppError); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(model.AppError)
		}
	}

	return r0
}

func (_m *mockRunningInvoke) RuntimeResponse(_a0 context.Context, _a1 RuntimeResponseRequest) model.AppError {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for RuntimeResponse")
	}

	var r0 model.AppError
	if rf, ok := ret.Get(0).(func(context.Context, RuntimeResponseRequest) model.AppError); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(model.AppError)
		}
	}

	return r0
}

func newMockRunningInvoke(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockRunningInvoke {
	mock := &mockRunningInvoke{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
