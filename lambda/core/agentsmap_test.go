// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestExternalAgentsMapLookupByName(t *testing.T) {
	m := NewExternalAgentsMap()

	err := m.Insert(&ExternalAgent{Name: "a", ID: uuid.New()})
	require.NoError(t, err)
	agentIn := &ExternalAgent{Name: "b", ID: uuid.New()}
	err = m.Insert(agentIn)
	require.NoError(t, err)
	err = m.Insert(&ExternalAgent{Name: "c", ID: uuid.New()})
	require.NoError(t, err)

	agentOut, found := m.FindByName(agentIn.Name)
	require.True(t, found)
	require.Equal(t, agentIn, agentOut)
}

func TestExternalAgentsMapLookupByID(t *testing.T) {
	m := NewExternalAgentsMap()

	err := m.Insert(&ExternalAgent{Name: "a", ID: uuid.New()})
	require.NoError(t, err)
	agentIn := &ExternalAgent{Name: "b", ID: uuid.New()}
	err = m.Insert(agentIn)
	require.NoError(t, err)
	err = m.Insert(&ExternalAgent{Name: "c", ID: uuid.New()})
	require.NoError(t, err)

	agentOut, found := m.FindByID(agentIn.ID)
	require.True(t, found)
	require.Equal(t, agentIn, agentOut)
}

func TestExternalAgentsMapInsertNameCollision(t *testing.T) {
	m := NewExternalAgentsMap()

	err := m.Insert(&ExternalAgent{Name: "a", ID: uuid.New()})
	require.NoError(t, err)

	err = m.Insert(&ExternalAgent{Name: "a", ID: uuid.New()})
	require.Equal(t, err, ErrAgentNameCollision)
}

func TestExternalAgentsMapInsertIDCollision(t *testing.T) {
	m := NewExternalAgentsMap()

	id := uuid.New()

	err := m.Insert(&ExternalAgent{Name: "a", ID: id})
	require.NoError(t, err)

	err = m.Insert(&ExternalAgent{Name: "b", ID: id})
	require.Equal(t, err, ErrAgentIDCollision)
}

func TestInternalAgentsMapLookupByName(t *testing.T) {
	m := NewInternalAgentsMap()

	err := m.Insert(&InternalAgent{Name: "a", ID: uuid.New()})
	require.NoError(t, err)
	agentIn := &InternalAgent{Name: "b", ID: uuid.New()}
	err = m.Insert(agentIn)
	require.NoError(t, err)
	err = m.Insert(&InternalAgent{Name: "c", ID: uuid.New()})
	require.NoError(t, err)

	agentOut, found := m.FindByName(agentIn.Name)
	require.True(t, found)
	require.Equal(t, agentIn, agentOut)
}

func TestInternalAgentsMapLookupByID(t *testing.T) {
	m := NewInternalAgentsMap()

	err := m.Insert(&InternalAgent{Name: "a", ID: uuid.New()})
	require.NoError(t, err)
	agentIn := &InternalAgent{Name: "b", ID: uuid.New()}
	err = m.Insert(agentIn)
	require.NoError(t, err)
	err = m.Insert(&InternalAgent{Name: "c", ID: uuid.New()})
	require.NoError(t, err)

	agentOut, found := m.FindByID(agentIn.ID)
	require.True(t, found)
	require.Equal(t, agentIn, agentOut)
}

func TestInternalAgentsMapInsertNameCollision(t *testing.T) {
	m := NewInternalAgentsMap()

	err := m.Insert(&InternalAgent{Name: "a", ID: uuid.New()})
	require.NoError(t, err)

	err = m.Insert(&InternalAgent{Name: "a", ID: uuid.New()})
	require.Equal(t, err, ErrAgentNameCollision)
}

func TestInternalAgentsMapInsertIDCollision(t *testing.T) {
	m := NewInternalAgentsMap()

	id := uuid.New()

	err := m.Insert(&InternalAgent{Name: "a", ID: id})
	require.NoError(t, err)

	err = m.Insert(&InternalAgent{Name: "b", ID: id})
	require.Equal(t, err, ErrAgentIDCollision)
}
