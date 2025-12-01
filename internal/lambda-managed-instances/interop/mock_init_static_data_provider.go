// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	time "time"

	mock "github.com/stretchr/testify/mock"
	model "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
)

type MockInitStaticDataProvider struct {
	mock.Mock
}

func (_m *MockInitStaticDataProvider) AmiId() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for AmiId")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockInitStaticDataProvider) ArtefactType() model.ArtefactType {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ArtefactType")
	}

	var r0 model.ArtefactType
	if rf, ok := ret.Get(0).(func() model.ArtefactType); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(model.ArtefactType)
	}

	return r0
}

func (_m *MockInitStaticDataProvider) AvailabilityZoneId() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for AvailabilityZoneId")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockInitStaticDataProvider) FunctionARN() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for FunctionARN")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockInitStaticDataProvider) FunctionTimeout() time.Duration {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for FunctionTimeout")
	}

	var r0 time.Duration
	if rf, ok := ret.Get(0).(func() time.Duration); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(time.Duration)
	}

	return r0
}

func (_m *MockInitStaticDataProvider) FunctionVersion() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for FunctionVersion")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockInitStaticDataProvider) FunctionVersionID() string {
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

func (_m *MockInitStaticDataProvider) InitTimeout() time.Duration {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for InitTimeout")
	}

	var r0 time.Duration
	if rf, ok := ret.Get(0).(func() time.Duration); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(time.Duration)
	}

	return r0
}

func (_m *MockInitStaticDataProvider) LogGroup() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for LogGroup")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockInitStaticDataProvider) LogStream() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for LogStream")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockInitStaticDataProvider) MemorySizeMB() uint64 {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for MemorySizeMB")
	}

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

func (_m *MockInitStaticDataProvider) RuntimeVersion() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for RuntimeVersion")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

func (_m *MockInitStaticDataProvider) XRayTracingMode() model.XrayTracingMode {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for XRayTracingMode")
	}

	var r0 model.XrayTracingMode
	if rf, ok := ret.Get(0).(func() model.XrayTracingMode); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(model.XrayTracingMode)
	}

	return r0
}

func NewMockInitStaticDataProvider(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockInitStaticDataProvider {
	mock := &MockInitStaticDataProvider{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
