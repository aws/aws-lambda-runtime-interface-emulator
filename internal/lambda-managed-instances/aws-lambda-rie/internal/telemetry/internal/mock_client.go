// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

type MockClient struct {
	mock.Mock
}

func (_m *MockClient) send(ctx context.Context, b batch) error {
	ret := _m.Called(ctx, b)

	if len(ret) == 0 {
		panic("no return value specified for send")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, batch) error); ok {
		r0 = rf(ctx, b)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func NewMockClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockClient {
	mock := &MockClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
