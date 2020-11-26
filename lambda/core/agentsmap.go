// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"github.com/google/uuid"
)

// ErrAgentNameCollision means that agent with the same name already exists in AgentsMap
var ErrAgentNameCollision = errors.New("ErrAgentNameCollision")

// ErrAgentIDCollision means that agent with the same ID already exists in AgentsMap
var ErrAgentIDCollision = errors.New("ErrAgentIDCollision")

// ExternalAgentsMap stores Agents indexed by Name and ID
type ExternalAgentsMap struct {
	byName map[string]*ExternalAgent
	byID   map[string]*ExternalAgent
}

// NewExternalAgentsMap creates empty ExternalAgentsMap
func NewExternalAgentsMap() ExternalAgentsMap {
	return ExternalAgentsMap{
		byName: make(map[string]*ExternalAgent),
		byID:   make(map[string]*ExternalAgent),
	}
}

// Insert places agent into ExternalAgentsMap. Error is returned if agent with this ID or name already exists
func (m *ExternalAgentsMap) Insert(a *ExternalAgent) error {
	if _, nameCollision := m.FindByName(a.Name); nameCollision {
		return ErrAgentNameCollision
	}

	if _, idCollision := m.FindByID(a.ID); idCollision {
		return ErrAgentIDCollision
	}

	m.byName[a.Name] = a
	m.byID[a.ID.String()] = a

	return nil
}

// FindByName finds agent by name
func (m *ExternalAgentsMap) FindByName(name string) (agent *ExternalAgent, found bool) {
	agent, found = m.byName[name]
	return
}

// FindByID finds agent by ID
func (m *ExternalAgentsMap) FindByID(id uuid.UUID) (agent *ExternalAgent, found bool) {
	agent, found = m.byID[id.String()]
	return
}

// Visit iterates through agents, calling cb for each of them
func (m *ExternalAgentsMap) Visit(cb func(*ExternalAgent)) {
	for _, a := range m.byName {
		cb(a)
	}
}

// Size returns the number of agents contained in the datastructure
func (m *ExternalAgentsMap) Size() int {
	return len(m.byName)
}

// AsArray returns shallow copy of all agents as a single array. The order of agents is unspecified.
func (m *ExternalAgentsMap) AsArray() []*ExternalAgent {
	agents := make([]*ExternalAgent, 0, len(m.byName))

	m.Visit(func(a *ExternalAgent) {
		agents = append(agents, a)
	})

	return agents
}

func (m *ExternalAgentsMap) Clear() {
	m.byName = make(map[string]*ExternalAgent)
	m.byID = make(map[string]*ExternalAgent)
}

// InternalAgentsMap stores Agents indexed by Name and ID
type InternalAgentsMap struct {
	byName map[string]*InternalAgent
	byID   map[string]*InternalAgent
}

// NewInternalAgentsMap creates empty InternalAgentsMap
func NewInternalAgentsMap() InternalAgentsMap {
	return InternalAgentsMap{
		byName: make(map[string]*InternalAgent),
		byID:   make(map[string]*InternalAgent),
	}
}

// Insert places agent into InternalAgentsMap. Error is returned if agent with this ID or name already exists
func (m *InternalAgentsMap) Insert(a *InternalAgent) error {
	if _, nameCollision := m.FindByName(a.Name); nameCollision {
		return ErrAgentNameCollision
	}

	if _, idCollision := m.FindByID(a.ID); idCollision {
		return ErrAgentIDCollision
	}

	m.byName[a.Name] = a
	m.byID[a.ID.String()] = a

	return nil
}

// FindByName finds agent by name
func (m *InternalAgentsMap) FindByName(name string) (agent *InternalAgent, found bool) {
	agent, found = m.byName[name]
	return
}

// FindByID finds agent by ID
func (m *InternalAgentsMap) FindByID(id uuid.UUID) (agent *InternalAgent, found bool) {
	agent, found = m.byID[id.String()]
	return
}

// Visit iterates through agents, calling cb for each of them
func (m *InternalAgentsMap) Visit(cb func(*InternalAgent)) {
	for _, a := range m.byName {
		cb(a)
	}
}

// Size returns the number of agents contained in the datastructure
func (m *InternalAgentsMap) Size() int {
	return len(m.byName)
}

// AsArray returns shallow copy of all agents as a single array. The order of agents is unspecified.
func (m *InternalAgentsMap) AsArray() []*InternalAgent {
	agents := make([]*InternalAgent, 0, len(m.byName))

	m.Visit(func(a *InternalAgent) {
		agents = append(agents, a)
	})

	return agents
}

func (m *InternalAgentsMap) Clear() {
	m.byName = make(map[string]*InternalAgent)
	m.byID = make(map[string]*InternalAgent)
}
