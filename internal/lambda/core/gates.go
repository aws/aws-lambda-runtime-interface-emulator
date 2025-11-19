// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"math"
	"sync"
)

const maxAgentsLimit uint16 = math.MaxUint16

// Gate ...
type Gate interface {
	Register(count uint16)
	Reset()
	SetCount(uint16) error
	WalkThrough() error
	AwaitGateCondition() error
	CancelWithError(error)
	Clear()
}

type gateImpl struct {
	count         uint16
	arrived       uint16
	gateCondition *sync.Cond
	canceled      bool
	err           error
}

func (g *gateImpl) Register(count uint16) {
	g.gateCondition.L.Lock()
	defer g.gateCondition.L.Unlock()
	g.count += count
}

// SetCount sets the expected number of arrivals on the gate
func (g *gateImpl) SetCount(count uint16) error {
	g.gateCondition.L.Lock()
	defer g.gateCondition.L.Unlock()
	// you can't set count larger than limit if limit is max uint but leaving it here for correctness in case limit changes
	if count > maxAgentsLimit || count < g.arrived {
		return ErrGateIntegrity
	}
	g.count = count
	return nil
}

func (g *gateImpl) Reset() {
	g.gateCondition.L.Lock()
	defer g.gateCondition.L.Unlock()
	if !g.canceled {
		g.arrived = 0
	}
}

// ErrGateIntegrity ...
var ErrGateIntegrity = errors.New("ErrGateIntegrity")

// ErrGateCanceled ...
var ErrGateCanceled = errors.New("ErrGateCanceled")

// WalkThrough walks through this gate without awaiting others.
func (g *gateImpl) WalkThrough() error {
	g.gateCondition.L.Lock()
	defer g.gateCondition.L.Unlock()

	if g.arrived == g.count {
		return ErrGateIntegrity
	}

	g.arrived++

	if g.arrived == g.count {
		g.gateCondition.Broadcast()
	}

	return nil
}

// AwaitGateCondition suspends thread execution until gate condition
// is met or await is canceled via Cancel method.
func (g *gateImpl) AwaitGateCondition() error {
	g.gateCondition.L.Lock()
	defer g.gateCondition.L.Unlock()

	for g.arrived != g.count && !g.canceled {
		g.gateCondition.Wait()
	}

	if g.canceled {
		if g.err != nil {
			return g.err
		}
		return ErrGateCanceled
	}

	return nil
}

// CancelWithError cancels gate condition with error and awakes suspended threads.
func (g *gateImpl) CancelWithError(err error) {
	g.gateCondition.L.Lock()
	defer g.gateCondition.L.Unlock()
	g.canceled = true
	g.err = err
	g.gateCondition.Broadcast()
}

// Clear gate state
func (g *gateImpl) Clear() {
	g.gateCondition.L.Lock()
	defer g.gateCondition.L.Unlock()

	g.canceled = false
	g.arrived = 0
	g.err = nil
}

// NewGate returns new gate instance.
func NewGate(count uint16) Gate {
	return &gateImpl{
		count:         count,
		gateCondition: sync.NewCond(&sync.Mutex{}),
	}
}
