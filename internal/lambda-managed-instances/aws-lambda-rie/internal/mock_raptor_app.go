// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	context "context"
	http "net/http"

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

func (_m *mockRaptorApp) Invoke(ctx context.Context, msg interop.InvokeRequest, metrics interop.InvokeMetrics, responseWriter http.ResponseWriter) (rapidmodel.AppError, bool, bool) {
	ret := _m.Called(ctx, msg, metrics, responseWriter)

	if len(ret) == 0 {
		panic("no return value specified for Invoke")
	}

	var r0 rapidmodel.AppError
	var r1 bool
	var r2 bool
	if rf, ok := ret.Get(0).(func(context.Context, interop.InvokeRequest, interop.InvokeMetrics, http.ResponseWriter) (rapidmodel.AppError, bool, bool)); ok {
		return rf(ctx, msg, metrics, responseWriter)
	}
	if rf, ok := ret.Get(0).(func(context.Context, interop.InvokeRequest, interop.InvokeMetrics, http.ResponseWriter) rapidmodel.AppError); ok {
		r0 = rf(ctx, msg, metrics, responseWriter)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(rapidmodel.AppError)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, interop.InvokeRequest, interop.InvokeMetrics, http.ResponseWriter) bool); ok {
		r1 = rf(ctx, msg, metrics, responseWriter)
	} else {
		r1 = ret.Get(1).(bool)
	}

	if rf, ok := ret.Get(2).(func(context.Context, interop.InvokeRequest, interop.InvokeMetrics, http.ResponseWriter) bool); ok {
		r2 = rf(ctx, msg, metrics, responseWriter)
	} else {
		r2 = ret.Get(2).(bool)
	}

	return r0, r1, r2
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
