// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	io "io"
	http "net/http"

	mock "github.com/stretchr/testify/mock"

	time "time"

	model "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type MockInvokeRequest struct {
	mock.Mock
}

func (_m *MockInvokeRequest) AddResponseHeader(_a0 string, _a1 string) {
	_m.Called(_a0, _a1)
}

func (_m *MockInvokeRequest) BodyReader() io.Reader {
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

func (_m *MockInvokeRequest) ClientContext() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ClientContext")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockInvokeRequest) CognitoId() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for CognitoId")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockInvokeRequest) CognitoPoolId() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for CognitoPoolId")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockInvokeRequest) ContentType() string {
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

func (_m *MockInvokeRequest) Deadline() time.Time {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Deadline")
	}

	var r0 time.Time
	if rf, ok := ret.Get(0).(func() time.Time); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(time.Time)
	}

	return r0
}

func (_m *MockInvokeRequest) FunctionVersionID() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for FunctionVersionID")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockInvokeRequest) InvokeID() string {
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

func (_m *MockInvokeRequest) MaxPayloadSize() int64 {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for MaxPayloadSize")
	}

	var r0 int64
	if rf, ok := ret.Get(0).(func() int64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int64)
	}

	return r0
}

func (_m *MockInvokeRequest) ResponseBandwidthBurstRate() int64 {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ResponseBandwidthBurstRate")
	}

	var r0 int64
	if rf, ok := ret.Get(0).(func() int64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int64)
	}

	return r0
}

func (_m *MockInvokeRequest) ResponseBandwidthRate() int64 {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ResponseBandwidthRate")
	}

	var r0 int64
	if rf, ok := ret.Get(0).(func() int64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int64)
	}

	return r0
}

func (_m *MockInvokeRequest) ResponseMode() string {
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

func (_m *MockInvokeRequest) ResponseWriter() http.ResponseWriter {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ResponseWriter")
	}

	var r0 http.ResponseWriter
	if rf, ok := ret.Get(0).(func() http.ResponseWriter); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(http.ResponseWriter)
		}
	}

	return r0
}

func (_m *MockInvokeRequest) SetResponseHeader(_a0 string, _a1 string) {
	_m.Called(_a0, _a1)
}

func (_m *MockInvokeRequest) TraceId() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for TraceId")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockInvokeRequest) UpdateFromInitData(_a0 InitStaticDataProvider) model.AppError {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for UpdateFromInitData")
	}

	var r0 model.AppError
	if rf, ok := ret.Get(0).(func(InitStaticDataProvider) model.AppError); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(model.AppError)
		}
	}

	return r0
}

func (_m *MockInvokeRequest) WriteResponseHeaders(_a0 int) {
	_m.Called(_a0)
}

func NewMockInvokeRequest(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockInvokeRequest {
	mock := &MockInvokeRequest{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
