// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import mock "github.com/stretchr/testify/mock"

type mockSubscriptionStore struct {
	mock.Mock
}

func (_m *mockSubscriptionStore) addSubscriber(subscriber sub) error {
	ret := _m.Called(subscriber)

	if len(ret) == 0 {
		panic("no return value specified for addSubscriber")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(sub) error); ok {
		r0 = rf(subscriber)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *mockSubscriptionStore) disableAddSubscriber() {
	_m.Called()
}

func newMockSubscriptionStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockSubscriptionStore {
	mock := &mockSubscriptionStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
