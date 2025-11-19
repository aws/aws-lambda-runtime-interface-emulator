// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

// InternalAgentState is internal agent state interface
type InternalAgentState interface {
	Register([]Event) error
	Ready() error
	InitError(errorType string) error
	ExitError(errorType string) error
	Name() string
}

// InternalAgentStartedState is the initial state of an internal agent
type InternalAgentStartedState struct {
	disallowEverything
	agent *InternalAgent
}

// Register an agent with the platform when agent is in started state
func (s *InternalAgentStartedState) Register(events []Event) error {
	for _, e := range events {
		if err := s.agent.subscribeUnsafe(e); err != nil {
			return err
		}
	}
	s.agent.setStateUnsafe(s.agent.RegisteredState)
	return nil
}

// Name return state's human friendly name
func (s *InternalAgentStartedState) Name() string {
	return AgentStartedStateName
}

// InternalAgentRegisteredState is the state of an agent that registered with the platform but has not reported ready (next)
type InternalAgentRegisteredState struct {
	disallowEverything
	agent    *InternalAgent
	initFlow InitFlowSynchronization
}

// Ready - agent has called next and is now successfully initialized
func (s *InternalAgentRegisteredState) Ready() error {
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
func (s *InternalAgentRegisteredState) InitError(errorType string) error {
	s.agent.setStateUnsafe(s.agent.InitErrorState)
	s.agent.errorType = errorType
	return nil
}

// ExitError - agent called /exit/error
func (s *InternalAgentRegisteredState) ExitError(errorType string) error {
	s.agent.setStateUnsafe(s.agent.ExitErrorState)
	s.agent.errorType = errorType
	return nil
}

// Name return state's human friendly name
func (s *InternalAgentRegisteredState) Name() string {
	return AgentRegisteredStateName
}

// InternalAgentReadyState is the state of an agent that reported ready to the platform
type InternalAgentReadyState struct {
	disallowEverything
	agent *InternalAgent
}

// ExitError - agent called /exit/error
func (s *InternalAgentReadyState) ExitError(errorType string) error {
	s.agent.setStateUnsafe(s.agent.ExitErrorState)
	s.agent.errorType = errorType
	return nil
}

// Name return state's human friendly name
func (s *InternalAgentReadyState) Name() string {
	return AgentReadyStateName
}

// InternalAgentRunningState is the state of an agent that is currently processing an event
type InternalAgentRunningState struct {
	disallowEverything
	agent      *InternalAgent
	invokeFlow InvokeFlowSynchronization
}

// Ready - agent can transition from ready to ready
func (s *InternalAgentRunningState) Ready() error {
	s.agent.setStateUnsafe(s.agent.ReadyState)
	s.invokeFlow.AgentReady()
	s.agent.ManagedThread.SuspendUnsafe()

	if s.agent.currentState != s.agent.ReadyState {
		return ErrConcurrentStateModification
	}
	s.agent.setStateUnsafe(s.agent.RunningState)

	return nil
}

// ExitError - agent called /exit/error
func (s *InternalAgentRunningState) ExitError(errorType string) error {
	s.agent.setStateUnsafe(s.agent.ExitErrorState)
	s.agent.errorType = errorType
	return nil
}

// Name return state's human friendly name
func (s *InternalAgentRunningState) Name() string {
	return AgentRunningStateName
}

// InternalAgentInitErrorState is a terminal state where agent has reported /init/error
type InternalAgentInitErrorState struct {
	disallowEverything
}

// Name return state's human friendly name
func (s *InternalAgentInitErrorState) Name() string {
	return AgentInitErrorStateName
}

// InitError - multiple calls are allowed, but only the first submitted error is accepted
func (s *InternalAgentInitErrorState) InitError(errorType string) error {
	// no-op
	return nil
}

// InternalAgentExitErrorState is a terminal state where agent has reported /exit/error
type InternalAgentExitErrorState struct {
	disallowEverything
}

// Name return state's human friendly name
func (s *InternalAgentExitErrorState) Name() string {
	return AgentExitErrorStateName
}

// ExitError - multiple calls are allowed, but only the first submitted error is accepted
func (s *InternalAgentExitErrorState) ExitError(errorType string) error {
	// no-op
	return nil
}
