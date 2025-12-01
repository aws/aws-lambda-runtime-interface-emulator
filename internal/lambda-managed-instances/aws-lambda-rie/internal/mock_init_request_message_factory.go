// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	mock "github.com/stretchr/testify/mock"
	model "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"

	utils "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils"
)

type MockInitRequestMessageFactory struct {
	mock.Mock
}

func (_m *MockInitRequestMessageFactory) Execute(fileUtil utils.FileUtil, args []string) (model.InitRequestMessage, rapidmodel.AppError) {
	ret := _m.Called(fileUtil, args)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 model.InitRequestMessage
	var r1 rapidmodel.AppError
	if rf, ok := ret.Get(0).(func(utils.FileUtil, []string) (model.InitRequestMessage, rapidmodel.AppError)); ok {
		return rf(fileUtil, args)
	}
	if rf, ok := ret.Get(0).(func(utils.FileUtil, []string) model.InitRequestMessage); ok {
		r0 = rf(fileUtil, args)
	} else {
		r0 = ret.Get(0).(model.InitRequestMessage)
	}

	if rf, ok := ret.Get(1).(func(utils.FileUtil, []string) rapidmodel.AppError); ok {
		r1 = rf(fileUtil, args)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(rapidmodel.AppError)
		}
	}

	return r0, r1
}

func NewMockInitRequestMessageFactory(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockInitRequestMessageFactory {
	mock := &MockInitRequestMessageFactory{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
