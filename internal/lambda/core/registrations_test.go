// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistrationServiceHappyPathDuringInit(t *testing.T) {
	// Setup
	initFlow, invokeFlow := NewInitFlowSynchronization(), NewInvokeFlowSynchronization()
	registrationService := NewRegistrationService(initFlow, invokeFlow)

	registrationService.SetFunctionMetadata(FunctionMetadata{
		FunctionName:    "AWS_LAMBDA_FUNCTION_NAME",
		FunctionVersion: "AWS_LAMBDA_FUNCTION_VERSION",
		Handler:         "_HANDLER",
	})

	// Extension INIT (external)
	extAgentNames := []string{"agentName1", "agentName2"}
	assert.NoError(t, initFlow.SetExternalAgentsRegisterCount(uint16(len(extAgentNames))))

	extAgent1, err := registrationService.CreateExternalAgent(extAgentNames[0])
	assert.NoError(t, err)

	extAgent2, err := registrationService.CreateExternalAgent(extAgentNames[1])
	assert.NoError(t, err)

	go func() {
		for _, agentName := range extAgentNames {
			agent, found := registrationService.FindExternalAgentByName(agentName)
			assert.True(t, found)

			assert.NoError(t, agent.Register([]Event{InvokeEvent, ShutdownEvent}))
		}
	}()

	assert.NoError(t, initFlow.AwaitExternalAgentsRegistered())

	// Runtime INIT (+ internal extensions)
	runtime := NewRuntime(initFlow, invokeFlow)
	assert.NoError(t, registrationService.PreregisterRuntime(runtime))

	intAgentNames := []string{"intAgentName1", "intAgentName2"}

	intAgent1, err := registrationService.CreateInternalAgent(intAgentNames[0])
	assert.NoError(t, err)

	intAgent2, err := registrationService.CreateInternalAgent(intAgentNames[1])
	assert.NoError(t, err)

	go func() {
		for _, agentName := range intAgentNames {
			agent, found := registrationService.FindInternalAgentByName(agentName)
			assert.True(t, found)

			assert.NoError(t, agent.Register([]Event{InvokeEvent}))
		}
		assert.NoError(t, runtime.Ready())
	}()

	assert.NoError(t, initFlow.AwaitRuntimeRestoreReady())
	registrationService.TurnOff()

	// Agents Ready

	assert.NoError(t, initFlow.SetAgentsReadyCount(registrationService.GetRegisteredAgentsSize()))
	go func() {
		for _, agentName := range intAgentNames {
			agent, found := registrationService.FindInternalAgentByName(agentName)
			assert.True(t, found)
			go func() { assert.NoError(t, agent.Ready()) }()
		}

		for _, agentName := range extAgentNames {
			agent, found := registrationService.FindExternalAgentByName(agentName)
			assert.True(t, found)
			go func() { assert.NoError(t, agent.Ready()) }()
		}
	}()

	assert.NoError(t, initFlow.AwaitAgentsReady())

	// Assertions
	expectedAgents := []AgentInfo{
		AgentInfo{extAgent1.Name, "Ready", []string{"INVOKE", "SHUTDOWN"}, ""},
		AgentInfo{extAgent2.Name, "Ready", []string{"INVOKE", "SHUTDOWN"}, ""},
		AgentInfo{intAgent1.Name, "Ready", []string{"INVOKE"}, ""},
		AgentInfo{intAgent2.Name, "Ready", []string{"INVOKE"}, ""},
	}

	assert.Len(t, registrationService.AgentsInfo(), len(expectedAgents))

	actualAgents := map[string]AgentInfo{}
	for _, agentInfo := range registrationService.AgentsInfo() {
		actualAgents[agentInfo.Name] = agentInfo
	}

	for _, agentInfo := range expectedAgents {
		assert.Contains(t, actualAgents, agentInfo.Name)
		assert.Equal(t, actualAgents[agentInfo.Name].Name, agentInfo.Name)
		assert.Equal(t, actualAgents[agentInfo.Name].State, agentInfo.State)
		for _, event := range agentInfo.Subscriptions {
			assert.Contains(t, actualAgents[agentInfo.Name].Subscriptions, event)
		}
	}
}
