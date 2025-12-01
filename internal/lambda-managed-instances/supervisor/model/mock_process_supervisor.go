// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

type MockProcessSupervisor struct {
	mock.Mock
}

func (_m *MockProcessSupervisor) Events(_a0 context.Context) (<-chan Event, error) {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Events")
	}

	var r0 <-chan Event
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (<-chan Event, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(context.Context) <-chan Event); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan Event)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *MockProcessSupervisor) Exec(_a0 context.Context, _a1 *ExecRequest) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for Exec")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *ExecRequest) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *MockProcessSupervisor) Kill(_a0 context.Context, _a1 *KillRequest) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for Kill")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *KillRequest) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *MockProcessSupervisor) Terminate(_a0 context.Context, _a1 *TerminateRequest) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for Terminate")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *TerminateRequest) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func NewMockProcessSupervisor(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockProcessSupervisor {
	mock := &MockProcessSupervisor{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
