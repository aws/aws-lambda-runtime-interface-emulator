// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import mock "github.com/stretchr/testify/mock"

type MockDurationMetricTimer struct {
	mock.Mock
}

func (_m *MockDurationMetricTimer) Done() {
	_m.Called()
}

func NewMockDurationMetricTimer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockDurationMetricTimer {
	mock := &MockDurationMetricTimer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
