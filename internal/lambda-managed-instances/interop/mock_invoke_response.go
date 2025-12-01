// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import mock "github.com/stretchr/testify/mock"

type MockInvokeResponse struct {
	mock.Mock
}

func (_m *MockInvokeResponse) invokeResponse() {
	_m.Called()
}

func NewMockInvokeResponse(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockInvokeResponse {
	mock := &MockInvokeResponse{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
