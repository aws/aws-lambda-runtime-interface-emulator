// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.amzn.com/lambda/testdata/mockthread"
	"testing"
)

func TestRuntimeStateTransitionsFromStartedState(t *testing.T) {
	initFlow := &mockInitFlowSynchronization{}
	invokeFlow := &mockInvokeFlowSynchronization{}
	runtime := NewRuntime(initFlow, invokeFlow)
	runtime.ManagedThread = &mockthread.MockManagedThread{}
	// Started
	assert.Equal(t, runtime.RuntimeStartedState, runtime.GetState())
	// Started -> InitError
	runtime.SetState(runtime.RuntimeStartedState)
	assert.NoError(t, runtime.InitError())
	assert.Equal(t, runtime.RuntimeInitErrorState, runtime.GetState())
	// Started -> Ready
	runtime.SetState(runtime.RuntimeStartedState)
	assert.NoError(t, runtime.Ready())
	assert.Equal(t, runtime.RuntimeReadyState, runtime.GetState())
	// Started -> ResponseSent
	runtime.SetState(runtime.RuntimeStartedState)
	assert.Equal(t, ErrNotAllowed, runtime.ResponseSent())
	assert.Equal(t, runtime.RuntimeStartedState, runtime.GetState())
	// Started -> InvocationResponse
	runtime.SetState(runtime.RuntimeStartedState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationResponse())
	assert.Equal(t, runtime.RuntimeStartedState, runtime.GetState())
	// Started -> InvocationErrorResponse
	runtime.SetState(runtime.RuntimeStartedState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationErrorResponse())
	assert.Equal(t, runtime.RuntimeStartedState, runtime.GetState())
}

func TestRuntimeStateTransitionsFromInitErrorState(t *testing.T) {
	initFlow := &mockInitFlowSynchronization{}
	invokeFlow := &mockInvokeFlowSynchronization{}
	runtime := NewRuntime(initFlow, invokeFlow)
	runtime.ManagedThread = &mockthread.MockManagedThread{}
	// InitError -> InitError
	runtime.SetState(runtime.RuntimeInitErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeInitErrorState, runtime.GetState())
	// InitError -> Ready
	runtime.SetState(runtime.RuntimeInitErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.Ready())
	assert.Equal(t, runtime.RuntimeInitErrorState, runtime.GetState())
	// InitError -> ResponseSent
	runtime.SetState(runtime.RuntimeInitErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.ResponseSent())
	assert.Equal(t, runtime.RuntimeInitErrorState, runtime.GetState())
	// InitError -> InvocationResponse
	runtime.SetState(runtime.RuntimeInitErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationResponse())
	assert.Equal(t, runtime.RuntimeInitErrorState, runtime.GetState())
	// InitError -> InvocationErrorResponse
	runtime.SetState(runtime.RuntimeInitErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationErrorResponse())
	assert.Equal(t, runtime.RuntimeInitErrorState, runtime.GetState())
}

func TestRuntimeStateTransitionsFromReadyState(t *testing.T) {
	initFlow := &mockInitFlowSynchronization{}
	invokeFlow := &mockInvokeFlowSynchronization{}
	runtime := NewRuntime(initFlow, invokeFlow)
	runtime.ManagedThread = &mockthread.MockManagedThread{}
	// Ready -> InitError
	runtime.SetState(runtime.RuntimeReadyState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeReadyState, runtime.GetState())
	// Ready -> Ready
	runtime.SetState(runtime.RuntimeReadyState)
	assert.NoError(t, runtime.Ready())
	assert.Equal(t, runtime.RuntimeReadyState, runtime.GetState())
	// Ready -> ResponseSent
	runtime.SetState(runtime.RuntimeReadyState)
	assert.Equal(t, ErrNotAllowed, runtime.ResponseSent())
	assert.Equal(t, runtime.RuntimeReadyState, runtime.GetState())
	// Ready -> InvocationResponse
	runtime.SetState(runtime.RuntimeReadyState)
	assert.NoError(t, runtime.InvocationResponse())
	assert.Equal(t, runtime.RuntimeInvocationResponseState, runtime.GetState())
	// Ready -> InvocationErrorResponse
	runtime.SetState(runtime.RuntimeReadyState)
	assert.NoError(t, runtime.InvocationErrorResponse())
	assert.Equal(t, runtime.RuntimeInvocationErrorResponseState, runtime.GetState())
}

func TestRuntimeStateTransitionsFromInvocationResponseState(t *testing.T) {
	initFlow := &mockInitFlowSynchronization{}
	invokeFlow := &mockInvokeFlowSynchronization{}
	runtime := NewRuntime(initFlow, invokeFlow)
	runtime.ManagedThread = &mockthread.MockManagedThread{}
	// InvocationResponse -> InitError
	runtime.SetState(runtime.RuntimeInvocationResponseState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeInvocationResponseState, runtime.GetState())
	// InvocationResponse -> Ready
	runtime.SetState(runtime.RuntimeInvocationResponseState)
	assert.Equal(t, ErrNotAllowed, runtime.Ready())
	assert.Equal(t, runtime.RuntimeInvocationResponseState, runtime.GetState())
	// InvocationResponse -> ResponseSent
	runtime.SetState(runtime.RuntimeInvocationResponseState)
	assert.NoError(t, runtime.ResponseSent())
	assert.Equal(t, runtime.RuntimeResponseSentState, runtime.GetState())
	assert.NotEqual(t, 0, runtime.GetRuntimeDescription().State.ResponseTimeNs)
	// InvocationResponse-> InvocationResponse
	runtime.SetState(runtime.RuntimeInvocationResponseState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationResponse())
	assert.Equal(t, runtime.RuntimeInvocationResponseState, runtime.GetState())
	// InvocationResponse -> InvocationErrorResponse
	runtime.SetState(runtime.RuntimeInvocationResponseState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationErrorResponse())
	assert.Equal(t, runtime.RuntimeInvocationResponseState, runtime.GetState())
}

func TestRuntimeStateTransitionsFromInvocationErrorResponseState(t *testing.T) {
	initFlow := &mockInitFlowSynchronization{}
	invokeFlow := &mockInvokeFlowSynchronization{}
	runtime := NewRuntime(initFlow, invokeFlow)
	runtime.ManagedThread = &mockthread.MockManagedThread{}
	// InvocationErrorResponse -> InitError
	runtime.SetState(runtime.RuntimeInvocationErrorResponseState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeInvocationErrorResponseState, runtime.GetState())
	// InvocationErrorResponse -> Ready
	runtime.SetState(runtime.RuntimeInvocationErrorResponseState)
	assert.Equal(t, ErrNotAllowed, runtime.Ready())
	assert.Equal(t, runtime.RuntimeInvocationErrorResponseState, runtime.GetState())
	// InvocationErrorResponse -> ResponseSent
	runtime.SetState(runtime.RuntimeInvocationErrorResponseState)
	assert.NoError(t, runtime.ResponseSent())
	assert.Equal(t, runtime.RuntimeResponseSentState, runtime.GetState())
	// InvocationErrorResponse -> InvocationResponse
	runtime.SetState(runtime.RuntimeInvocationErrorResponseState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationResponse())
	assert.Equal(t, runtime.RuntimeInvocationErrorResponseState, runtime.GetState())
	// InvocationErrorResponse -> InvocationErrorResponse
	runtime.SetState(runtime.RuntimeInvocationErrorResponseState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationErrorResponse())
	assert.Equal(t, runtime.RuntimeInvocationErrorResponseState, runtime.GetState())
}

func TestRuntimeStateTransitionsFromResponseSentState(t *testing.T) {
	initFlow := &mockInitFlowSynchronization{}
	invokeFlow := &mockInvokeFlowSynchronization{}
	runtime := NewRuntime(initFlow, invokeFlow)
	runtime.ManagedThread = &mockthread.MockManagedThread{}
	// ResponseSent -> InitError
	runtime.SetState(runtime.RuntimeResponseSentState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeResponseSentState, runtime.GetState())
	// ResponseSent -> Ready
	runtime.SetState(runtime.RuntimeResponseSentState)
	assert.NoError(t, runtime.Ready())
	assert.Equal(t, runtime.RuntimeReadyState, runtime.GetState())
	// ResponseSent -> ResponseSent
	runtime.SetState(runtime.RuntimeResponseSentState)
	assert.Equal(t, ErrNotAllowed, runtime.ResponseSent())
	assert.Equal(t, runtime.RuntimeResponseSentState, runtime.GetState())
	// ResponseSent -> InvocationResponse
	runtime.SetState(runtime.RuntimeResponseSentState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationResponse())
	assert.Equal(t, runtime.RuntimeResponseSentState, runtime.GetState())
	// ResponseSent -> InvocationErrorResponse
	runtime.SetState(runtime.RuntimeResponseSentState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationErrorResponse())
	assert.Equal(t, runtime.RuntimeResponseSentState, runtime.GetState())
}

type mockInitFlowSynchronization struct{ mock.Mock }

func (s *mockInitFlowSynchronization) SetExternalAgentsRegisterCount(agentCount uint16) error {
	return nil
}

func (s *mockInitFlowSynchronization) SetAgentsReadyCount(agentCount uint16) error {
	return nil
}

func (s *mockInitFlowSynchronization) AwaitExternalAgentsRegistered() error {
	return nil
}
func (s *mockInitFlowSynchronization) ExternalAgentRegistered() error {
	return nil
}
func (s *mockInitFlowSynchronization) AwaitRuntimeReady() error {
	return nil
}
func (s *mockInitFlowSynchronization) AwaitAgentsReady() error {
	return nil
}
func (s *mockInitFlowSynchronization) RuntimeReady() error {
	return nil
}
func (s *mockInitFlowSynchronization) AgentReady() error {
	return nil
}
func (s *mockInitFlowSynchronization) CancelWithError(err error) {
	s.Called(err)
}
func (s *mockInitFlowSynchronization) Clear() {}

type mockInvokeFlowSynchronization struct{ mock.Mock }

func (s *mockInvokeFlowSynchronization) InitializeBarriers() error {
	return nil
}
func (s *mockInvokeFlowSynchronization) AwaitRuntimeResponse() error {
	return nil
}
func (s *mockInvokeFlowSynchronization) AwaitRuntimeReady() error {
	return nil
}
func (s *mockInvokeFlowSynchronization) RuntimeResponse(runtime *Runtime) error {
	return nil
}
func (s *mockInvokeFlowSynchronization) RuntimeReady(runtime *Runtime) error {
	return nil
}
func (s *mockInvokeFlowSynchronization) SetAgentsReadyCount(agentCount uint16) error {
	return nil
}
func (s *mockInvokeFlowSynchronization) AwaitAgentsReady() error {
	return nil
}
func (s *mockInvokeFlowSynchronization) AgentReady() error {
	return nil
}
func (s *mockInvokeFlowSynchronization) CancelWithError(err error) {
	s.Called(err)
}
func (s *mockInvokeFlowSynchronization) Clear() {}
