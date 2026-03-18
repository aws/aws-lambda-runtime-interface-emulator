// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import mock "github.com/stretchr/testify/mock"

type MockReserveIdleRuntimeRequest struct {
	mock.Mock
}

func (_m *MockReserveIdleRuntimeRequest) InvokeID() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for InvokeID")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func NewMockReserveIdleRuntimeRequest(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockReserveIdleRuntimeRequest {
	mock := &MockReserveIdleRuntimeRequest{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
