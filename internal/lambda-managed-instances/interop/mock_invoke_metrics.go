// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	json "encoding/json"
	time "time"

	mock "github.com/stretchr/testify/mock"
	model "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type MockInvokeMetrics struct {
	mock.Mock
}

func (_m *MockInvokeMetrics) AttachDependencies(_a0 InitStaticDataProvider, _a1 EventsAPI) {
	_m.Called(_a0, _a1)
}

func (_m *MockInvokeMetrics) AttachInvokeRequest(_a0 InvokeRequest) {
	_m.Called(_a0)
}

func (_m *MockInvokeMetrics) SendInvokeFinishedEvent(tracingCtx *TracingCtx, xrayErrorCause json.RawMessage) error {
	ret := _m.Called(tracingCtx, xrayErrorCause)

	if len(ret) == 0 {
		panic("no return value specified for SendInvokeFinishedEvent")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*TracingCtx, json.RawMessage) error); ok {
		r0 = rf(tracingCtx, xrayErrorCause)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *MockInvokeMetrics) SendInvokeStartEvent(_a0 *TracingCtx) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for SendInvokeStartEvent")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*TracingCtx) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *MockInvokeMetrics) SendMetrics(_a0 model.AppError) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for SendMetrics")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(model.AppError) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *MockInvokeMetrics) TriggerGetRequest() {
	_m.Called()
}

func (_m *MockInvokeMetrics) TriggerGetResponse() {
	_m.Called()
}

func (_m *MockInvokeMetrics) TriggerInvokeDone() (time.Duration, *time.Duration, InitStaticDataProvider) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for TriggerInvokeDone")
	}

	var r0 time.Duration
	var r1 *time.Duration
	var r2 InitStaticDataProvider
	if rf, ok := ret.Get(0).(func() (time.Duration, *time.Duration, InitStaticDataProvider)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() time.Duration); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(time.Duration)
	}

	if rf, ok := ret.Get(1).(func() *time.Duration); ok {
		r1 = rf()
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*time.Duration)
		}
	}

	if rf, ok := ret.Get(2).(func() InitStaticDataProvider); ok {
		r2 = rf()
	} else {
		if ret.Get(2) != nil {
			r2 = ret.Get(2).(InitStaticDataProvider)
		}
	}

	return r0, r1, r2
}

func (_m *MockInvokeMetrics) TriggerSentRequest(bytes int64, requestPayloadReadDuration time.Duration, requestPayloadWriteDuration time.Duration) {
	_m.Called(bytes, requestPayloadReadDuration, requestPayloadWriteDuration)
}

func (_m *MockInvokeMetrics) TriggerSentResponse(runtimeResponseSent bool, responseErr model.AppError, streamingMetrics *InvokeResponseMetrics, errorPayloadSizeBytes int) {
	_m.Called(runtimeResponseSent, responseErr, streamingMetrics, errorPayloadSizeBytes)
}

func (_m *MockInvokeMetrics) TriggerStartRequest() {
	_m.Called()
}

func (_m *MockInvokeMetrics) UpdateConcurrencyMetrics(inflightInvokes int, idleRuntimesCount int) {
	_m.Called(inflightInvokes, idleRuntimesCount)
}

func NewMockInvokeMetrics(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockInvokeMetrics {
	mock := &MockInvokeMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
