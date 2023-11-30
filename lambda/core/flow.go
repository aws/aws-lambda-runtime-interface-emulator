// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"

	"go.amzn.com/lambda/interop"
)

// InitFlowSynchronization wraps init flow barriers.
type InitFlowSynchronization interface {
	SetExternalAgentsRegisterCount(uint16) error
	SetAgentsReadyCount(uint16) error

	ExternalAgentRegistered() error
	AwaitExternalAgentsRegistered() error

	RuntimeReady() error
	AwaitRuntimeReady() error
	AwaitRuntimeReadyWithDeadline(context.Context) error

	AgentReady() error
	AwaitAgentsReady() error

	CancelWithError(error)

	RuntimeRestoreReady() error
	AwaitRuntimeRestoreReady() error

	Clear()
}

type initFlowSynchronizationImpl struct {
	externalAgentsRegisteredGate Gate
	runtimeReadyGate             Gate
	agentReadyGate               Gate
	runtimeRestoreReadyGate      Gate
}

// SetExternalAgentsRegisterCount notifies init flow that N /extension/register calls should be done in future by external agents
func (s *initFlowSynchronizationImpl) SetExternalAgentsRegisterCount(externalAgentsNumber uint16) error {
	return s.externalAgentsRegisteredGate.SetCount(externalAgentsNumber)
}

// SetAgentsReadyCount sets the number of agents we expect at the gate once we know it
func (s *initFlowSynchronizationImpl) SetAgentsReadyCount(agentCount uint16) error {
	return s.agentReadyGate.SetCount(agentCount)
}

// AwaitRuntimeReady awaits runtime ready state
func (s *initFlowSynchronizationImpl) AwaitRuntimeReady() error {
	return s.runtimeReadyGate.AwaitGateCondition()
}

func (s *initFlowSynchronizationImpl) AwaitRuntimeReadyWithDeadline(ctx context.Context) error {
	var err error
	errorChan := make(chan error)

	go func() {
		errorChan <- s.runtimeReadyGate.AwaitGateCondition()
	}()

	select {
	case err = <-errorChan:
		break
	case <-ctx.Done():
		err = interop.ErrRestoreHookTimeout
		s.CancelWithError(err)
		break
	}

	return err
}

// AwaitRuntimeRestoreReady awaits runtime restore ready state (/restore/next is called by runtime)
func (s *initFlowSynchronizationImpl) AwaitRuntimeRestoreReady() error {
	return s.runtimeRestoreReadyGate.AwaitGateCondition()
}

// AwaitExternalAgentsRegistered awaits for all subscribed agents to report registered
func (s *initFlowSynchronizationImpl) AwaitExternalAgentsRegistered() error {
	return s.externalAgentsRegisteredGate.AwaitGateCondition()
}

// AwaitAgentReady awaits for registered extensions to report ready
func (s *initFlowSynchronizationImpl) AwaitAgentsReady() error {
	return s.agentReadyGate.AwaitGateCondition()
}

// Ready called by runtime when initialized
func (s *initFlowSynchronizationImpl) RuntimeReady() error {
	return s.runtimeReadyGate.WalkThrough()
}

// Ready called by runtime when restore is completed (i.e. /next is called after /restore/next)
func (s *initFlowSynchronizationImpl) RuntimeRestoreReady() error {
	return s.runtimeRestoreReadyGate.WalkThrough()
}

// Ready called by agent when initialized
func (s *initFlowSynchronizationImpl) AgentReady() error {
	return s.agentReadyGate.WalkThrough()
}

// ExternalAgentRegistered called by agent as part of /register request
func (s *initFlowSynchronizationImpl) ExternalAgentRegistered() error {
	return s.externalAgentsRegisteredGate.WalkThrough()
}

// Cancel cancels gates with error.
func (s *initFlowSynchronizationImpl) CancelWithError(err error) {
	s.externalAgentsRegisteredGate.CancelWithError(err)
	s.runtimeReadyGate.CancelWithError(err)
	s.agentReadyGate.CancelWithError(err)
	s.runtimeRestoreReadyGate.CancelWithError(err)
}

// Clear gates state
func (s *initFlowSynchronizationImpl) Clear() {
	s.externalAgentsRegisteredGate.Clear()
	s.runtimeReadyGate.Clear()
	s.agentReadyGate.Clear()
	s.runtimeRestoreReadyGate.Clear()
}

// NewInitFlowSynchronization returns new InitFlowSynchronization instance.
func NewInitFlowSynchronization() InitFlowSynchronization {
	initFlow := &initFlowSynchronizationImpl{
		runtimeReadyGate:             NewGate(1),
		externalAgentsRegisteredGate: NewGate(0),
		agentReadyGate:               NewGate(maxAgentsLimit),
		runtimeRestoreReadyGate:      NewGate(1),
	}
	return initFlow
}

// InvokeFlowSynchronization wraps invoke flow barriers.
type InvokeFlowSynchronization interface {
	InitializeBarriers() error
	AwaitRuntimeResponse() error
	AwaitRuntimeReady() error
	RuntimeResponse(runtime *Runtime) error
	RuntimeReady(runtime *Runtime) error
	SetAgentsReadyCount(agentCount uint16) error
	AgentReady() error
	AwaitAgentsReady() error
	CancelWithError(error)
	Clear()
}

type invokeFlowSynchronizationImpl struct {
	runtimeReadyGate    Gate
	runtimeResponseGate Gate
	agentReadyGate      Gate
}

// InitializeBarriers ...
func (s *invokeFlowSynchronizationImpl) InitializeBarriers() error {
	s.runtimeReadyGate.Reset()
	s.runtimeResponseGate.Reset()
	s.agentReadyGate.Reset()
	return nil
}

// Clear gates state
func (s *invokeFlowSynchronizationImpl) Clear() {
	s.runtimeReadyGate.Clear()
	s.runtimeResponseGate.Clear()
	s.agentReadyGate.Clear()
}

// AwaitRuntimeResponse awaits runtime to send response to the platform.
func (s *invokeFlowSynchronizationImpl) AwaitRuntimeResponse() error {
	return s.runtimeResponseGate.AwaitGateCondition()
}

// AwaitRuntimeReady awaits runtime ready state.
func (s *invokeFlowSynchronizationImpl) AwaitRuntimeReady() error {
	return s.runtimeReadyGate.AwaitGateCondition()
}

// RuntimeResponse called by runtime  when runtime response is made available to the platform.
func (s *invokeFlowSynchronizationImpl) RuntimeResponse(a *Runtime) error {
	return s.runtimeResponseGate.WalkThrough()
}

// RuntimeReady called by runtime  when runtime ready.
func (s *invokeFlowSynchronizationImpl) RuntimeReady(a *Runtime) error {
	return s.runtimeReadyGate.WalkThrough()
}

// Cancel cancels gates.
func (s *invokeFlowSynchronizationImpl) CancelWithError(err error) {
	s.runtimeResponseGate.CancelWithError(err)
	s.runtimeReadyGate.CancelWithError(err)
	s.agentReadyGate.CancelWithError(err)
}

// SetAgentsReadyCount sets the number of agents we expect at the gate once we know it
func (s *invokeFlowSynchronizationImpl) SetAgentsReadyCount(agentCount uint16) error {
	return s.agentReadyGate.SetCount(agentCount)
}

// Ready called by agent when initialized
func (s *invokeFlowSynchronizationImpl) AgentReady() error {
	return s.agentReadyGate.WalkThrough()
}

// AwaitAgentReady awaits for registered extensions to report ready
func (s *invokeFlowSynchronizationImpl) AwaitAgentsReady() error {
	return s.agentReadyGate.AwaitGateCondition()
}

// NewInvokeFlowSynchronization returns new InvokeFlowSynchronization instance.
func NewInvokeFlowSynchronization() InvokeFlowSynchronization {
	return &invokeFlowSynchronizationImpl{
		runtimeReadyGate:    NewGate(1),
		runtimeResponseGate: NewGate(1),
		agentReadyGate:      NewGate(maxAgentsLimit),
	}
}
