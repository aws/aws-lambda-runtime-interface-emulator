// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

type Status int

var ErrInvalidTransition = errors.New("invalid state transition")

func NewInvalidTransitionError(from, to Status) error {
	return fmt.Errorf("%w: from %v to %v", ErrInvalidTransition, from.String(), to.String())
}

const (
	Idle Status = iota
	Initializing
	Initialized
	ShuttingDown
	Shutdown
)

func (s Status) String() string {
	switch s {
	case Idle:
		return "Idle"
	case Initializing:
		return "Initializing"
	case Initialized:
		return "Initialized"
	case ShuttingDown:
		return "ShuttingDown"
	case Shutdown:
		return "Shutdown"
	default:
		return fmt.Sprintf("Status(%d)", int(s))
	}
}

type StateGuard struct {
	current Status
	mu      sync.RWMutex
}

func NewStateGuard() *StateGuard {
	return &StateGuard{}
}

func (sm *StateGuard) GetState() Status {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	slog.Debug("Raptor current state", "currState:", sm.current.String())
	return sm.current
}

func (sm *StateGuard) SetState(state Status) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if !isValidTransition(sm.current, state) {
		slog.Error("invalid state transition", "from", sm.current.String(), "to", state.String())
		return NewInvalidTransitionError(sm.current, state)
	}
	sm.current = state
	return nil
}

func isValidTransition(from, to Status) bool {

	if to == Idle {
		return true
	}

	switch from {
	case Idle:
		return to == Initializing || to == ShuttingDown
	case Initializing:
		return to == Initialized || to == ShuttingDown
	case Initialized:
		return to == ShuttingDown
	case ShuttingDown:
		return to == Shutdown
	case Shutdown:
		return false
	default:
		return false
	}
}
