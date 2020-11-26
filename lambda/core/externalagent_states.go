// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

// ExternalAgentState is external agent state interface
type ExternalAgentState interface {
	Register([]Event) error
	Ready() error
	InitError(errorType string) error
	ExitError(errorType string) error
	ShutdownFailed() error
	Exited() error
	LaunchError(error) error
	Name() string
}

// ExternalAgentStartedState is the initial state of an external agent
type ExternalAgentStartedState struct {
	disallowEverything
	agent    *ExternalAgent
	initFlow InitFlowSynchronization
}

// Register an agent with the platform when agent is in started state
func (s *ExternalAgentStartedState) Register(events []Event) error {
	for _, e := range events {
		if err := s.agent.subscribeUnsafe(e); err != nil {
			return err
		}
	}
	s.agent.setStateUnsafe(s.agent.RegisteredState)
	s.initFlow.ExternalAgentRegistered()
	return nil
}

// LaunchError signals that agent could not launch (non-exec/permission denied)
func (s *ExternalAgentStartedState) LaunchError(err error) error {
	s.agent.setStateUnsafe(s.agent.LaunchErrorState)
	s.agent.errorType = string(MapErrorToAgentInfoErrorType(err))
	return nil
}

// Name return state's human friendly name
func (s *ExternalAgentStartedState) Name() string {
	return AgentStartedStateName
}

// ExternalAgentRegisteredState is the state of an agent that registered with the platform but has not reported ready (next)
type ExternalAgentRegisteredState struct {
	disallowEverything
	agent    *ExternalAgent
	initFlow InitFlowSynchronization
}

// Ready - agent has called next and is now successfully initialized
func (s *ExternalAgentRegisteredState) Ready() error {
	s.agent.setStateUnsafe(s.agent.ReadyState)
	s.initFlow.AgentReady()
	s.agent.ManagedThread.SuspendUnsafe()

	if s.agent.currentState != s.agent.ReadyState {
		return ErrConcurrentStateModification
	}
	s.agent.setStateUnsafe(s.agent.RunningState)

	return nil
}

// InitError - agent can transitions to InitErrorState if it failed to initialize
func (s *ExternalAgentRegisteredState) InitError(errorType string) error {
	s.agent.setStateUnsafe(s.agent.InitErrorState)
	s.agent.errorType = errorType
	return nil
}

// ExitError - agent called /exit/error
func (s *ExternalAgentRegisteredState) ExitError(errorType string) error {
	s.agent.setStateUnsafe(s.agent.ExitErrorState)
	s.agent.errorType = errorType
	return nil
}

// Name return state's human friendly name
func (s *ExternalAgentRegisteredState) Name() string {
	return AgentRegisteredStateName
}

// ExternalAgentReadyState is the state of an agent that reported ready to the platform
type ExternalAgentReadyState struct {
	disallowEverything
	agent *ExternalAgent
}

// ExitError signals that agent provided unrecoverable error description
func (s *ExternalAgentReadyState) ExitError(errorType string) error {
	s.agent.setStateUnsafe(s.agent.ExitErrorState)
	s.agent.errorType = errorType
	return nil
}

// Name return state's human friendly name
func (s *ExternalAgentReadyState) Name() string {
	return AgentReadyStateName
}

// ExternalAgentRunningState is the state of an agent that has received an invoke event and is currently processing it
type ExternalAgentRunningState struct {
	disallowEverything
	agent      *ExternalAgent
	invokeFlow InvokeFlowSynchronization
}

// Ready - agent transitions to Ready and the calling thread gets suspended. Upon release the agent transitions to Running
func (s *ExternalAgentRunningState) Ready() error {
	s.agent.setStateUnsafe(s.agent.ReadyState)
	s.invokeFlow.AgentReady()
	s.agent.ManagedThread.SuspendUnsafe()

	if s.agent.currentState != s.agent.ReadyState {
		return ErrConcurrentStateModification
	}
	s.agent.setStateUnsafe(s.agent.RunningState)

	return nil
}

// ExitError signals that agent provided unrecoverable error description
func (s *ExternalAgentRunningState) ExitError(errorType string) error {
	s.agent.setStateUnsafe(s.agent.ExitErrorState)
	s.agent.errorType = errorType
	return nil
}

// ShutdownFailed transitions agent into the ShutdownFailed terminal state
func (s *ExternalAgentRunningState) ShutdownFailed() error {
	s.agent.setStateUnsafe(s.agent.ShutdownFailedState)
	return nil
}

// Exited - agent process has exited
func (s *ExternalAgentRunningState) Exited() error {
	s.agent.setStateUnsafe(s.agent.ExitedState)
	return nil
}

// Name return state's human friendly name
func (s *ExternalAgentRunningState) Name() string {
	return AgentRunningStateName
}

// ExternalAgentInitErrorState is a terminal state where agent has reported /init/error
type ExternalAgentInitErrorState struct {
	disallowEverything
}

// Name return state's human friendly name
func (s *ExternalAgentInitErrorState) Name() string {
	return AgentInitErrorStateName
}

// InitError - multiple calls are allowed, but only the first submitted error is accepted
func (s *ExternalAgentInitErrorState) InitError(errorType string) error {
	// no-op
	return nil
}

// ExternalAgentExitErrorState is a terminal state where agent has reported /exit/error
type ExternalAgentExitErrorState struct {
	disallowEverything
}

// Name return state's human friendly name
func (s *ExternalAgentExitErrorState) Name() string {
	return AgentExitErrorStateName
}

// ExitError - multiple calls are allowed, but only the first submitted error is accepted
func (s *ExternalAgentExitErrorState) ExitError(errorType string) error {
	// no-op
	return nil
}

type ExternalAgentShutdownFailedState struct {
	disallowEverything
}

// Name return state's human friendly name
func (s *ExternalAgentShutdownFailedState) Name() string {
	return AgentShutdownFailedStateName
}

type ExternalAgentExitedState struct {
	disallowEverything
}

// Name return state's human friendly name
func (s *ExternalAgentExitedState) Name() string {
	return AgentExitedStateName
}

type ExternalAgentLaunchErrorState struct {
	disallowEverything
}

// Name return state's human friendly name
func (s *ExternalAgentLaunchErrorState) Name() string {
	return AgentLaunchErrorName
}
