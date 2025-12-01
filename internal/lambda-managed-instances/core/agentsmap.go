// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"

	"github.com/google/uuid"
)

type Agent interface {
	*ExternalAgent | *InternalAgent
	Name() string
	ID() uuid.UUID
}

var ErrAgentNameCollision = errors.New("ErrAgentNameCollision")

var ErrAgentIDCollision = errors.New("ErrAgentIDCollision")

type AgentsMap[T Agent] struct {
	byName map[string]T
	byID   map[string]T
}

func NewAgentsMap[T Agent]() AgentsMap[T] {
	return AgentsMap[T]{
		byName: make(map[string]T),
		byID:   make(map[string]T),
	}
}

func (m *AgentsMap[T]) Insert(a T) error {
	if _, nameCollision := m.FindByName(a.Name()); nameCollision {
		return ErrAgentNameCollision
	}

	if _, idCollision := m.FindByID(a.ID()); idCollision {
		return ErrAgentIDCollision
	}

	m.byName[a.Name()] = a
	m.byID[a.ID().String()] = a

	return nil
}

func (m *AgentsMap[T]) FindByName(name string) (agent T, found bool) {
	agent, found = m.byName[name]
	return agent, found
}

func (m *AgentsMap[T]) FindByID(id uuid.UUID) (agent T, found bool) {
	agent, found = m.byID[id.String()]
	return agent, found
}

func (m *AgentsMap[T]) Visit(cb func(T)) {
	for _, a := range m.byName {
		cb(a)
	}
}

func (m *AgentsMap[T]) Size() int {
	return len(m.byName)
}

func (m *AgentsMap[T]) Clear() {
	m.byName = make(map[string]T)
	m.byID = make(map[string]T)
}
