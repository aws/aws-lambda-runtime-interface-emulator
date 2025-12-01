// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"math"
	"sync"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
)

const maxAgentsLimit uint16 = math.MaxUint16

type Gate interface {
	Register(count uint16)
	Reset()
	SetCount(uint16) error
	WalkThrough(interface{}) error
	AwaitGateCondition(ctx context.Context) (interface{}, error)
	CancelWithError(error)
	Clear()
}

type gateImpl struct {
	gateCondition *sync.Cond
	count         uint16
	arrived       uint16
	value         interface{}
	canceled      bool
	err           error
}

func (g *gateImpl) Register(count uint16) {
	g.gateCondition.L.Lock()
	defer g.gateCondition.L.Unlock()
	g.count += count
}

func (g *gateImpl) SetCount(count uint16) error {
	g.gateCondition.L.Lock()
	defer g.gateCondition.L.Unlock()

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

var ErrGateIntegrity = errors.New("ErrGateIntegrity")

var ErrGateCanceled = errors.New("ErrGateCanceled")

func (g *gateImpl) WalkThrough(value interface{}) error {
	g.gateCondition.L.Lock()
	defer g.gateCondition.L.Unlock()

	if g.arrived == g.count {
		return ErrGateIntegrity
	}

	g.arrived++
	g.value = value

	if g.arrived == g.count {
		g.gateCondition.Broadcast()
	}

	return nil
}

func (g *gateImpl) awaitGateCondition() error {
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

func (g *gateImpl) AwaitGateCondition(ctx context.Context) (interface{}, error) {
	var err error
	errorChan := make(chan error)

	go func() {
		errorChan <- g.awaitGateCondition()
	}()

	select {
	case err = <-errorChan:
		break
	case <-ctx.Done():
		err = interop.ErrTimeout
		g.CancelWithError(err)
		break
	}

	return g.value, err
}

func (g *gateImpl) CancelWithError(err error) {
	g.gateCondition.L.Lock()
	defer g.gateCondition.L.Unlock()
	g.canceled = true
	g.err = err
	g.gateCondition.Broadcast()
}

func (g *gateImpl) Clear() {
	g.gateCondition.L.Lock()
	defer g.gateCondition.L.Unlock()

	g.canceled = false
	g.arrived = 0
	g.value = nil
	g.err = nil
}

func NewGate(count uint16) Gate {
	return &gateImpl{
		count:         count,
		gateCondition: sync.NewCond(&sync.Mutex{}),
	}
}
