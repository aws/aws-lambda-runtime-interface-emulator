// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import mock "github.com/stretchr/testify/mock"

type MockShutdownResponse struct {
	mock.Mock
}

func (_m *MockShutdownResponse) shutdownResponse() {
	_m.Called()
}

func NewMockShutdownResponse(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockShutdownResponse {
	mock := &MockShutdownResponse{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
