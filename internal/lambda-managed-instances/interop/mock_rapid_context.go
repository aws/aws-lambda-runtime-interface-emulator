// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	context "context"
	netip "net/netip"

	mock "github.com/stretchr/testify/mock"
	model "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type MockRapidContext struct {
	mock.Mock
}

func (_m *MockRapidContext) HandleInit(ctx context.Context, initData InitExecutionData, initMetrics InitMetrics) model.AppError {
	ret := _m.Called(ctx, initData, initMetrics)

	if len(ret) == 0 {
		panic("no return value specified for HandleInit")
	}

	var r0 model.AppError
	if rf, ok := ret.Get(0).(func(context.Context, InitExecutionData, InitMetrics) model.AppError); ok {
		r0 = rf(ctx, initData, initMetrics)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(model.AppError)
		}
	}

	return r0
}

func (_m *MockRapidContext) HandleInvoke(ctx context.Context, invokeRequest InvokeRequest, invokeMetrics InvokeMetrics) (model.AppError, bool) {
	ret := _m.Called(ctx, invokeRequest, invokeMetrics)

	if len(ret) == 0 {
		panic("no return value specified for HandleInvoke")
	}

	var r0 model.AppError
	var r1 bool
	if rf, ok := ret.Get(0).(func(context.Context, InvokeRequest, InvokeMetrics) (model.AppError, bool)); ok {
		return rf(ctx, invokeRequest, invokeMetrics)
	}
	if rf, ok := ret.Get(0).(func(context.Context, InvokeRequest, InvokeMetrics) model.AppError); ok {
		r0 = rf(ctx, invokeRequest, invokeMetrics)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(model.AppError)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, InvokeRequest, InvokeMetrics) bool); ok {
		r1 = rf(ctx, invokeRequest, invokeMetrics)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

func (_m *MockRapidContext) HandleShutdown(shutdownCause model.AppError, metrics ShutdownMetrics) model.AppError {
	ret := _m.Called(shutdownCause, metrics)

	if len(ret) == 0 {
		panic("no return value specified for HandleShutdown")
	}

	var r0 model.AppError
	if rf, ok := ret.Get(0).(func(model.AppError, ShutdownMetrics) model.AppError); ok {
		r0 = rf(shutdownCause, metrics)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(model.AppError)
		}
	}

	return r0
}

func (_m *MockRapidContext) ProcessTerminationNotifier() <-chan model.AppError {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ProcessTerminationNotifier")
	}

	var r0 <-chan model.AppError
	if rf, ok := ret.Get(0).(func() <-chan model.AppError); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan model.AppError)
		}
	}

	return r0
}

func (_m *MockRapidContext) RuntimeAPIAddrPort() netip.AddrPort {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for RuntimeAPIAddrPort")
	}

	var r0 netip.AddrPort
	if rf, ok := ret.Get(0).(func() netip.AddrPort); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(netip.AddrPort)
	}

	return r0
}

func NewMockRapidContext(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockRapidContext {
	mock := &MockRapidContext{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
