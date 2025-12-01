// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import mock "github.com/stretchr/testify/mock"

type mockTelemetrySubscriptionEventAPI struct {
	mock.Mock
}

func (_m *mockTelemetrySubscriptionEventAPI) sendTelemetrySubscription(agentName string, state string, categories []string) error {
	ret := _m.Called(agentName, state, categories)

	if len(ret) == 0 {
		panic("no return value specified for sendTelemetrySubscription")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, []string) error); ok {
		r0 = rf(agentName, state, categories)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func newMockTelemetrySubscriptionEventAPI(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockTelemetrySubscriptionEventAPI {
	mock := &mockTelemetrySubscriptionEventAPI{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
