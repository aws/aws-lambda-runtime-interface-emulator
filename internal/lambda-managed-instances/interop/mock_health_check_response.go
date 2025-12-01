// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import mock "github.com/stretchr/testify/mock"

type MockHealthCheckResponse struct {
	mock.Mock
}

func (_m *MockHealthCheckResponse) healthCheckResponse() {
	_m.Called()
}

func NewMockHealthCheckResponse(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockHealthCheckResponse {
	mock := &MockHealthCheckResponse{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
