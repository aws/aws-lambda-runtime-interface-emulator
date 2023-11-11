// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/testdata/mockthread"
)

func TestRuntimeInitErrorAfterReady(t *testing.T) {
	initFlow := &mockInitFlowSynchronization{}
	initFlow.ReadyCond = sync.NewCond(&sync.Mutex{})
	invokeFlow := &mockInvokeFlowSynchronization{}
	runtime := NewRuntime(initFlow, invokeFlow)

	readyChan := make(chan struct{})
	runtime.SetState(runtime.RuntimeStartedState)
	go func() {
		assert.NoError(t, runtime.Ready())
		readyChan <- struct{}{}
	}()

	initFlow.ReadyCond.L.Lock()
	for !initFlow.ReadyCalled {
		initFlow.ReadyCond.Wait()
	}
	initFlow.ReadyCond.L.Unlock()
	assert.Equal(t, runtime.RuntimeReadyState, runtime.GetState())

	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	runtime.Release()
	<-readyChan
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeRunningState, runtime.GetState())
}

func TestRuntimeStateTransitionsFromStartedState(t *testing.T) {
	runtime := newRuntime()
	// Started
	assert.Equal(t, runtime.RuntimeStartedState, runtime.GetState())
	// Started -> InitError
	runtime.SetState(runtime.RuntimeStartedState)
	assert.NoError(t, runtime.InitError())
	assert.Equal(t, runtime.RuntimeInitErrorState, runtime.GetState())
	// Started -> Ready
	runtime.SetState(runtime.RuntimeStartedState)
	assert.NoError(t, runtime.Ready())
	assert.Equal(t, runtime.RuntimeRunningState, runtime.GetState())
	// Started -> RestoreReady
	runtime.SetState(runtime.RuntimeStartedState)
	assert.NoError(t, runtime.RestoreReady())
	assert.Equal(t, runtime.RuntimeRestoringState, runtime.GetState())
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
	runtime := newRuntime()
	// InitError -> InitError
	runtime.SetState(runtime.RuntimeInitErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeInitErrorState, runtime.GetState())
	// InitError -> Ready
	runtime.SetState(runtime.RuntimeInitErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.Ready())
	assert.Equal(t, runtime.RuntimeInitErrorState, runtime.GetState())
	// InitError -> RestoreReady
	runtime.SetState(runtime.RuntimeInitErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.RestoreReady())
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

func TestRuntimeStateTransitionsFromRestoreErrorState(t *testing.T) {
	runtime := newRuntime()
	// RestoreError -> InitError
	runtime.SetState(runtime.RuntimeRestoreErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeRestoreErrorState, runtime.GetState())
	// RestoreError -> Ready
	runtime.SetState(runtime.RuntimeRestoreErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.Ready())
	assert.Equal(t, runtime.RuntimeRestoreErrorState, runtime.GetState())
	// RestoreError -> RestoreReady
	runtime.SetState(runtime.RuntimeRestoreErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.RestoreReady())
	assert.Equal(t, runtime.RuntimeRestoreErrorState, runtime.GetState())
	// RestoreError -> ResponseSent
	runtime.SetState(runtime.RuntimeRestoreErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.ResponseSent())
	assert.Equal(t, runtime.RuntimeRestoreErrorState, runtime.GetState())
	// RestoreError -> InvocationResponse
	runtime.SetState(runtime.RuntimeRestoreErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationResponse())
	assert.Equal(t, runtime.RuntimeRestoreErrorState, runtime.GetState())
	// RestoreError -> InvocationErrorResponse
	runtime.SetState(runtime.RuntimeRestoreErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationErrorResponse())
	assert.Equal(t, runtime.RuntimeRestoreErrorState, runtime.GetState())
}

func TestRuntimeStateTransitionsFromReadyState(t *testing.T) {
	runtime := newRuntime()
	// Ready -> InitError
	runtime.SetState(runtime.RuntimeReadyState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeReadyState, runtime.GetState())
	// Ready -> Ready
	runtime.SetState(runtime.RuntimeReadyState)
	assert.NoError(t, runtime.Ready())
	assert.Equal(t, runtime.RuntimeRunningState, runtime.GetState())
	// Ready -> RestoreReady
	runtime.SetState(runtime.RuntimeReadyState)
	assert.Equal(t, ErrNotAllowed, runtime.RestoreReady())
	assert.Equal(t, runtime.RuntimeReadyState, runtime.GetState())
	// Ready -> ResponseSent
	runtime.SetState(runtime.RuntimeReadyState)
	assert.Equal(t, ErrNotAllowed, runtime.ResponseSent())
	assert.Equal(t, runtime.RuntimeReadyState, runtime.GetState())
	// Ready -> InvocationResponse
	runtime.SetState(runtime.RuntimeReadyState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationResponse())
	assert.Equal(t, runtime.RuntimeReadyState, runtime.GetState())
	// Ready -> InvocationErrorResponse
	runtime.SetState(runtime.RuntimeReadyState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationErrorResponse())
	assert.Equal(t, runtime.RuntimeReadyState, runtime.GetState())
}

func TestRuntimeStateTransitionsFromRunningState(t *testing.T) {
	runtime := newRuntime()
	// Running -> InitError
	runtime.SetState(runtime.RuntimeRunningState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeRunningState, runtime.GetState())
	// Running -> Ready
	runtime.SetState(runtime.RuntimeRunningState)
	assert.NoError(t, runtime.Ready())
	assert.Equal(t, runtime.RuntimeRunningState, runtime.GetState())
	// Running -> RestoreReady
	runtime.SetState(runtime.RuntimeRunningState)
	assert.Equal(t, ErrNotAllowed, runtime.RestoreReady())
	assert.Equal(t, runtime.RuntimeRunningState, runtime.GetState())
	// Running -> ResponseSent
	runtime.SetState(runtime.RuntimeRunningState)
	assert.Equal(t, ErrNotAllowed, runtime.ResponseSent())
	assert.Equal(t, runtime.RuntimeRunningState, runtime.GetState())
	// Running -> InvocationResponse
	runtime.SetState(runtime.RuntimeRunningState)
	assert.NoError(t, runtime.InvocationResponse())
	assert.Equal(t, runtime.RuntimeInvocationResponseState, runtime.GetState())
	// Running -> InvocationErrorResponse
	runtime.SetState(runtime.RuntimeRunningState)
	assert.NoError(t, runtime.InvocationErrorResponse())
	assert.Equal(t, runtime.RuntimeInvocationErrorResponseState, runtime.GetState())
}

func TestRuntimeStateTransitionsFromInvocationResponseState(t *testing.T) {
	runtime := newRuntime()
	// InvocationResponse -> InitError
	runtime.SetState(runtime.RuntimeInvocationResponseState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeInvocationResponseState, runtime.GetState())
	// InvocationResponse -> Ready
	runtime.SetState(runtime.RuntimeInvocationResponseState)
	assert.Equal(t, ErrNotAllowed, runtime.Ready())
	assert.Equal(t, runtime.RuntimeInvocationResponseState, runtime.GetState())
	// InvocationResponse -> RestoreReady
	runtime.SetState(runtime.RuntimeInvocationResponseState)
	assert.Equal(t, ErrNotAllowed, runtime.RestoreReady())
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
	runtime := newRuntime()
	// InvocationErrorResponse -> InitError
	runtime.SetState(runtime.RuntimeInvocationErrorResponseState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeInvocationErrorResponseState, runtime.GetState())
	// InvocationErrorResponse -> Ready
	runtime.SetState(runtime.RuntimeInvocationErrorResponseState)
	assert.Equal(t, ErrNotAllowed, runtime.Ready())
	assert.Equal(t, runtime.RuntimeInvocationErrorResponseState, runtime.GetState())
	// InvocationErrorResponse -> RestoreReady
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
	runtime := newRuntime()
	// ResponseSent -> InitError
	runtime.SetState(runtime.RuntimeResponseSentState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeResponseSentState, runtime.GetState())
	// ResponseSent -> Ready
	runtime.SetState(runtime.RuntimeResponseSentState)
	assert.NoError(t, runtime.Ready())
	assert.Equal(t, runtime.RuntimeRunningState, runtime.GetState())
	// ResponseSent -> RestoreReady
	runtime.SetState(runtime.RuntimeResponseSentState)
	assert.Equal(t, ErrNotAllowed, runtime.RestoreReady())
	assert.Equal(t, runtime.RuntimeResponseSentState, runtime.GetState())
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

func TestRuntimeStateTransitionsFromRestoreReadyState(t *testing.T) {
	runtime := newRuntime()
	// RestoreReady -> InitError
	runtime.SetState(runtime.RuntimeRestoreReadyState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeRestoreReadyState, runtime.GetState())
	// RestoreReady -> Ready
	runtime.SetState(runtime.RuntimeRestoreReadyState)
	assert.Equal(t, ErrNotAllowed, runtime.Ready())
	assert.Equal(t, runtime.RuntimeRestoreReadyState, runtime.GetState())
	// RestoreReady -> RestoreReady()
	runtime.SetState(runtime.RuntimeRestoreReadyState)
	assert.Equal(t, ErrNotAllowed, runtime.RestoreReady())
	assert.Equal(t, runtime.RuntimeRestoreReadyState, runtime.GetState())
	// RestoreReady -> ResponseSent
	runtime.SetState(runtime.RuntimeRestoreReadyState)
	assert.Equal(t, ErrNotAllowed, runtime.ResponseSent())
	assert.Equal(t, runtime.RuntimeRestoreReadyState, runtime.GetState())
	// RestoreReady -> InvocationResponse
	runtime.SetState(runtime.RuntimeRestoreReadyState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationResponse())
	assert.Equal(t, runtime.RuntimeRestoreReadyState, runtime.GetState())
	// RestoreReady -> InvocationErrorResponse
	runtime.SetState(runtime.RuntimeRestoreReadyState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationErrorResponse())
	assert.Equal(t, runtime.RuntimeRestoreReadyState, runtime.GetState())
}

func TestRuntimeStateTransitionsFromRestoringState(t *testing.T) {
	runtime, mockInitFlow, _ := newRuntimeGetMockFlows()
	runtime.SetState(runtime.RuntimeRestoringState)
	mockInitFlow.On("CancelWithError", interop.ErrRestoreHookUserError{UserError: interop.FunctionError{}}).Return()
	// RestoreRunning -> Ready
	runtime.SetState(runtime.RuntimeRestoringState)
	assert.NoError(t, runtime.Ready())
	assert.Equal(t, runtime.RuntimeRunningState, runtime.GetState())
	// RestoreRunning -> RestoreReady
	runtime.SetState(runtime.RuntimeRestoringState)
	assert.Equal(t, ErrNotAllowed, runtime.RestoreReady())
	assert.Equal(t, runtime.RuntimeRestoringState, runtime.GetState())
	// RestoreRunning -> ResponseSent
	runtime.SetState(runtime.RuntimeRestoringState)
	assert.Equal(t, ErrNotAllowed, runtime.ResponseSent())
	assert.Equal(t, runtime.RuntimeRestoringState, runtime.GetState())
	// RestoreRunning -> InvocationResponse
	runtime.SetState(runtime.RuntimeRestoringState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationResponse())
	assert.Equal(t, runtime.RuntimeRestoringState, runtime.GetState())
	// RestoreRunning -> InvocationErrorResponse
	runtime.SetState(runtime.RuntimeRestoringState)
	assert.Equal(t, ErrNotAllowed, runtime.InvocationErrorResponse())
	assert.Equal(t, runtime.RuntimeRestoringState, runtime.GetState())
	// RestoreRunning -> RestoreError
	runtime.SetState(runtime.RuntimeRestoringState)
	assert.NoError(t, runtime.RestoreError(interop.FunctionError{}))
	assert.Equal(t, runtime.RuntimeRestoreErrorState, runtime.GetState())
}

func newRuntime() *Runtime {
	initFlow := &mockInitFlowSynchronization{}
	invokeFlow := &mockInvokeFlowSynchronization{}
	runtime := NewRuntime(initFlow, invokeFlow)
	runtime.ManagedThread = &mockthread.MockManagedThread{}

	return runtime
}

func newRuntimeGetMockFlows() (*Runtime, *mockInitFlowSynchronization, *mockInvokeFlowSynchronization) {
	initFlow := &mockInitFlowSynchronization{}
	invokeFlow := &mockInvokeFlowSynchronization{}
	runtime := NewRuntime(initFlow, invokeFlow)
	runtime.ManagedThread = &mockthread.MockManagedThread{}

	return runtime, initFlow, invokeFlow
}

type mockInitFlowSynchronization struct {
	mock.Mock
	ReadyCond   *sync.Cond
	ReadyCalled bool
}

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
func (s *mockInitFlowSynchronization) AwaitRuntimeReadyWithDeadline(ctx context.Context) error {
	return nil
}
func (s *mockInitFlowSynchronization) AwaitAgentsReady() error {
	return nil
}
func (s *mockInitFlowSynchronization) RuntimeReady() error {
	if s.ReadyCond != nil {
		s.ReadyCond.L.Lock()
		defer s.ReadyCond.L.Unlock()
		s.ReadyCalled = true
		s.ReadyCond.Signal()
	}
	return nil
}
func (s *mockInitFlowSynchronization) AgentReady() error {
	return nil
}
func (s *mockInitFlowSynchronization) CancelWithError(err error) {
	s.Called(err)
}
func (s *mockInitFlowSynchronization) Clear() {}
func (s *mockInitFlowSynchronization) RuntimeRestoreReady() error {
	return nil
}
func (s *mockInitFlowSynchronization) AwaitRuntimeRestoreReady() error {
	return nil
}

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
