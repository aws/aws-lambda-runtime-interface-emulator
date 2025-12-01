// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import mock "github.com/stretchr/testify/mock"

type MockMessage struct {
	mock.Mock
}

func NewMockMessage(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockMessage {
	mock := &MockMessage{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
