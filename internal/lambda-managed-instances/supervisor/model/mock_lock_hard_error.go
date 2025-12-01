// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import mock "github.com/stretchr/testify/mock"

type MockLockHardError struct {
	mock.Mock
}

func (_m *MockLockHardError) Cause() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Cause")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockLockHardError) HookName() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for HookName")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockLockHardError) Reason() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Reason")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockLockHardError) Source() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Source")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func NewMockLockHardError(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockLockHardError {
	mock := &MockLockHardError{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
