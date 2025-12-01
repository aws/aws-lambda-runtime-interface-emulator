// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import mock "github.com/stretchr/testify/mock"

type MockEventsAPI struct {
	mock.Mock
}

func (_m *MockEventsAPI) Flush() {
	_m.Called()
}

func (_m *MockEventsAPI) SendExtensionInit(_a0 ExtensionInitData) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for SendExtensionInit")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(ExtensionInitData) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *MockEventsAPI) SendImageError(_a0 ImageErrorLogData) {
	_m.Called(_a0)
}

func (_m *MockEventsAPI) SendInitReport(_a0 InitReportData) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for SendInitReport")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(InitReportData) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *MockEventsAPI) SendInitRuntimeDone(_a0 InitRuntimeDoneData) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for SendInitRuntimeDone")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(InitRuntimeDoneData) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *MockEventsAPI) SendInitStart(_a0 InitStartData) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for SendInitStart")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(InitStartData) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *MockEventsAPI) SendInternalXRayErrorCause(_a0 InternalXRayErrorCauseData) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for SendInternalXRayErrorCause")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(InternalXRayErrorCauseData) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *MockEventsAPI) SendInvokeStart(_a0 InvokeStartData) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for SendInvokeStart")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(InvokeStartData) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *MockEventsAPI) SendReport(_a0 ReportData) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for SendReport")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(ReportData) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func NewMockEventsAPI(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockEventsAPI {
	mock := &MockEventsAPI{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
