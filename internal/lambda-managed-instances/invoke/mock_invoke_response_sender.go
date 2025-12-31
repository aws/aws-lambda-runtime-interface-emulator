// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	interop "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"

	time "time"
)

type MockInvokeResponseSender struct {
	mock.Mock
}

func (_m *MockInvokeResponseSender) ErrorPayloadSizeBytes() int {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ErrorPayloadSizeBytes")
	}

	var r0 int
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	return r0
}

func (_m *MockInvokeResponseSender) SendError(_a0 ErrorForInvoker, _a1 interop.InitStaticDataProvider) {
	_m.Called(_a0, _a1)
}

func (_m *MockInvokeResponseSender) SendErrorTrailers(_a0 ErrorForInvoker, _a1 InvokeBodyResponseStatus) {
	_m.Called(_a0, _a1)
}

func (_m *MockInvokeResponseSender) SendRuntimeResponseBody(ctx context.Context, runtimeResp RuntimeResponseRequest, functionTimeout time.Duration) SendResponseBodyResult {
	ret := _m.Called(ctx, runtimeResp, functionTimeout)

	if len(ret) == 0 {
		panic("no return value specified for SendRuntimeResponseBody")
	}

	var r0 SendResponseBodyResult
	if rf, ok := ret.Get(0).(func(context.Context, RuntimeResponseRequest, time.Duration) SendResponseBodyResult); ok {
		r0 = rf(ctx, runtimeResp, functionTimeout)
	} else {
		r0 = ret.Get(0).(SendResponseBodyResult)
	}

	return r0
}

func (_m *MockInvokeResponseSender) SendRuntimeResponseHeaders(initData interop.InitStaticDataProvider, contentType string, responseMode string) {
	_m.Called(initData, contentType, responseMode)
}

func (_m *MockInvokeResponseSender) SendRuntimeResponseTrailers(_a0 RuntimeResponseRequest) {
	_m.Called(_a0)
}

func NewMockInvokeResponseSender(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockInvokeResponseSender {
	mock := &MockInvokeResponseSender{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
