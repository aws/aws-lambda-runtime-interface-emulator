// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

type mockRelay struct {
	mock.Mock
}

func (_m *mockRelay) broadcast(record interface{}, category string, typ string) {
	_m.Called(record, category, typ)
}

func (_m *mockRelay) flush(ctx context.Context) {
	_m.Called(ctx)
}

func newMockRelay(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockRelay {
	mock := &mockRelay{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
