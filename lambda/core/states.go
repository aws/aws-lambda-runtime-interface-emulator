// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"sync"
	"time"

	"go.amzn.com/lambda/core/statejson"
	"go.amzn.com/lambda/interop"
)

// Suspendable on operator condition.
type Suspendable interface {
	SuspendUnsafe()
	Release()
	Lock()
	Unlock()
}

// ManagedThread is suspendable on operator condition.
type ManagedThread struct {
	operatorCondition      *sync.Cond
	operatorConditionValue bool
}

// SuspendUnsafe suspends ManagedThread on operator condition. This allows thread
// to be suspended and then resumed from the main thread.
// It's marked Unsafe because ManagedThread should be locked before SuspendUnsafe is called
func (s *ManagedThread) SuspendUnsafe() {
	for !s.operatorConditionValue {
		s.operatorCondition.Wait()
	}
	s.operatorConditionValue = false // reset back to false
}

// Release releases operator condition. This allows thread
// to be suspended and then resumed from the main thread.
func (s *ManagedThread) Release() {
	s.operatorCondition.L.Lock()
	defer s.operatorCondition.L.Unlock()
	s.operatorConditionValue = true
	s.operatorCondition.Signal()
}

// Lock ManagedThread condvar mutex
func (s *ManagedThread) Lock() {
	s.operatorCondition.L.Lock()
}

// Unlock ManagedThread condvar mutex
func (s *ManagedThread) Unlock() {
	s.operatorCondition.L.Unlock()
}

// NewManagedThread returns new ManagedThread instance.
func NewManagedThread() *ManagedThread {
	return &ManagedThread{
		operatorCondition:      sync.NewCond(&sync.Mutex{}),
		operatorConditionValue: false,
	}
}

// ErrNotAllowed returned on illegal state transition
var ErrNotAllowed = errors.New("State transition is not allowed")

// ErrConcurrentStateModification returned when we've detected an invalid state transision caused by concurrent modification
var ErrConcurrentStateModification = errors.New("Concurrent state modification")

// RuntimeState is runtime state machine interface.
type RuntimeState interface {
	InitError() error
	Ready() error
	RestoreReady() error
	InvocationResponse() error
	InvocationErrorResponse() error
	ResponseSent() error
	RestoreError(interop.FunctionError) error
	Name() string
}

type disallowEveryTransitionByDefault struct{}

func (s *disallowEveryTransitionByDefault) InitError() error               { return ErrNotAllowed }
func (s *disallowEveryTransitionByDefault) Ready() error                   { return ErrNotAllowed }
func (s *disallowEveryTransitionByDefault) RestoreReady() error            { return ErrNotAllowed }
func (s *disallowEveryTransitionByDefault) InvocationResponse() error      { return ErrNotAllowed }
func (s *disallowEveryTransitionByDefault) InvocationErrorResponse() error { return ErrNotAllowed }
func (s *disallowEveryTransitionByDefault) ResponseSent() error            { return ErrNotAllowed }
func (s *disallowEveryTransitionByDefault) RestoreError(interop.FunctionError) error {
	return ErrNotAllowed
}

// Runtime is runtime object.
type Runtime struct {
	ManagedThread Suspendable

	currentState      RuntimeState
	stateLastModified time.Time
	responseTime      time.Time

	RuntimeStartedState                 RuntimeState
	RuntimeInitErrorState               RuntimeState
	RuntimeReadyState                   RuntimeState
	RuntimeRunningState                 RuntimeState
	RuntimeRestoreReadyState            RuntimeState
	RuntimeRestoringState               RuntimeState
	RuntimeInvocationResponseState      RuntimeState
	RuntimeInvocationErrorResponseState RuntimeState
	RuntimeResponseSentState            RuntimeState
	RuntimeRestoreErrorState            RuntimeState
}

// Release ...
func (s *Runtime) Release() {
	s.ManagedThread.Release()
}

// SetState ...
func (s *Runtime) SetState(state RuntimeState) {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	s.setStateUnsafe(state)
}

func (s *Runtime) setStateUnsafe(state RuntimeState) {
	s.currentState = state
	s.stateLastModified = time.Now()
}

// GetState ...
func (s *Runtime) GetState() RuntimeState {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState
}

// Ready delegates to state implementation.
func (s *Runtime) Ready() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.Ready()
}

func (s *Runtime) RestoreReady() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.RestoreReady()
}

// InvocationResponse delegates to state implementation.
func (s *Runtime) InvocationResponse() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.InvocationResponse()
}

// InvocationErrorResponse delegates to state implementation.
func (s *Runtime) InvocationErrorResponse() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.InvocationErrorResponse()
}

// InitError delegates to state implementation.
func (s *Runtime) InitError() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.InitError()
}

// ResponseSent delegates to state implementation.
func (s *Runtime) ResponseSent() error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	err := s.currentState.ResponseSent()
	if err == nil {
		s.responseTime = time.Now()
	}
	return err
}

func (s *Runtime) RestoreError(UserError interop.FunctionError) error {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	return s.currentState.RestoreError(UserError)
}

// GetRuntimeDescription returns runtime description object for debugging purposes
func (s *Runtime) GetRuntimeDescription() statejson.RuntimeDescription {
	s.ManagedThread.Lock()
	defer s.ManagedThread.Unlock()
	res := statejson.RuntimeDescription{
		State: statejson.StateDescription{
			Name:         s.currentState.Name(),
			LastModified: s.stateLastModified.UnixNano() / int64(time.Millisecond),
		},
	}
	if !s.responseTime.IsZero() {
		res.State.ResponseTimeNs = s.responseTime.UnixNano()
	}
	return res
}

// NewRuntime returns new Runtime instance.
func NewRuntime(initFlow InitFlowSynchronization, invokeFlow InvokeFlowSynchronization) *Runtime {
	runtime := &Runtime{
		ManagedThread: NewManagedThread(),
	}

	runtime.RuntimeStartedState = &RuntimeStartedState{runtime: runtime, initFlow: initFlow}
	runtime.RuntimeInitErrorState = &RuntimeInitErrorState{runtime: runtime, initFlow: initFlow}
	runtime.RuntimeReadyState = &RuntimeReadyState{runtime: runtime}
	runtime.RuntimeRunningState = &RuntimeRunningState{runtime: runtime, invokeFlow: invokeFlow}
	runtime.RuntimeInvocationResponseState = &RuntimeInvocationResponseState{runtime: runtime, invokeFlow: invokeFlow}
	runtime.RuntimeInvocationErrorResponseState = &RuntimeInvocationErrorResponseState{runtime: runtime, invokeFlow: invokeFlow}
	runtime.RuntimeResponseSentState = &RuntimeResponseSentState{runtime: runtime, invokeFlow: invokeFlow}
	runtime.RuntimeRestoreReadyState = &RuntimeRestoreReadyState{}
	runtime.RuntimeRestoringState = &RuntimeRestoringState{runtime: runtime, initFlow: initFlow}
	runtime.RuntimeRestoreErrorState = &RuntimeRestoreErrorState{runtime: runtime, initFlow: initFlow}

	runtime.setStateUnsafe(runtime.RuntimeStartedState)
	return runtime
}

// RuntimeStartedState runtime started state.
type RuntimeStartedState struct {
	disallowEveryTransitionByDefault
	runtime  *Runtime
	initFlow InitFlowSynchronization
}

// Ready call when runtime init done.
func (s *RuntimeStartedState) Ready() error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeReadyState)
	// runtime called /next without calling /restore/next
	// that means it's not interested in restore phase
	err := s.initFlow.RuntimeRestoreReady()
	if err != nil {
		return err
	}

	err = s.initFlow.RuntimeReady()
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

func (s *RuntimeStartedState) RestoreReady() error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeRestoreReadyState)
	err := s.initFlow.RuntimeRestoreReady()
	if err != nil {
		return err
	}

	s.runtime.ManagedThread.SuspendUnsafe()
	if s.runtime.currentState != s.runtime.RuntimeRestoreReadyState && s.runtime.currentState != s.runtime.RuntimeRestoringState {
		return ErrConcurrentStateModification
	}

	s.runtime.setStateUnsafe(s.runtime.RuntimeRestoringState)
	return nil
}

// InitError move runtime to init error state.
func (s *RuntimeStartedState) InitError() error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeInitErrorState)
	return nil
}

// Name ...
func (s *RuntimeStartedState) Name() string {
	return RuntimeStartedStateName
}

type RuntimeRestoringState struct {
	disallowEveryTransitionByDefault
	runtime  *Runtime
	initFlow InitFlowSynchronization
}

// Runtime is healthy after restore and called /next
func (s *RuntimeRestoringState) Ready() error {
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

func (s *RuntimeRestoringState) RestoreError(userError interop.FunctionError) error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeRestoreErrorState)
	s.initFlow.CancelWithError(interop.ErrRestoreHookUserError{UserError: userError})
	return nil
}

func (s *RuntimeRestoringState) Name() string {
	return RuntimeRestoringStateName
}

// RuntimeInitErrorState runtime started state.
type RuntimeInitErrorState struct {
	disallowEveryTransitionByDefault
	runtime  *Runtime
	initFlow InitFlowSynchronization
}

// Name ...
func (s *RuntimeInitErrorState) Name() string {
	return RuntimeInitErrorStateName
}

// RuntimeReadyState runtime ready state.
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

// Name ...
func (s *RuntimeReadyState) Name() string {
	return RuntimeReadyStateName
}

// RuntimeRunningState runtime ready state.
type RuntimeRunningState struct {
	disallowEveryTransitionByDefault
	runtime    *Runtime
	invokeFlow InvokeFlowSynchronization
}

func (s *RuntimeRunningState) Ready() error {
	return nil
}

// InvocationResponse call when runtime response is available.
func (s *RuntimeRunningState) InvocationResponse() error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeInvocationResponseState)
	return nil
}

// InvocationErrorResponse call when runtime error response is available.
func (s *RuntimeRunningState) InvocationErrorResponse() error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeInvocationErrorResponseState)
	return nil
}

// Name ...
func (s *RuntimeRunningState) Name() string {
	return RuntimeRunningStateName
}

type RuntimeRestoreReadyState struct {
	disallowEveryTransitionByDefault
}

func (s *RuntimeRestoreReadyState) Name() string {
	return RuntimeRestoreReadyStateName
}

// RuntimeInvocationResponseState runtime response is available.
// Start state for runtime response submission.
type RuntimeInvocationResponseState struct {
	disallowEveryTransitionByDefault
	runtime    *Runtime
	invokeFlow InvokeFlowSynchronization
}

// ResponseSent completes RuntimeInvocationResponseState.
func (s *RuntimeInvocationResponseState) ResponseSent() error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeResponseSentState)
	return s.invokeFlow.RuntimeResponse(s.runtime)
}

// Name ...
func (s *RuntimeInvocationResponseState) Name() string {
	return RuntimeInvocationResponseStateName
}

// RuntimeInvocationErrorResponseState runtime response is available.
// Start state for runtime error response submission.
type RuntimeInvocationErrorResponseState struct {
	disallowEveryTransitionByDefault
	runtime    *Runtime
	invokeFlow InvokeFlowSynchronization
}

// ResponseSent completes RuntimeInvocationErrorResponseState.
func (s *RuntimeInvocationErrorResponseState) ResponseSent() error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeResponseSentState)
	return s.invokeFlow.RuntimeResponse(s.runtime)
}

// Name ...
func (s *RuntimeInvocationErrorResponseState) Name() string {
	return RuntimeInvocationErrorResponseStateName
}

// RuntimeResponseSentState ends started runtime response or runtime error response submission.
type RuntimeResponseSentState struct {
	disallowEveryTransitionByDefault
	runtime    *Runtime
	invokeFlow InvokeFlowSynchronization
}

// Ready call when runtime ready.
func (s *RuntimeResponseSentState) Ready() error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeReadyState)
	if err := s.invokeFlow.RuntimeReady(s.runtime); err != nil {
		return err
	}

	s.runtime.ManagedThread.SuspendUnsafe()
	if s.runtime.currentState != s.runtime.RuntimeReadyState && s.runtime.currentState != s.runtime.RuntimeRunningState {
		return ErrConcurrentStateModification
	}

	s.runtime.setStateUnsafe(s.runtime.RuntimeRunningState)
	return nil
}

// Name ...
func (s *RuntimeResponseSentState) Name() string {
	return RuntimeResponseSentStateName
}

type RuntimeRestoreErrorState struct {
	disallowEveryTransitionByDefault
	runtime  *Runtime
	initFlow InitFlowSynchronization
}

func (s *RuntimeRestoreErrorState) Name() string {
	return RuntimeRestoreErrorStateName
}
