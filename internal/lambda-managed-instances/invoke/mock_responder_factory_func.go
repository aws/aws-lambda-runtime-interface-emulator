// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	context "context"
	http "net/http"

	mock "github.com/stretchr/testify/mock"
	interop "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
)

type MockResponderFactoryFunc struct {
	mock.Mock
}

func (_m *MockResponderFactoryFunc) Execute(_a0 context.Context, _a1 interop.InvokeRequest, _a2 http.ResponseWriter) InvokeResponseSender {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 InvokeResponseSender
	if rf, ok := ret.Get(0).(func(context.Context, interop.InvokeRequest, http.ResponseWriter) InvokeResponseSender); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(InvokeResponseSender)
		}
	}

	return r0
}

func NewMockResponderFactoryFunc(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockResponderFactoryFunc {
	mock := &MockResponderFactoryFunc{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
