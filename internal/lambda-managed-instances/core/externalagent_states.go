// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"log/slog"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type ExternalAgentState interface {
	Register([]Event) error
	Ready() error
	InitError(errorType model.ErrorType) error
	ExitError(errorType model.ErrorType) error
	ShutdownFailed() error
	Exited() error
	LaunchError(model.ErrorType) error
	Name() string
}

type ExternalAgentStartedState struct {
	disallowEverything
	agent    *ExternalAgent
	initFlow InitFlowSynchronization
}

func (s *ExternalAgentStartedState) Register(events []Event) error {
	for _, e := range events {
		if err := s.agent.subscribeUnsafe(e); err != nil {
			return err
		}
	}
	s.agent.setStateUnsafe(s.agent.RegisteredState)
	if err := s.initFlow.ExternalAgentRegistered(); err != nil {
		slog.Error("External agent registration failed", "err", err)
	}
	return nil
}

func (s *ExternalAgentStartedState) LaunchError(err model.ErrorType) error {
	s.agent.setStateUnsafe(s.agent.LaunchErrorState)
	s.agent.errorType = err
	return nil
}

func (s *ExternalAgentStartedState) Name() string {
	return AgentStartedStateName
}

type ExternalAgentRegisteredState struct {
	disallowEverything
	agent    *ExternalAgent
	initFlow InitFlowSynchronization
}

func (s *ExternalAgentRegisteredState) Ready() error {
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

func (s *ExternalAgentRegisteredState) InitError(errorType model.ErrorType) error {
	s.agent.setStateUnsafe(s.agent.InitErrorState)
	s.agent.errorType = errorType
	return nil
}

func (s *ExternalAgentRegisteredState) ExitError(errorType model.ErrorType) error {
	s.agent.setStateUnsafe(s.agent.ExitErrorState)
	s.agent.errorType = errorType
	return nil
}

func (s *ExternalAgentRegisteredState) Name() string {
	return AgentRegisteredStateName
}

type ExternalAgentReadyState struct {
	disallowEverything
	agent *ExternalAgent
}

func (s *ExternalAgentReadyState) ExitError(errorType model.ErrorType) error {
	s.agent.setStateUnsafe(s.agent.ExitErrorState)
	s.agent.errorType = errorType
	return nil
}

func (s *ExternalAgentReadyState) Name() string {
	return AgentReadyStateName
}

type ExternalAgentRunningState struct {
	disallowEverything
	agent *ExternalAgent
}

func (s *ExternalAgentRunningState) Ready() error {
	s.agent.setStateUnsafe(s.agent.ReadyState)
	s.agent.ManagedThread.SuspendUnsafe()

	if s.agent.currentState != s.agent.ReadyState {
		return ErrConcurrentStateModification
	}
	s.agent.setStateUnsafe(s.agent.RunningState)

	return nil
}

func (s *ExternalAgentRunningState) ExitError(errorType model.ErrorType) error {
	s.agent.setStateUnsafe(s.agent.ExitErrorState)
	s.agent.errorType = errorType
	return nil
}

func (s *ExternalAgentRunningState) ShutdownFailed() error {
	s.agent.setStateUnsafe(s.agent.ShutdownFailedState)
	return nil
}

func (s *ExternalAgentRunningState) Exited() error {
	s.agent.setStateUnsafe(s.agent.ExitedState)
	return nil
}

func (s *ExternalAgentRunningState) Name() string {
	return AgentRunningStateName
}

type ExternalAgentInitErrorState struct {
	disallowEverything
}

func (s *ExternalAgentInitErrorState) Name() string {
	return AgentInitErrorStateName
}

func (s *ExternalAgentInitErrorState) InitError(errorType model.ErrorType) error {

	return nil
}

type ExternalAgentExitErrorState struct {
	disallowEverything
}

func (s *ExternalAgentExitErrorState) Name() string {
	return AgentExitErrorStateName
}

func (s *ExternalAgentExitErrorState) ExitError(errorType model.ErrorType) error {

	return nil
}

type ExternalAgentShutdownFailedState struct {
	disallowEverything
}

func (s *ExternalAgentShutdownFailedState) Name() string {
	return AgentShutdownFailedStateName
}

type ExternalAgentExitedState struct {
	disallowEverything
}

func (s *ExternalAgentExitedState) Name() string {
	return AgentExitedStateName
}

type ExternalAgentLaunchErrorState struct {
	disallowEverything
}

func (s *ExternalAgentLaunchErrorState) Name() string {
	return AgentLaunchErrorName
}
