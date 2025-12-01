// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import mock "github.com/stretchr/testify/mock"

type MockServer struct {
	mock.Mock
}

func (_m *MockServer) SendInitErrorResponse(response *ErrorInvokeResponse) (*InvokeResponseMetrics, error) {
	ret := _m.Called(response)

	if len(ret) == 0 {
		panic("no return value specified for SendInitErrorResponse")
	}

	var r0 *InvokeResponseMetrics
	var r1 error
	if rf, ok := ret.Get(0).(func(*ErrorInvokeResponse) (*InvokeResponseMetrics, error)); ok {
		return rf(response)
	}
	if rf, ok := ret.Get(0).(func(*ErrorInvokeResponse) *InvokeResponseMetrics); ok {
		r0 = rf(response)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*InvokeResponseMetrics)
		}
	}

	if rf, ok := ret.Get(1).(func(*ErrorInvokeResponse) error); ok {
		r1 = rf(response)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func NewMockServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockServer {
	mock := &MockServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
