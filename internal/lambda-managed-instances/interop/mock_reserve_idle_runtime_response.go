// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import mock "github.com/stretchr/testify/mock"

type MockReserveIdleRuntimeResponse struct {
	mock.Mock
}

func (_m *MockReserveIdleRuntimeResponse) reserveIdleRuntimeResponse() {
	_m.Called()
}

func NewMockReserveIdleRuntimeResponse(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockReserveIdleRuntimeResponse {
	mock := &MockReserveIdleRuntimeResponse{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
