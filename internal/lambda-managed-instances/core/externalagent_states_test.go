// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/testdata/mockthread"
)

func TestExternalAgentStateUnknownEventType(t *testing.T) {
	agent := NewExternalAgent("name", &mockInitFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	require.Equal(t, agent.StartedState, agent.GetState())
	require.Equal(t, errInvalidEventType, agent.Register([]Event{"foo"}))
	require.Equal(t, agent.StartedState, agent.GetState())
}

func TestExternalAgentStateTransitionsFromStartedState(t *testing.T) {
	agent := NewExternalAgent("name", &mockInitFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}

	require.Equal(t, agent.StartedState, agent.GetState())

	require.NoError(t, agent.Register([]Event{}))
	require.Equal(t, agent.RegisteredState, agent.GetState())
	agent.SetState(agent.StartedState)

	require.NoError(t, agent.LaunchError(model.ErrorAgentPermissionDenied))
	require.Equal(t, agent.LaunchErrorState, agent.GetState())
	agent.SetState(agent.StartedState)

	require.Equal(t, ErrNotAllowed, agent.Ready())
	require.Equal(t, agent.StartedState, agent.GetState())

	require.Equal(t, ErrNotAllowed, agent.InitError("Extension.TestError"))
	require.Equal(t, agent.StartedState, agent.GetState())

	require.Equal(t, ErrNotAllowed, agent.ExitError("Extension.TestError"))
	require.Equal(t, agent.StartedState, agent.GetState())

	require.Equal(t, ErrNotAllowed, agent.ShutdownFailed())
	require.Equal(t, agent.StartedState, agent.GetState())

	require.Equal(t, ErrNotAllowed, agent.Exited())
	require.Equal(t, agent.StartedState, agent.GetState())
}

func TestExternalAgentStateTransitionsFromRegisteredState(t *testing.T) {
	agent := NewExternalAgent("name", &mockInitFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	agent.SetState(agent.RegisteredState)

	require.Equal(t, ErrNotAllowed, agent.Register([]Event{}))
	require.Equal(t, agent.RegisteredState, agent.GetState())

	require.NoError(t, agent.Ready())
	require.Equal(t, agent.RunningState, agent.GetState())

	agent.SetState(agent.RegisteredState)
	require.NoError(t, agent.InitError("Extension.TestError"))
	require.Equal(t, agent.InitErrorState, agent.GetState())
	require.Equal(t, model.ErrorType("Extension.TestError"), agent.errorType)

	agent.SetState(agent.RegisteredState)
	require.NoError(t, agent.ExitError("Extension.TestError"))
	require.Equal(t, agent.ExitErrorState, agent.GetState())
	require.Equal(t, model.ErrorType("Extension.TestError"), agent.errorType)
}

func TestExternalAgentStateTransitionsFromReadyState(t *testing.T) {
	agent := NewExternalAgent("name", &mockInitFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	agent.SetState(agent.ReadyState)

	require.Equal(t, ErrNotAllowed, agent.Register([]Event{}))
	require.Equal(t, agent.ReadyState, agent.GetState())

	require.Equal(t, ErrNotAllowed, agent.Ready())
	require.Equal(t, agent.ReadyState, agent.GetState())

	require.Equal(t, ErrNotAllowed, agent.InitError("Extension.TestError"))
	require.Equal(t, agent.ReadyState, agent.GetState())

	agent.SetState(agent.ReadyState)
	require.NoError(t, agent.ExitError("Extension.TestError"))
	require.Equal(t, agent.ExitErrorState, agent.GetState())
	require.Equal(t, model.ErrorType("Extension.TestError"), agent.errorType)

	agent.SetState(agent.ReadyState)
	require.Equal(t, ErrNotAllowed, agent.Exited())
	require.Equal(t, agent.ReadyState, agent.GetState())

	require.Equal(t, ErrNotAllowed, agent.ShutdownFailed())
	require.Equal(t, agent.ReadyState, agent.GetState())
}

func assertAgentIsInFinalState(t *testing.T, agent *ExternalAgent) {
	initialState := agent.GetState()
	require.Equal(t, ErrNotAllowed, agent.Register([]Event{}))
	require.Equal(t, initialState, agent.GetState())
	require.Equal(t, ErrNotAllowed, agent.Ready())
	require.Equal(t, initialState, agent.GetState())
	require.Equal(t, ErrNotAllowed, agent.ShutdownFailed())
	require.Equal(t, initialState, agent.GetState())
	require.Equal(t, ErrNotAllowed, agent.Exited())
	require.Equal(t, initialState, agent.GetState())
	require.Equal(t, ErrNotAllowed, agent.LaunchError(model.ErrorAgentExtensionLaunch))
	require.Equal(t, initialState, agent.GetState())

	if agent.InitErrorState == initialState {
		require.Equal(t, nil, agent.InitError("Extension.TestError"))
	} else {
		require.Equal(t, ErrNotAllowed, agent.InitError("Extension.TestError"))
	}

	require.Equal(t, initialState, agent.GetState())

	if agent.ExitErrorState == initialState {
		require.Equal(t, nil, agent.ExitError("Extension.TestError"))
	} else {
		require.Equal(t, ErrNotAllowed, agent.ExitError("Extension.TestError"))
	}

	require.Equal(t, initialState, agent.GetState())
}

func TestExternalAgentStateTransitionsFromInitErrorState(t *testing.T) {
	agent := NewExternalAgent("name", &mockInitFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	agent.SetState(agent.InitErrorState)
	assertAgentIsInFinalState(t, agent)
}

func TestExternalAgentStateTransitionsFromExitErrorState(t *testing.T) {
	agent := NewExternalAgent("name", &mockInitFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	agent.SetState(agent.ExitErrorState)
	assertAgentIsInFinalState(t, agent)
}

func TestExternalAgentStateTransitionsFromShutdownFailedState(t *testing.T) {
	agent := NewExternalAgent("name", &mockInitFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	agent.SetState(agent.ShutdownFailedState)
	assertAgentIsInFinalState(t, agent)
}

func TestExternalAgentStateTransitionsFromExitedState(t *testing.T) {
	agent := NewExternalAgent("name", &mockInitFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	agent.SetState(agent.ExitedState)
	assertAgentIsInFinalState(t, agent)
}

func TestExternalAgentStateTransitionsFromRunningState(t *testing.T) {
	agent := NewExternalAgent("name", &mockInitFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	agent.SetState(agent.RunningState)
	require.Equal(t, agent.RunningState, agent.GetState())

	require.Equal(t, ErrNotAllowed, agent.Register([]Event{}))
	require.Equal(t, agent.RunningState, agent.GetState())

	require.Equal(t, ErrNotAllowed, agent.InitError("Extension.TestError"))
	require.Equal(t, agent.RunningState, agent.GetState())

	require.NoError(t, agent.ShutdownFailed())
	require.Equal(t, agent.ShutdownFailedState, agent.GetState())

	agent.SetState(agent.RunningState)
	require.NoError(t, agent.Exited())
	require.Equal(t, agent.ExitedState, agent.GetState())

	agent.SetState(agent.RunningState)
	require.NoError(t, agent.Ready())
	require.Equal(t, agent.RunningState, agent.GetState())
}

func TestExternalAgentStateTransitionsFromLaunchErrorState(t *testing.T) {
	agent := NewExternalAgent("name", &mockInitFlowSynchronization{})
	agent.ManagedThread = &mockthread.MockManagedThread{}
	agent.SetState(agent.LaunchErrorState)
	assertAgentIsInFinalState(t, agent)
}
