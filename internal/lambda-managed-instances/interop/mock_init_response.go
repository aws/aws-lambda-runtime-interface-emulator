// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import mock "github.com/stretchr/testify/mock"

type MockInitResponse struct {
	mock.Mock
}

func (_m *MockInitResponse) initResponse() {
	_m.Called()
}

func NewMockInitResponse(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockInitResponse {
	mock := &MockInitResponse{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
