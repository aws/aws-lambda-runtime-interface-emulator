// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	interop "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"

	model "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"

	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type mockRaptorApp struct {
	mock.Mock
}

func (_m *mockRaptorApp) Init(ctx context.Context, req *model.InitRequestMessage, metrics interop.InitMetrics) rapidmodel.AppError {
	ret := _m.Called(ctx, req, metrics)

	if len(ret) == 0 {
		panic("no return value specified for Init")
	}

	var r0 rapidmodel.AppError
	if rf, ok := ret.Get(0).(func(context.Context, *model.InitRequestMessage, interop.InitMetrics) rapidmodel.AppError); ok {
		r0 = rf(ctx, req, metrics)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(rapidmodel.AppError)
		}
	}

	return r0
}

func (_m *mockRaptorApp) Invoke(ctx context.Context, msg interop.InvokeRequest, metrics interop.InvokeMetrics) (rapidmodel.AppError, bool) {
	ret := _m.Called(ctx, msg, metrics)

	if len(ret) == 0 {
		panic("no return value specified for Invoke")
	}

	var r0 rapidmodel.AppError
	var r1 bool
	if rf, ok := ret.Get(0).(func(context.Context, interop.InvokeRequest, interop.InvokeMetrics) (rapidmodel.AppError, bool)); ok {
		return rf(ctx, msg, metrics)
	}
	if rf, ok := ret.Get(0).(func(context.Context, interop.InvokeRequest, interop.InvokeMetrics) rapidmodel.AppError); ok {
		r0 = rf(ctx, msg, metrics)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(rapidmodel.AppError)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, interop.InvokeRequest, interop.InvokeMetrics) bool); ok {
		r1 = rf(ctx, msg, metrics)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

func newMockRaptorApp(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockRaptorApp {
	mock := &mockRaptorApp{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
