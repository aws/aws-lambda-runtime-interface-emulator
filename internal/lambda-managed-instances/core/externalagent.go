// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core/statejson"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type ExternalAgent struct {
	name   string
	id     uuid.UUID
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

	errorType model.ErrorType
}

func NewExternalAgent(name string, initFlow InitFlowSynchronization) *ExternalAgent {
	agent := &ExternalAgent{
		name:          name,
		id:            uuid.New(),
		ManagedThread: NewManagedThread(),
		events:        make(map[Event]struct{}),
	}

	agent.StartedState = &ExternalAgentStartedState{agent: agent, initFlow: initFlow}
	agent.RegisteredState = &ExternalAgentRegisteredState{agent: agent, initFlow: initFlow}
	agent.ReadyState = &ExternalAgentReadyState{agent: agent}
	agent.RunningState = &ExternalAgentRunningState{agent: agent}
	agent.InitErrorState = &ExternalAgentInitErrorState{}
	agent.ExitErrorState = &ExternalAgentExitErrorState{}
	agent.ShutdownFailedState = &ExternalAgentShutdownFailedState{}
	agent.ExitedState = &ExternalAgentExitedState{}
	agent.LaunchErrorState = &ExternalAgentLaunchErrorState{}

	agent.setStateUnsafe(agent.StartedState)

	return agent
}

func (s *ExternalAgent) Name() string {
	return s.name
}

func (s *ExternalAgent) ID() uuid.UUID {
	return s.id
}

func (s *ExternalAgent) String() string {
	return fmt.Sprintf("%s (%s)", s.name, s.id)
}

func (s *ExternalAgent) SuspendUnsafe() {
	s.ManagedThread.SuspendUnsafe()
}

func (s *ExternalAgent) Release() {
	s.ManagedThread.Release()
}

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
	if e == ShutdownEvent {
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

func (s *ExternalAgent) IsSubscribed(e Event) bool {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	_, found := s.events[e]
	return found
}

func (s *ExternalAgent) SubscribedEvents() []string {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()

	events := []string{}
	for event := range s.events {
		events = append(events, string(event))
	}
	return events
}

func (s *ExternalAgent) GetState() ExternalAgentState {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState
}

func (s *ExternalAgent) Register(events []Event) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.Register(events)
}

func (s *ExternalAgent) Ready() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.Ready()
}

func (s *ExternalAgent) InitError(errorType model.ErrorType) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.InitError(errorType)
}

func (s *ExternalAgent) ExitError(errorType model.ErrorType) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.ExitError(errorType)
}

func (s *ExternalAgent) ShutdownFailed() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.ShutdownFailed()
}

func (s *ExternalAgent) Exited() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.Exited()
}

func (s *ExternalAgent) ErrorType() model.ErrorType {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.errorType
}

func (s *ExternalAgent) LaunchError(err model.ErrorType) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.LaunchError(err)
}

func (s *ExternalAgent) GetAgentDescription() statejson.ExtensionDescription {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return statejson.ExtensionDescription{
		Name: s.name,
		ID:   s.id.String(),
		State: statejson.StateDescription{
			Name:         s.currentState.Name(),
			LastModified: s.stateLastModified,
		},
		ErrorType: string(s.errorType),
	}
}
