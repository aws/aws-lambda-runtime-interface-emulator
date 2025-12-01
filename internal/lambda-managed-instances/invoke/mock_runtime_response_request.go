// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	io "io"

	mock "github.com/stretchr/testify/mock"
	model "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type MockRuntimeResponseRequest struct {
	mock.Mock
}

func (_m *MockRuntimeResponseRequest) BodyReader() io.Reader {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for BodyReader")
	}

	var r0 io.Reader
	if rf, ok := ret.Get(0).(func() io.Reader); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.Reader)
		}
	}

	return r0
}

func (_m *MockRuntimeResponseRequest) ContentType() string {
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

func (_m *MockRuntimeResponseRequest) InvokeID() string {
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

func (_m *MockRuntimeResponseRequest) ParsingError() model.AppError {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ParsingError")
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

func (_m *MockRuntimeResponseRequest) ResponseMode() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ResponseMode")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockRuntimeResponseRequest) TrailerError() ErrorForInvoker {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for TrailerError")
	}

	var r0 ErrorForInvoker
	if rf, ok := ret.Get(0).(func() ErrorForInvoker); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(ErrorForInvoker)
		}
	}

	return r0
}

func NewMockRuntimeResponseRequest(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockRuntimeResponseRequest {
	mock := &MockRuntimeResponseRequest{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
