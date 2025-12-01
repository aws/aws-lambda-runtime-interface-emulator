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

type InternalAgent struct {
	name string
	id   uuid.UUID

	ManagedThread Suspendable

	currentState      InternalAgentState
	stateLastModified time.Time

	StartedState    InternalAgentState
	RegisteredState InternalAgentState
	RunningState    InternalAgentState
	ReadyState      InternalAgentState
	InitErrorState  InternalAgentState
	ExitErrorState  InternalAgentState

	errorType model.ErrorType
}

func NewInternalAgent(name string, initFlow InitFlowSynchronization) *InternalAgent {
	agent := &InternalAgent{
		name:          name,
		id:            uuid.New(),
		ManagedThread: NewManagedThread(),
	}

	agent.StartedState = &InternalAgentStartedState{agent: agent}
	agent.RegisteredState = &InternalAgentRegisteredState{agent: agent, initFlow: initFlow}
	agent.RunningState = &InternalAgentRunningState{agent: agent}
	agent.ReadyState = &InternalAgentReadyState{agent: agent}
	agent.InitErrorState = &InternalAgentInitErrorState{}
	agent.ExitErrorState = &InternalAgentExitErrorState{}

	agent.setStateUnsafe(agent.StartedState)

	return agent
}

func (s *InternalAgent) Name() string {
	return s.name
}

func (s *InternalAgent) ID() uuid.UUID {
	return s.id
}

func (s InternalAgent) String() string {
	return fmt.Sprintf("%s (%s)", s.name, s.id)
}

func (s *InternalAgent) SuspendUnsafe() {
	s.ManagedThread.SuspendUnsafe()
}

func (s *InternalAgent) Release() {
	s.ManagedThread.Release()
}

func (s *InternalAgent) SetState(state InternalAgentState) {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	s.setStateUnsafe(state)
}

func (s *InternalAgent) setStateUnsafe(state InternalAgentState) {
	s.currentState = state
	s.stateLastModified = time.Now()
}

func (s *InternalAgent) GetState() InternalAgentState {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState
}

func (s *InternalAgent) Register(events []Event) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.Register(events)
}

func (s *InternalAgent) Ready() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.Ready()
}

func (s *InternalAgent) InitError(errorType model.ErrorType) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.InitError(errorType)
}

func (s *InternalAgent) ExitError(errorType model.ErrorType) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.ExitError(errorType)
}

func (s *InternalAgent) ErrorType() model.ErrorType {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.errorType
}

func (s *InternalAgent) GetAgentDescription() statejson.ExtensionDescription {
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
