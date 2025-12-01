// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	json "encoding/json"

	mock "github.com/stretchr/testify/mock"
	model "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type MockRuntimeErrorRequest struct {
	mock.Mock
}

func (_m *MockRuntimeErrorRequest) ContentType() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ContentType")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockRuntimeErrorRequest) ErrorCategory() model.ErrorCategory {
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

func (_m *MockRuntimeErrorRequest) ErrorDetails() string {
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

func (_m *MockRuntimeErrorRequest) ErrorType() model.ErrorType {
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

func (_m *MockRuntimeErrorRequest) GetError() model.AppError {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetError")
	}

	var r0 model.AppError
	if rf, ok := ret.Get(0).(func() model.AppError); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(model.AppError)
		}
	}

	return r0
}

func (_m *MockRuntimeErrorRequest) GetXrayErrorCause() json.RawMessage {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetXrayErrorCause")
	}

	var r0 json.RawMessage
	if rf, ok := ret.Get(0).(func() json.RawMessage); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(json.RawMessage)
		}
	}

	return r0
}

func (_m *MockRuntimeErrorRequest) InvokeID() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for InvokeID")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockRuntimeErrorRequest) IsRuntimeError(_a0 model.AppError) bool {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for IsRuntimeError")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func(model.AppError) bool); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

func (_m *MockRuntimeErrorRequest) ReturnCode() int {
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

func NewMockRuntimeErrorRequest(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockRuntimeErrorRequest {
	mock := &MockRuntimeErrorRequest{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
