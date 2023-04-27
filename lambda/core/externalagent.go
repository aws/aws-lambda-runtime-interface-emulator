// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"time"

	"go.amzn.com/lambda/core/statejson"

	"github.com/google/uuid"
)

// ExternalAgent represents external agent
type ExternalAgent struct {
	Name   string
	ID     uuid.UUID
	events map[Event]struct{}

	ManagedThread Suspendable

	currentState      ExternalAgentState
	stateLastModified time.Time

	StartedState        ExternalAgentState
	RegisteredState     ExternalAgentState
	ReadyState          ExternalAgentState
	RunningState        ExternalAgentState
	InitErrorState      ExternalAgentState
	ExitErrorState      ExternalAgentState
	ShutdownFailedState ExternalAgentState
	ExitedState         ExternalAgentState
	LaunchErrorState    ExternalAgentState

	errorType string
}

// NewExternalAgent returns new instance of a named agent
func NewExternalAgent(name string, initFlow InitFlowSynchronization, invokeFlow InvokeFlowSynchronization) *ExternalAgent {
	agent := &ExternalAgent{
		Name:          name,
		ID:            uuid.New(),
		ManagedThread: NewManagedThread(),
		events:        make(map[Event]struct{}),
	}

	agent.StartedState = &ExternalAgentStartedState{agent: agent, initFlow: initFlow}
	agent.RegisteredState = &ExternalAgentRegisteredState{agent: agent, initFlow: initFlow}
	agent.ReadyState = &ExternalAgentReadyState{agent: agent}
	agent.RunningState = &ExternalAgentRunningState{agent: agent, invokeFlow: invokeFlow}
	agent.InitErrorState = &ExternalAgentInitErrorState{}
	agent.ExitErrorState = &ExternalAgentExitErrorState{}
	agent.ShutdownFailedState = &ExternalAgentShutdownFailedState{}
	agent.ExitedState = &ExternalAgentExitedState{}
	agent.LaunchErrorState = &ExternalAgentLaunchErrorState{}

	agent.setStateUnsafe(agent.StartedState)

	return agent
}

func (s *ExternalAgent) String() string {
	return fmt.Sprintf("%s (%s)", s.Name, s.ID)
}

// SuspendUnsafe the current running thread
func (s *ExternalAgent) SuspendUnsafe() {
	s.ManagedThread.SuspendUnsafe()
}

// Release will resume a suspended thread
func (s *ExternalAgent) Release() {
	s.ManagedThread.Release()
}

// SetState using the lock
func (s *ExternalAgent) SetState(state ExternalAgentState) {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	s.setStateUnsafe(state)
}

func (s *ExternalAgent) setStateUnsafe(state ExternalAgentState) {
	s.currentState = state
	s.stateLastModified = time.Now()
}

func ValidateExternalAgentEvent(e Event) error {
	switch e {
	case InvokeEvent:
		return nil
	case ShutdownEvent:
		return nil
	}
	return errInvalidEventType
}

func (s *ExternalAgent) subscribeUnsafe(e Event) error {
	if err := ValidateExternalAgentEvent(e); err != nil {
		return err
	}
	s.events[e] = struct{}{}
	return nil
}

// IsSubscribed checks whether agent is subscribed the Event
func (s *ExternalAgent) IsSubscribed(e Event) bool {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	_, found := s.events[e]
	return found
}

// SubscribedEvents returns events to which the agent is subscribed
func (s *ExternalAgent) SubscribedEvents() []string {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()

	events := []string{}
	for event := range s.events {
		events = append(events, string(event))
	}
	return events
}

// GetState returns agent's current state
func (s *ExternalAgent) GetState() ExternalAgentState {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState
}

// Register an agent with the platform
func (s *ExternalAgent) Register(events []Event) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.Register(events)
}

// Ready - mark an agent as ready
func (s *ExternalAgent) Ready() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.Ready()
}

// InitError - agent registered but failed to initialize
func (s *ExternalAgent) InitError(errorType string) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.InitError(errorType)
}

// ExitError - agent reported unrecoverable error
func (s *ExternalAgent) ExitError(errorType string) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.ExitError(errorType)
}

// ShutdownFailed - terminal state, agent didn't exit gracefully
func (s *ExternalAgent) ShutdownFailed() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.ShutdownFailed()
}

// Exited - agent shut down successfully
func (s *ExternalAgent) Exited() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.Exited()
}

// ErrorType returns error type reported during init or exit
func (s *ExternalAgent) ErrorType() string {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.errorType
}

// Exited - agent shut down successfully
func (s *ExternalAgent) LaunchError(err error) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.LaunchError(err)
}

// GetAgentDescription returns agent description object for debugging purposes
func (s *ExternalAgent) GetAgentDescription() statejson.ExtensionDescription {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return statejson.ExtensionDescription{
		Name: s.Name,
		ID:   s.ID.String(),
		State: statejson.StateDescription{
			Name:         s.currentState.Name(),
			LastModified: s.stateLastModified.UnixNano() / int64(time.Millisecond),
		},
		ErrorType: s.errorType,
	}
}
