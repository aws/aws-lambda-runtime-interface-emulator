// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	mock "github.com/stretchr/testify/mock"
	statejson "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core/statejson"
)

type MockInternalStateGetter struct {
	mock.Mock
}

func (_m *MockInternalStateGetter) Execute() statejson.InternalStateDescription {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 statejson.InternalStateDescription
	if rf, ok := ret.Get(0).(func() statejson.InternalStateDescription); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(statejson.InternalStateDescription)
	}

	return r0
}

func NewMockInternalStateGetter(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockInternalStateGetter {
	mock := &MockInternalStateGetter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
