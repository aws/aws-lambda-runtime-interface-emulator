// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package raptor

import (
	mock "github.com/stretchr/testify/mock"
	model "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type mockShutdownHandler struct {
	mock.Mock
}

func (_m *mockShutdownHandler) Shutdown(shutdownReason model.AppError) {
	_m.Called(shutdownReason)
}

func newMockShutdownHandler(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockShutdownHandler {
	mock := &mockShutdownHandler{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
