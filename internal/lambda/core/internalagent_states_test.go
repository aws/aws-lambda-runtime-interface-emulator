// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"testing"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/testdata/mockthread"
	"github.com/stretchr/testify/require"
)

func TestInternalAgentStateUnknownEventType(t *testing.T) {
	agent := NewInternalAgent("name", &mockInitFlowSynchronization{}, &mockInvokeFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	require.Equal(t, agent.StartedState, agent.GetState())
	require.Equal(t, errInvalidEventType, agent.Register([]Event{"foo"}))
	require.Equal(t, agent.StartedState, agent.GetState())
}

func TestInternalAgentStateInvalidEventType(t *testing.T) {
	agent := NewInternalAgent("name", &mockInitFlowSynchronization{}, &mockInvokeFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	require.Equal(t, agent.StartedState, agent.GetState())
	require.Equal(t, errEventNotSupportedForInternalAgent, agent.Register([]Event{ShutdownEvent}))
	require.Equal(t, agent.StartedState, agent.GetState())
}

func TestInternalAgentStateTransitionsFromStartedState(t *testing.T) {
	agent := NewInternalAgent("name", &mockInitFlowSynchronization{}, &mockInvokeFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	// Initial agent state is Start
	require.Equal(t, agent.StartedState, agent.GetState())
	require.NoError(t, agent.Register([]Event{}))
	require.Equal(t, agent.RegisteredState, agent.GetState())

	agent.SetState(agent.StartedState)
	require.Equal(t, ErrNotAllowed, agent.Ready())
	require.Equal(t, agent.StartedState, agent.GetState())

	require.Equal(t, ErrNotAllowed, agent.InitError("Extension.TestError"))
	require.Equal(t, agent.StartedState, agent.GetState())

	require.Equal(t, ErrNotAllowed, agent.ExitError("Extension.TestError"))
	require.Equal(t, agent.StartedState, agent.GetState())
}

func TestInternalAgentStateTransitionsFromRegisteredState(t *testing.T) {
	agent := NewInternalAgent("name", &mockInitFlowSynchronization{}, &mockInvokeFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	agent.SetState(agent.RegisteredState)

	require.Equal(t, ErrNotAllowed, agent.Register([]Event{}))
	require.Equal(t, agent.RegisteredState, agent.GetState())

	require.NoError(t, agent.Ready())
	require.Equal(t, agent.RunningState, agent.GetState())

	agent.SetState(agent.RegisteredState)
	require.NoError(t, agent.InitError("Extension.TestError"))
	require.Equal(t, agent.InitErrorState, agent.GetState())
	require.Equal(t, "Extension.TestError", agent.errorType)

	agent.SetState(agent.RegisteredState)
	require.NoError(t, agent.ExitError("Extension.TestError"))
	require.Equal(t, agent.ExitErrorState, agent.GetState())
	require.Equal(t, "Extension.TestError", agent.errorType)
}

func TestInternalAgentStateTransitionsFromReadyState(t *testing.T) {
	agent := NewInternalAgent("name", &mockInitFlowSynchronization{}, &mockInvokeFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	agent.SetState(agent.ReadyState)

	require.Equal(t, ErrNotAllowed, agent.Register([]Event{}))
	require.Equal(t, agent.ReadyState, agent.GetState())

	require.Equal(t, ErrNotAllowed, agent.InitError("Extension.TestError"))
	require.Equal(t, agent.ReadyState, agent.GetState())

	agent.SetState(agent.ReadyState)
	require.NoError(t, agent.ExitError("Extension.TestError"))
	require.Equal(t, agent.ExitErrorState, agent.GetState())
	require.Equal(t, "Extension.TestError", agent.errorType)

	agent.SetState(agent.ReadyState)
	require.Equal(t, ErrNotAllowed, agent.Ready())
	require.Equal(t, agent.ReadyState, agent.GetState())
}

func TestInternalAgentStateTransitionsFromInitErrorState(t *testing.T) {
	agent := NewInternalAgent("name", &mockInitFlowSynchronization{}, &mockInvokeFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	agent.SetState(agent.InitErrorState)

	require.Equal(t, ErrNotAllowed, agent.Register([]Event{}))
	require.Equal(t, agent.InitErrorState, agent.GetState())
	require.Equal(t, nil, agent.InitError("Extension.TestError")) // InitError -> InitError reentrancy
	require.Equal(t, agent.InitErrorState, agent.GetState())
	require.Equal(t, ErrNotAllowed, agent.ExitError("Extension.TestError"))
	require.Equal(t, agent.InitErrorState, agent.GetState())
	require.Equal(t, ErrNotAllowed, agent.Ready())
	require.Equal(t, agent.InitErrorState, agent.GetState())
}

func TestInternalAgentStateTransitionsFromExitErrorState(t *testing.T) {
	agent := NewInternalAgent("name", &mockInitFlowSynchronization{}, &mockInvokeFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	agent.SetState(agent.ExitErrorState)

	require.Equal(t, ErrNotAllowed, agent.Register([]Event{}))
	require.Equal(t, agent.ExitErrorState, agent.GetState())
	require.Equal(t, nil, agent.ExitError("Extension.TestError")) // ExitError -> ExitError reentrancy
	require.Equal(t, agent.ExitErrorState, agent.GetState())
	require.Equal(t, ErrNotAllowed, agent.InitError("Extension.TestError"))
	require.Equal(t, agent.ExitErrorState, agent.GetState())
	require.Equal(t, ErrNotAllowed, agent.Ready())
	require.Equal(t, agent.ExitErrorState, agent.GetState())
}

func TestInternalAgentStateTransitionsFromRunningState(t *testing.T) {
	agent := NewInternalAgent("name", &mockInitFlowSynchronization{}, &mockInvokeFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	agent.SetState(agent.RunningState)
	require.Equal(t, agent.RunningState, agent.GetState())

	require.Equal(t, ErrNotAllowed, agent.Register([]Event{}))
	require.Equal(t, agent.RunningState, agent.GetState())

	agent.SetState(agent.RunningState)
	require.NoError(t, agent.Ready())
	require.Equal(t, agent.RunningState, agent.GetState())
}
