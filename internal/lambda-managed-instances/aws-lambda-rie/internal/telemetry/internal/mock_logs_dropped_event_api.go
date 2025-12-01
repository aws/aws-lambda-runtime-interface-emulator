// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import mock "github.com/stretchr/testify/mock"

type MockLogsDroppedEventAPI struct {
	mock.Mock
}

func (_m *MockLogsDroppedEventAPI) SendPlatformLogsDropped(droppedBytes int, droppedRecords int, reason string) error {
	ret := _m.Called(droppedBytes, droppedRecords, reason)

	if len(ret) == 0 {
		panic("no return value specified for SendPlatformLogsDropped")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(int, int, string) error); ok {
		r0 = rf(droppedBytes, droppedRecords, reason)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func NewMockLogsDroppedEventAPI(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockLogsDroppedEventAPI {
	mock := &MockLogsDroppedEventAPI{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
