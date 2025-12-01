// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExternalAgentsMapLookupByName(t *testing.T) {
	m := NewAgentsMap[*ExternalAgent]()

	err := m.Insert(&ExternalAgent{name: "a", id: uuid.New()})
	require.NoError(t, err)
	agentIn := &ExternalAgent{name: "b", id: uuid.New()}
	err = m.Insert(agentIn)
	require.NoError(t, err)
	err = m.Insert(&ExternalAgent{name: "c", id: uuid.New()})
	require.NoError(t, err)

	agentOut, found := m.FindByName(agentIn.Name())
	require.True(t, found)
	require.Equal(t, agentIn, agentOut)

	assert.Equal(t, m.Size(), 3)
}

func TestExternalAgentsMapLookupByID(t *testing.T) {
	m := NewAgentsMap[*ExternalAgent]()

	err := m.Insert(&ExternalAgent{name: "a", id: uuid.New()})
	require.NoError(t, err)
	agentIn := &ExternalAgent{name: "b", id: uuid.New()}
	err = m.Insert(agentIn)
	require.NoError(t, err)
	err = m.Insert(&ExternalAgent{name: "c", id: uuid.New()})
	require.NoError(t, err)

	agentOut, found := m.FindByID(agentIn.ID())
	require.True(t, found)
	require.Equal(t, agentIn, agentOut)

	m.Clear()
	assert.Equal(t, m.Size(), 0)
}

func TestExternalAgentsMapInsertNameCollision(t *testing.T) {
	m := NewAgentsMap[*ExternalAgent]()

	err := m.Insert(&ExternalAgent{name: "a", id: uuid.New()})
	require.NoError(t, err)

	err = m.Insert(&ExternalAgent{name: "a", id: uuid.New()})
	require.Equal(t, err, ErrAgentNameCollision)

	assert.Equal(t, m.Size(), 1)
}

func TestExternalAgentsMapInsertIDCollision(t *testing.T) {
	m := NewAgentsMap[*ExternalAgent]()

	id := uuid.New()

	err := m.Insert(&ExternalAgent{name: "a", id: id})
	require.NoError(t, err)

	err = m.Insert(&ExternalAgent{name: "b", id: id})
	require.Equal(t, err, ErrAgentIDCollision)

	assert.Equal(t, m.Size(), 1)
}
