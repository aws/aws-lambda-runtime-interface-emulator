// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/testdata/mockthread"
)

func TestRuntimeInitErrorAfterReady(t *testing.T) {
	initFlow := &mockInitFlowSynchronization{}
	initFlow.ReadyCond = sync.NewCond(&sync.Mutex{})
	runtime := NewRuntime(initFlow)

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

	assert.Equal(t, runtime.RuntimeStartedState, runtime.GetState())

	runtime.SetState(runtime.RuntimeStartedState)
	assert.NoError(t, runtime.InitError())
	assert.Equal(t, runtime.RuntimeInitErrorState, runtime.GetState())

	runtime.SetState(runtime.RuntimeStartedState)
	assert.NoError(t, runtime.Ready())
	assert.Equal(t, runtime.RuntimeRunningState, runtime.GetState())
}

func TestRuntimeStateTransitionsFromInitErrorState(t *testing.T) {
	runtime := newRuntime()

	runtime.SetState(runtime.RuntimeInitErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeInitErrorState, runtime.GetState())

	runtime.SetState(runtime.RuntimeInitErrorState)
	assert.Equal(t, ErrNotAllowed, runtime.Ready())
	assert.Equal(t, runtime.RuntimeInitErrorState, runtime.GetState())
}

func TestRuntimeStateTransitionsFromReadyState(t *testing.T) {
	runtime := newRuntime()

	runtime.SetState(runtime.RuntimeReadyState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeReadyState, runtime.GetState())

	runtime.SetState(runtime.RuntimeReadyState)
	assert.NoError(t, runtime.Ready())
	assert.Equal(t, runtime.RuntimeRunningState, runtime.GetState())
}

func TestRuntimeStateTransitionsFromRunningState(t *testing.T) {
	runtime := newRuntime()

	runtime.SetState(runtime.RuntimeRunningState)
	assert.Equal(t, ErrNotAllowed, runtime.InitError())
	assert.Equal(t, runtime.RuntimeRunningState, runtime.GetState())

	runtime.SetState(runtime.RuntimeRunningState)
	assert.NoError(t, runtime.Ready())
	assert.Equal(t, runtime.RuntimeRunningState, runtime.GetState())
}

func newRuntime() *Runtime {
	initFlow := &mockInitFlowSynchronization{}
	runtime := NewRuntime(initFlow)
	runtime.ManagedThread = &mockthread.MockManagedThread{}

	return runtime
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

func (s *mockInitFlowSynchronization) AwaitExternalAgentsRegistered(ctx context.Context) error {
	return nil
}

func (s *mockInitFlowSynchronization) ExternalAgentRegistered() error {
	return nil
}

func (s *mockInitFlowSynchronization) AwaitRuntimeReady(ctx context.Context) error {
	return nil
}

func (s *mockInitFlowSynchronization) AwaitAgentsReady(ctx context.Context) error {
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
