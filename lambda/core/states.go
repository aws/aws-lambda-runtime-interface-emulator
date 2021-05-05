// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"sync"
	"time"

	"go.amzn.com/lambda/core/statejson"
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
	InvocationResponse() error
	InvocationErrorResponse() error
	ResponseSent() error
	Name() string
}

// Runtime is runtime object.
type Runtime struct {
	ManagedThread Suspendable

	currentState      RuntimeState
	stateLastModified time.Time
	Pid               int
	responseTime      time.Time

	RuntimeStartedState                 RuntimeState
	RuntimeInitErrorState               RuntimeState
	RuntimeReadyState                   RuntimeState
	RuntimeInvocationResponseState      RuntimeState
	RuntimeInvocationErrorResponseState RuntimeState
	RuntimeResponseSentState            RuntimeState
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
	runtime.RuntimeReadyState = &RuntimeReadyState{runtime: runtime, invokeFlow: invokeFlow}
	runtime.RuntimeInvocationResponseState = &RuntimeInvocationResponseState{runtime: runtime, invokeFlow: invokeFlow}
	runtime.RuntimeInvocationErrorResponseState = &RuntimeInvocationErrorResponseState{runtime: runtime, invokeFlow: invokeFlow}
	runtime.RuntimeResponseSentState = &RuntimeResponseSentState{runtime: runtime, invokeFlow: invokeFlow}

	runtime.setStateUnsafe(runtime.RuntimeStartedState)
	return runtime
}

// RuntimeStartedState runtime started state.
type RuntimeStartedState struct {
	runtime  *Runtime
	initFlow InitFlowSynchronization
}

// Ready call when runtime init done.
func (s *RuntimeStartedState) Ready() error {
	err := s.initFlow.RuntimeReady()
	if err != nil {
		return err
	}

	s.runtime.ManagedThread.SuspendUnsafe()

	s.runtime.setStateUnsafe(s.runtime.RuntimeReadyState)
	return nil
}

// InvocationResponse not allowed in this state.
func (s *RuntimeStartedState) InvocationResponse() error {
	return ErrNotAllowed
}

// InvocationErrorResponse not allowed in this state.
func (s *RuntimeStartedState) InvocationErrorResponse() error {
	return ErrNotAllowed
}

// InvocationErrorResponse not allowed in this state.
func (s *RuntimeStartedState) ResponseSent() error {
	return ErrNotAllowed
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

// RuntimeInitErrorState runtime started state.
type RuntimeInitErrorState struct {
	runtime  *Runtime
	initFlow InitFlowSynchronization
}

// Ready not allowed
func (s *RuntimeInitErrorState) Ready() error {
	return ErrNotAllowed
}

// InvocationResponse not allowed
func (s *RuntimeInitErrorState) InvocationResponse() error {
	return ErrNotAllowed
}

// InvocationErrorResponse not allowed
func (s *RuntimeInitErrorState) InvocationErrorResponse() error {
	return ErrNotAllowed
}

// InvocationErrorResponse not allowed
func (s *RuntimeInitErrorState) ResponseSent() error {
	return ErrNotAllowed
}

// InitError not allowed
func (s *RuntimeInitErrorState) InitError() error {
	return ErrNotAllowed
}

// Name ...
func (s *RuntimeInitErrorState) Name() string {
	return RuntimeInitErrorStateName
}

// RuntimeReadyState runtime ready state.
type RuntimeReadyState struct {
	runtime    *Runtime
	invokeFlow InvokeFlowSynchronization
}

func (s *RuntimeReadyState) Ready() error {
	return nil
}

// InvocationResponse call when runtime response is available.
func (s *RuntimeReadyState) InvocationResponse() error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeInvocationResponseState)
	return nil
}

// InvocationErrorResponse call when runtime error response is available.
func (s *RuntimeReadyState) InvocationErrorResponse() error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeInvocationErrorResponseState)
	return nil
}

// ResponseSent is a closing state for InvocationResponseState and InvocationErrorResponseState.
func (s *RuntimeReadyState) ResponseSent() error {
	return ErrNotAllowed
}

// InitError not allowed in this state.
func (s *RuntimeReadyState) InitError() error {
	return ErrNotAllowed
}

// Name ...
func (s *RuntimeReadyState) Name() string {
	return RuntimeReadyStateName
}

// RuntimeInvocationResponseState runtime response is available.
// Start state for runtime response submission.
type RuntimeInvocationResponseState struct {
	runtime    *Runtime
	invokeFlow InvokeFlowSynchronization
}

// Ready call when runtime ready.
func (s *RuntimeInvocationResponseState) Ready() error {
	return ErrNotAllowed
}

// InvocationResponse not allowed in this state.
func (s *RuntimeInvocationResponseState) InvocationResponse() error {
	return ErrNotAllowed
}

// InvocationErrorResponse not allowed in this state.
func (s *RuntimeInvocationResponseState) InvocationErrorResponse() error {
	return ErrNotAllowed
}

// ResponseSent completes RuntimeInvocationResponseState.
func (s *RuntimeInvocationResponseState) ResponseSent() error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeResponseSentState)
	return s.invokeFlow.RuntimeResponse(s.runtime)
}

// InitError not allowed in this state.
func (s *RuntimeInvocationResponseState) InitError() error {
	// TODO log
	return ErrNotAllowed
}

// Name ...
func (s *RuntimeInvocationResponseState) Name() string {
	return RuntimeInvocationResponseStateName
}

// RuntimeInvocationErrorResponseState runtime response is available.
// Start state for runtime error response submission.
type RuntimeInvocationErrorResponseState struct {
	runtime    *Runtime
	invokeFlow InvokeFlowSynchronization
}

// Ready call when runtime ready.
func (s *RuntimeInvocationErrorResponseState) Ready() error {
	return ErrNotAllowed
}

// InvocationResponse not allowed in this state.
func (s *RuntimeInvocationErrorResponseState) InvocationResponse() error {
	return ErrNotAllowed
}

// InvocationErrorResponse not allowed in this state.
func (s *RuntimeInvocationErrorResponseState) InvocationErrorResponse() error {
	return ErrNotAllowed
}

// ResponseSent completes RuntimeInvocationErrorResponseState.
func (s *RuntimeInvocationErrorResponseState) ResponseSent() error {
	s.runtime.setStateUnsafe(s.runtime.RuntimeResponseSentState)
	return s.invokeFlow.RuntimeResponse(s.runtime)
}

// InitError not allowed in this state.
func (s *RuntimeInvocationErrorResponseState) InitError() error {
	return ErrNotAllowed
}

// Name ...
func (s *RuntimeInvocationErrorResponseState) Name() string {
	return RuntimeInvocationErrorResponseStateName
}

// RuntimeResponseSentState ends started runtime response or runtime error response submission.
type RuntimeResponseSentState struct {
	runtime    *Runtime
	invokeFlow InvokeFlowSynchronization
}

// Ready call when runtime ready.
func (s *RuntimeResponseSentState) Ready() error {
	if err := s.invokeFlow.RuntimeReady(s.runtime); err != nil {
		return err
	}

	s.runtime.ManagedThread.SuspendUnsafe()

	s.runtime.setStateUnsafe(s.runtime.RuntimeReadyState)
	return nil
}

// InvocationResponse not allowed in this state.
func (s *RuntimeResponseSentState) InvocationResponse() error {
	return ErrNotAllowed
}

// InvocationErrorResponse not allowed in this state.
func (s *RuntimeResponseSentState) InvocationErrorResponse() error {
	return ErrNotAllowed
}

// ResponseSent completes RuntimeInvocationErrorResponseState.
func (s *RuntimeResponseSentState) ResponseSent() error {
	return ErrNotAllowed
}

// InitError not allowed in this state.
func (s *RuntimeResponseSentState) InitError() error {
	// TODO log
	return ErrNotAllowed
}

// Name ...
func (s *RuntimeResponseSentState) Name() string {
	return RuntimeResponseSentStateName
}
