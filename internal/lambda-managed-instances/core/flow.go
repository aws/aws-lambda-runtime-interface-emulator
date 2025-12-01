// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
)

type InitFlowSynchronization interface {
	SetExternalAgentsRegisterCount(uint16) error
	SetAgentsReadyCount(uint16) error

	ExternalAgentRegistered() error
	AwaitExternalAgentsRegistered(context.Context) error

	RuntimeReady() error
	AwaitRuntimeReady(context.Context) error

	AgentReady() error
	AwaitAgentsReady(context.Context) error

	CancelWithError(error)

	Clear()
}

type initFlowSynchronizationImpl struct {
	externalAgentsRegisteredGate Gate
	runtimeReadyGate             Gate
	agentReadyGate               Gate
}

func (s *initFlowSynchronizationImpl) SetExternalAgentsRegisterCount(externalAgentsNumber uint16) error {
	return s.externalAgentsRegisteredGate.SetCount(externalAgentsNumber)
}

func (s *initFlowSynchronizationImpl) SetAgentsReadyCount(agentCount uint16) error {
	return s.agentReadyGate.SetCount(agentCount)
}

func (s *initFlowSynchronizationImpl) AwaitRuntimeReady(ctx context.Context) error {
	_, err := s.runtimeReadyGate.AwaitGateCondition(ctx)
	return err
}

func (s *initFlowSynchronizationImpl) AwaitExternalAgentsRegistered(ctx context.Context) error {
	_, err := s.externalAgentsRegisteredGate.AwaitGateCondition(ctx)
	return err
}

func (s *initFlowSynchronizationImpl) AwaitAgentsReady(ctx context.Context) error {
	_, err := s.agentReadyGate.AwaitGateCondition(ctx)
	return err
}

func (s *initFlowSynchronizationImpl) RuntimeReady() error {
	return s.runtimeReadyGate.WalkThrough(nil)
}

func (s *initFlowSynchronizationImpl) AgentReady() error {
	return s.agentReadyGate.WalkThrough(nil)
}

func (s *initFlowSynchronizationImpl) ExternalAgentRegistered() error {
	return s.externalAgentsRegisteredGate.WalkThrough(nil)
}

func (s *initFlowSynchronizationImpl) CancelWithError(err error) {
	s.externalAgentsRegisteredGate.CancelWithError(err)
	s.runtimeReadyGate.CancelWithError(err)
	s.agentReadyGate.CancelWithError(err)
}

func (s *initFlowSynchronizationImpl) Clear() {
	s.externalAgentsRegisteredGate.Clear()
	s.runtimeReadyGate.Clear()
	s.agentReadyGate.Clear()
}

func NewInitFlowSynchronization() InitFlowSynchronization {
	initFlow := &initFlowSynchronizationImpl{
		runtimeReadyGate:             NewGate(1),
		externalAgentsRegisteredGate: NewGate(0),
		agentReadyGate:               NewGate(maxAgentsLimit),
	}
	return initFlow
}
