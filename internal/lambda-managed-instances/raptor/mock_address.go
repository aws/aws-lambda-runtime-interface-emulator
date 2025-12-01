// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package raptor

import (
	net "net"

	mock "github.com/stretchr/testify/mock"
)

type MockAddress struct {
	mock.Mock
}

func (_m *MockAddress) Protocol() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Protocol")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockAddress) String() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for String")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockAddress) UpdateFromListener(listener net.Listener) {
	_m.Called(listener)
}

func NewMockAddress(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockAddress {
	mock := &MockAddress{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
