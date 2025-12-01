// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"log/slog"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type InternalAgentState interface {
	Register([]Event) error
	Ready() error
	InitError(errorType model.ErrorType) error
	ExitError(errorType model.ErrorType) error
	Name() string
}

type InternalAgentStartedState struct {
	disallowEverything
	agent *InternalAgent
}

func (s *InternalAgentStartedState) Register(events []Event) error {
	s.agent.setStateUnsafe(s.agent.RegisteredState)
	return nil
}

func (s *InternalAgentStartedState) Name() string {
	return AgentStartedStateName
}

type InternalAgentRegisteredState struct {
	disallowEverything
	agent    *InternalAgent
	initFlow InitFlowSynchronization
}

func (s *InternalAgentRegisteredState) Ready() error {
	s.agent.setStateUnsafe(s.agent.ReadyState)
	if err := s.initFlow.AgentReady(); err != nil {
		slog.Error("Agent ready failed", "err", err)
	}
	s.agent.ManagedThread.SuspendUnsafe()

	if s.agent.currentState != s.agent.ReadyState {
		return ErrConcurrentStateModification
	}
	s.agent.setStateUnsafe(s.agent.RunningState)

	return nil
}

func (s *InternalAgentRegisteredState) InitError(errorType model.ErrorType) error {
	s.agent.setStateUnsafe(s.agent.InitErrorState)
	s.agent.errorType = errorType
	return nil
}

func (s *InternalAgentRegisteredState) ExitError(errorType model.ErrorType) error {
	s.agent.setStateUnsafe(s.agent.ExitErrorState)
	s.agent.errorType = errorType
	return nil
}

func (s *InternalAgentRegisteredState) Name() string {
	return AgentRegisteredStateName
}

type InternalAgentReadyState struct {
	disallowEverything
	agent *InternalAgent
}

func (s *InternalAgentReadyState) ExitError(errorType model.ErrorType) error {
	s.agent.setStateUnsafe(s.agent.ExitErrorState)
	s.agent.errorType = errorType
	return nil
}

func (s *InternalAgentReadyState) Name() string {
	return AgentReadyStateName
}

type InternalAgentRunningState struct {
	disallowEverything
	agent *InternalAgent
}

func (s *InternalAgentRunningState) Ready() error {
	s.agent.setStateUnsafe(s.agent.ReadyState)
	s.agent.ManagedThread.SuspendUnsafe()

	if s.agent.currentState != s.agent.ReadyState {
		return ErrConcurrentStateModification
	}
	s.agent.setStateUnsafe(s.agent.RunningState)

	return nil
}

func (s *InternalAgentRunningState) ExitError(errorType model.ErrorType) error {
	s.agent.setStateUnsafe(s.agent.ExitErrorState)
	s.agent.errorType = errorType
	return nil
}

func (s *InternalAgentRunningState) Name() string {
	return AgentRunningStateName
}

type InternalAgentInitErrorState struct {
	disallowEverything
}

func (s *InternalAgentInitErrorState) Name() string {
	return AgentInitErrorStateName
}

func (s *InternalAgentInitErrorState) InitError(errorType model.ErrorType) error {

	return nil
}

type InternalAgentExitErrorState struct {
	disallowEverything
}

func (s *InternalAgentExitErrorState) Name() string {
	return AgentExitErrorStateName
}

func (s *InternalAgentExitErrorState) ExitError(errorType model.ErrorType) error {

	return nil
}
