// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	time "time"

	mock "github.com/stretchr/testify/mock"
)

type MockReconnectMetrics struct {
	mock.Mock
}

func (_m *MockReconnectMetrics) SetFunctionDoneTime(t time.Time) {
	_m.Called(t)
}

func (_m *MockReconnectMetrics) TriggerConnectionGap(lastDisconnectTime *time.Time) {
	_m.Called(lastDisconnectTime)
}

func (_m *MockReconnectMetrics) TriggerPollEnd() {
	_m.Called()
}

func (_m *MockReconnectMetrics) TriggerPollStart() {
	_m.Called()
}

func (_m *MockReconnectMetrics) TriggerResponseReplayDone(size int) {
	_m.Called(size)
}

func (_m *MockReconnectMetrics) TriggerResponseReplayStart() {
	_m.Called()
}

func NewMockReconnectMetrics(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockReconnectMetrics {
	mock := &MockReconnectMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
