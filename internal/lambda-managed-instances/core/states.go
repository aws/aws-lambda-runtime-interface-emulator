// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"sync"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core/statejson"
)

type Suspendable interface {
	SuspendUnsafe()
	Release()
	Lock()
	Unlock()
}

type ManagedThread struct {
	operatorCondition      *sync.Cond
	operatorConditionValue bool
}

func (s *ManagedThread) SuspendUnsafe() {
	for !s.operatorConditionValue {
		s.operatorCondition.Wait()
	}
	s.operatorConditionValue = false
}

func (s *ManagedThread) Release() {
	s.operatorCondition.L.Lock()
	defer s.operatorCondition.L.Unlock()
	s.operatorConditionValue = true
	s.operatorCondition.Signal()
}

func (s *ManagedThread) Lock() {
	s.operatorCondition.L.Lock()
}

func (s *ManagedThread) Unlock() {
	s.operatorCondition.L.Unlock()
}

func NewManagedThread() *ManagedThread {
	return &ManagedThread{
		operatorCondition:      sync.NewCond(&sync.Mutex{}),
		operatorConditionValue: false,
	}
}

var ErrNotAllowed = errors.New("state transition is not allowed")

var ErrConcurrentStateModification = errors.New("concurrent state modification")

type RuntimeState interface {
	InitError() error
	Ready() error
	Name() string
}

type disallowEveryTransitionByDefault struct{}

func (s *disallowEveryTransitionByDefault) InitError() error { return ErrNotAllowed }
func (s *disallowEveryTransitionByDefault) Ready() error     { return ErrNotAllowed }

type Runtime struct {
	ManagedThread Suspendable

	currentState      RuntimeState
	stateLastModified time.Time
	responseTime      time.Time

	RuntimeStartedState   RuntimeState
	RuntimeInitErrorState RuntimeState
	RuntimeReadyState     RuntimeState
	RuntimeRunningState   RuntimeState
}

func (s *Runtime) Release() {
	s.ManagedThread.Release()
}

func (s *Runtime) SetState(state RuntimeState) {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	s.setStateUnsafe(state)
}

func (s *Runtime) setStateUnsafe(state RuntimeState) {
	s.currentState = state
	s.stateLastModified = time.Now()
}

func (s *Runtime) GetState() RuntimeState {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState
}

func (s *Runtime) Ready() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.Ready()
}

func (s *Runtime) InitError() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.InitError()
}

func (s *Runtime) GetRuntimeDescription() statejson.RuntimeDescription {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	res := statejson.RuntimeDescription{
		State: statejson.StateDescription{
			Name:         s.currentState.Name(),
			LastModified: s.stateLastModified,
		},
	}
	if !s.responseTime.IsZero() {
		res.State.ResponseTime = s.responseTime
	}
	return res
}

func NewRuntime(initFlow InitFlowSynchronization) *Runtime {
	runtime := &Runtime{
		ManagedThread: NewManagedThread(),
	}

	runtime.RuntimeStartedState = &RuntimeStartedState{runtime: runtime, initFlow: initFlow}
	runtime.RuntimeInitErrorState = &RuntimeInitErrorState{runtime: runtime, initFlow: initFlow}
	runtime.RuntimeReadyState = &RuntimeReadyState{runtime: runtime}
	runtime.RuntimeRunningState = &RuntimeRunningState{runtime: runtime}
	runtime.setStateUnsafe(runtime.RuntimeStartedState)
	return runtime
}

type RuntimeStartedState struct {
	runtime  *Runtime
	initFlow InitFlowSynchronization
}

func (s *RuntimeStartedState) Ready() error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeReadyState)

	err := s.initFlow.RuntimeReady()
	if err != nil {
		return err
	}

	s.runtime.ManagedThread.SuspendUnsafe()
	if s.runtime.currentState != s.runtime.RuntimeReadyState && s.runtime.currentState != s.runtime.RuntimeRunningState {
		return ErrConcurrentStateModification
	}

	s.runtime.setStateUnsafe(s.runtime.RuntimeRunningState)
	return nil
}

func (s *RuntimeStartedState) InitError() error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeInitErrorState)
	return nil
}

func (s *RuntimeStartedState) Name() string {
	return RuntimeStartedStateName
}

type RuntimeInitErrorState struct {
	disallowEveryTransitionByDefault
	runtime  *Runtime
	initFlow InitFlowSynchronization
}

func (s *RuntimeInitErrorState) Name() string {
	return RuntimeInitErrorStateName
}

type RuntimeReadyState struct {
	disallowEveryTransitionByDefault
	runtime *Runtime
}

func (s *RuntimeReadyState) Ready() error {
	s.runtime.ManagedThread.SuspendUnsafe()
	if s.runtime.currentState != s.runtime.RuntimeReadyState && s.runtime.currentState != s.runtime.RuntimeRunningState {
		return ErrConcurrentStateModification
	}

	s.runtime.setStateUnsafe(s.runtime.RuntimeRunningState)
	return nil
}

func (s *RuntimeReadyState) Name() string {
	return RuntimeReadyStateName
}

type RuntimeRunningState struct {
	disallowEveryTransitionByDefault
	runtime *Runtime
}

func (s *RuntimeRunningState) Ready() error {
	return nil
}

func (s *RuntimeRunningState) Name() string {
	return RuntimeRunningStateName
}
