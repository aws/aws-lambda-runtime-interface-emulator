// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invariant

import mock "github.com/stretchr/testify/mock"

type MockViolationExecutor struct {
	mock.Mock
}

func (_m *MockViolationExecutor) Exec(_a0 ViolationError) {
	_m.Called(_a0)
}

func NewMockViolationExecutor(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockViolationExecutor {
	mock := &MockViolationExecutor{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
