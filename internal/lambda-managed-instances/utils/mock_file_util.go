// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	fs "io/fs"

	mock "github.com/stretchr/testify/mock"
)

type MockFileUtil struct {
	mock.Mock
}

func (_m *MockFileUtil) IsNotExist(err error) bool {
	ret := _m.Called(err)

	if len(ret) == 0 {
		panic("no return value specified for IsNotExist")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func(error) bool); ok {
		r0 = rf(err)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

func (_m *MockFileUtil) ReadDirectory(dirPath string) ([]fs.DirEntry, error) {
	ret := _m.Called(dirPath)

	if len(ret) == 0 {
		panic("no return value specified for ReadDirectory")
	}

	var r0 []fs.DirEntry
	var r1 error
	if rf, ok := ret.Get(0).(func(string) ([]fs.DirEntry, error)); ok {
		return rf(dirPath)
	}
	if rf, ok := ret.Get(0).(func(string) []fs.DirEntry); ok {
		r0 = rf(dirPath)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]fs.DirEntry)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(dirPath)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *MockFileUtil) Stat(name string) (fs.FileInfo, error) {
	ret := _m.Called(name)

	if len(ret) == 0 {
		panic("no return value specified for Stat")
	}

	var r0 fs.FileInfo
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (fs.FileInfo, error)); ok {
		return rf(name)
	}
	if rf, ok := ret.Get(0).(func(string) fs.FileInfo); ok {
		r0 = rf(name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(fs.FileInfo)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func NewMockFileUtil(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockFileUtil {
	mock := &MockFileUtil{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
