// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	mock "github.com/stretchr/testify/mock"
	model "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type MockErrorForInvoker struct {
	mock.Mock
}

func (_m *MockErrorForInvoker) ErrorCategory() model.ErrorCategory {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ErrorCategory")
	}

	var r0 model.ErrorCategory
	if rf, ok := ret.Get(0).(func() model.ErrorCategory); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(model.ErrorCategory)
	}

	return r0
}

func (_m *MockErrorForInvoker) ErrorDetails() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ErrorDetails")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockErrorForInvoker) ErrorType() model.ErrorType {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ErrorType")
	}

	var r0 model.ErrorType
	if rf, ok := ret.Get(0).(func() model.ErrorType); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(model.ErrorType)
	}

	return r0
}

func (_m *MockErrorForInvoker) ReturnCode() int {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ReturnCode")
	}

	var r0 int
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	return r0
}

func NewMockErrorForInvoker(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockErrorForInvoker {
	mock := &MockErrorForInvoker{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
