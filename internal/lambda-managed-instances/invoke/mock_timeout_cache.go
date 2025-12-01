// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import mock "github.com/stretchr/testify/mock"

type mockTimeoutCache struct {
	mock.Mock
}

func (_m *mockTimeoutCache) Consume(invokeID string) bool {
	ret := _m.Called(invokeID)

	if len(ret) == 0 {
		panic("no return value specified for Consume")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func(string) bool); ok {
		r0 = rf(invokeID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

func (_m *mockTimeoutCache) Register(invokeID string) {
	_m.Called(invokeID)
}

func newMockTimeoutCache(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockTimeoutCache {
	mock := &mockTimeoutCache{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
