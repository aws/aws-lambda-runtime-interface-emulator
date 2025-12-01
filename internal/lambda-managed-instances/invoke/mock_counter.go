// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import mock "github.com/stretchr/testify/mock"

type MockCounter struct {
	mock.Mock
}

func (_m *MockCounter) AddInvoke(proxiedBytes uint64) {
	_m.Called(proxiedBytes)
}

func NewMockCounter(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockCounter {
	mock := &MockCounter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
