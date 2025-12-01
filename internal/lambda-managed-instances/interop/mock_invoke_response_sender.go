// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import mock "github.com/stretchr/testify/mock"

type MockInvokeResponseSender struct {
	mock.Mock
}

func (_m *MockInvokeResponseSender) SendErrorResponse(invokeID string, response *ErrorInvokeResponse) (*InvokeResponseMetrics, error) {
	ret := _m.Called(invokeID, response)

	if len(ret) == 0 {
		panic("no return value specified for SendErrorResponse")
	}

	var r0 *InvokeResponseMetrics
	var r1 error
	if rf, ok := ret.Get(0).(func(string, *ErrorInvokeResponse) (*InvokeResponseMetrics, error)); ok {
		return rf(invokeID, response)
	}
	if rf, ok := ret.Get(0).(func(string, *ErrorInvokeResponse) *InvokeResponseMetrics); ok {
		r0 = rf(invokeID, response)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*InvokeResponseMetrics)
		}
	}

	if rf, ok := ret.Get(1).(func(string, *ErrorInvokeResponse) error); ok {
		r1 = rf(invokeID, response)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *MockInvokeResponseSender) SendResponse(invokeID string, response *StreamableInvokeResponse) (*InvokeResponseMetrics, error) {
	ret := _m.Called(invokeID, response)

	if len(ret) == 0 {
		panic("no return value specified for SendResponse")
	}

	var r0 *InvokeResponseMetrics
	var r1 error
	if rf, ok := ret.Get(0).(func(string, *StreamableInvokeResponse) (*InvokeResponseMetrics, error)); ok {
		return rf(invokeID, response)
	}
	if rf, ok := ret.Get(0).(func(string, *StreamableInvokeResponse) *InvokeResponseMetrics); ok {
		r0 = rf(invokeID, response)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*InvokeResponseMetrics)
		}
	}

	if rf, ok := ret.Get(1).(func(string, *StreamableInvokeResponse) error); ok {
		r1 = rf(invokeID, response)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
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
