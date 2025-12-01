// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	context "context"
	json "encoding/json"

	mock "github.com/stretchr/testify/mock"
)

type mockSub struct {
	mock.Mock
}

func (_m *mockSub) AgentName() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for AgentName")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *mockSub) Flush(ctx context.Context) {
	_m.Called(ctx)
}

func (_m *mockSub) SendAsync(event json.RawMessage, cat string) {
	_m.Called(event, cat)
}

func newMockSub(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockSub {
	mock := &mockSub{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
