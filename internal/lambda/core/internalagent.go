// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/core/statejson"

	"github.com/google/uuid"
)

// InternalAgent represents internal agent
type InternalAgent struct {
	Name   string
	ID     uuid.UUID
	events map[Event]struct{}

	ManagedThread Suspendable

	currentState      InternalAgentState
	stateLastModified time.Time

	StartedState    InternalAgentState
	RegisteredState InternalAgentState
	RunningState    InternalAgentState
	ReadyState      InternalAgentState
	InitErrorState  InternalAgentState
	ExitErrorState  InternalAgentState

	errorType string
}

// NewInternalAgent returns new instance of a named agent
func NewInternalAgent(name string, initFlow InitFlowSynchronization, invokeFlow InvokeFlowSynchronization) *InternalAgent {
	agent := &InternalAgent{
		Name:          name,
		ID:            uuid.New(),
		ManagedThread: NewManagedThread(),
		events:        make(map[Event]struct{}),
	}

	agent.StartedState = &InternalAgentStartedState{agent: agent}
	agent.RegisteredState = &InternalAgentRegisteredState{agent: agent, initFlow: initFlow}
	agent.RunningState = &InternalAgentRunningState{agent: agent, invokeFlow: invokeFlow}
	agent.ReadyState = &InternalAgentReadyState{agent: agent}
	agent.InitErrorState = &InternalAgentInitErrorState{}
	agent.ExitErrorState = &InternalAgentExitErrorState{}

	agent.setStateUnsafe(agent.StartedState)

	return agent
}

func (s *InternalAgent) String() string {
	return fmt.Sprintf("%s (%s)", s.Name, s.ID)
}

// SuspendUnsafe the current running thread
func (s *InternalAgent) SuspendUnsafe() {
	s.ManagedThread.SuspendUnsafe()
}

// Release will resume a suspended thread
func (s *InternalAgent) Release() {
	s.ManagedThread.Release()
}

// SetState using the lock
func (s *InternalAgent) SetState(state InternalAgentState) {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	s.setStateUnsafe(state)
}

func (s *InternalAgent) setStateUnsafe(state InternalAgentState) {
	s.currentState = state
	s.stateLastModified = time.Now()
}

func ValidateInternalAgentEvent(e Event) error {
	switch e {
	case InvokeEvent:
		return nil
	case ShutdownEvent:
		return errEventNotSupportedForInternalAgent
	}
	return errInvalidEventType
}

func (s *InternalAgent) subscribeUnsafe(e Event) error {
	if err := ValidateInternalAgentEvent(e); err != nil {
		return err
	}
	s.events[e] = struct{}{}
	return nil
}

// IsSubscribed checks whether agent is subscribed the Event
func (s *InternalAgent) IsSubscribed(e Event) bool {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	_, found := s.events[e]
	return found
}

// SubscribedEvents returns events to which the agent is subscribed
func (s *InternalAgent) SubscribedEvents() []string {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()

	events := []string{}
	for event := range s.events {
		events = append(events, string(event))
	}
	return events
}

// GetState returns agent's current state
func (s *InternalAgent) GetState() InternalAgentState {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState
}

// Register an agent with the platform
func (s *InternalAgent) Register(events []Event) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.Register(events)
}

// Ready - mark an agent as ready
func (s *InternalAgent) Ready() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.Ready()
}

// InitError - agent registered but failed to initialize
func (s *InternalAgent) InitError(errorType string) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.InitError(errorType)
}

// ExitError - agent registered but failed to initialize
func (s *InternalAgent) ExitError(errorType string) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.ExitError(errorType)
}

// ErrorType returns error type reported during init or exit
func (s *InternalAgent) ErrorType() string {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.errorType
}

// GetAgentDescription returns agent description object for debugging purposes
func (s *InternalAgent) GetAgentDescription() statejson.ExtensionDescription {
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
