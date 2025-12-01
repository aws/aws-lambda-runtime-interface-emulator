// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invariant

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestInvariantViolationError(t *testing.T) {
	msg := "oops violation message"
	err := ViolationError{Statement: msg}
	assert.ErrorContains(t, err, msg)
}

func TestGlobalChecksDoNothingOnOk(t *testing.T) {
	defer SetViolationExecutor(std.executor)

	m := &mockViolationExecutor{}
	SetViolationExecutor(m)

	Check(true, "oops")
	Checkf(true, "oops with arg %v", 42)

	assert.Empty(t, m.Calls)
	m.AssertExpectations(t)
}

func TestGlobalViolationsUseProvidedImplementation(t *testing.T) {
	defer SetViolationExecutor(std.executor)

	m := &mockViolationExecutor{}
	SetViolationExecutor(m)

	{
		m.On("Exec", ViolationError{Statement: "oops check"}).Once()
		Check(false, "oops check")
		m.AssertExpectations(t)
	}
	{
		m.On("Exec", ViolationError{Statement: "oops check with arg 42"}).Once()
		Checkf(false, "oops check with arg %v", 42)
		m.AssertExpectations(t)
	}
	{
		m.On("Exec", ViolationError{Statement: "oops violate"}).Once()
		Violate("oops violate")
		m.AssertExpectations(t)
	}
	{
		m.On("Exec", ViolationError{Statement: "oops violate with arg 42"}).Once()
		Violatef("oops violate with arg %v", 42)
		m.AssertExpectations(t)
	}
}

func TestGlobalImplementationIsOfExpectedType(t *testing.T) {
	var valueOfExpectedType *PanicViolationExecutor
	assert.IsType(t, valueOfExpectedType, std.executor)

	violator := NewPanicViolationExecuter()
	assert.IsType(t, valueOfExpectedType, violator)
}

type mockViolationExecutor struct {
	mock.Mock
}

var _ ViolationExecutor = (*mockViolationExecutor)(nil)

func (m *mockViolationExecutor) Exec(err ViolationError) {
	m.Called(err)
}
