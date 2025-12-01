// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func TestRegistrationServiceHappyPathDuringInit(t *testing.T) {

	initFlow := NewInitFlowSynchronization()
	registrationService := NewRegistrationService(initFlow)

	registrationService.SetFunctionMetadata(model.FunctionMetadata{
		FunctionName:    "AWS_LAMBDA_FUNCTION_NAME",
		FunctionVersion: "AWS_LAMBDA_FUNCTION_VERSION",
		Handler:         "_HANDLER",
	})

	extAgentNames := []string{"agentName1", "agentName2"}
	assert.NoError(t, initFlow.SetExternalAgentsRegisterCount(uint16(len(extAgentNames))))

	extAgent1, err := registrationService.CreateExternalAgent(extAgentNames[0])
	assert.NoError(t, err)
	assert.Equal(t, extAgent1.String(), fmt.Sprintf("agentName1 (%s)", extAgent1.id))

	extAgent2, err := registrationService.CreateExternalAgent(extAgentNames[1])
	assert.NoError(t, err)

	go func() {
		for _, agentName := range extAgentNames {
			agent, found := registrationService.FindExternalAgentByName(agentName)
			assert.True(t, found)

			assert.NoError(t, agent.Register([]Event{ShutdownEvent}))
		}
	}()

	assert.NoError(t, initFlow.AwaitExternalAgentsRegistered(context.Background()))

	runtime := NewRuntime(initFlow)
	assert.NoError(t, registrationService.PreregisterRuntime(runtime))

	intAgentNames := []string{"intAgentName1", "intAgentName2"}

	intAgent1, err := registrationService.CreateInternalAgent(intAgentNames[0])
	assert.NoError(t, err)
	assert.Equal(t, intAgent1.String(), fmt.Sprintf("intAgentName1 (%s)", intAgent1.id))

	intAgent2, err := registrationService.CreateInternalAgent(intAgentNames[1])
	assert.NoError(t, err)

	go func() {
		for _, agentName := range intAgentNames {
			agent, found := registrationService.FindInternalAgentByName(agentName)
			assert.True(t, found)

			assert.NoError(t, agent.Register([]Event{}))
		}
		assert.NoError(t, runtime.Ready())
	}()

	assert.NoError(t, initFlow.AwaitRuntimeReady(context.Background()))
	registrationService.TurnOff()

	extAgent1Description := extAgent1.GetAgentDescription()
	assert.Equal(t, extAgent1Description.Name, "agentName1")
	assert.Equal(t, extAgent1Description.State.Name, "Registered")
	assert.Equal(t, extAgent1Description.ErrorType, "")

	intAgent1Description := intAgent1.GetAgentDescription()
	assert.Equal(t, intAgent1Description.Name, "intAgentName1")
	assert.Equal(t, intAgent1Description.State.Name, "Registered")
	assert.Equal(t, intAgent1Description.ErrorType, "")

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

	assert.NoError(t, initFlow.AwaitAgentsReady(context.Background()))

	expectedAgents := []AgentInfo{
		{extAgent1.Name(), "Ready", []string{"SHUTDOWN"}, ""},
		{extAgent2.Name(), "Ready", []string{"SHUTDOWN"}, ""},
		{intAgent1.Name(), "Ready", []string{}, ""},
		{intAgent2.Name(), "Ready", []string{}, ""},
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

func TestGetAgents(t *testing.T) {
	initFlow := NewInitFlowSynchronization()
	registrationService := NewRegistrationService(initFlow)

	externalAgentNames := map[string]bool{
		"external/1": true,
		"external/2": true,
	}

	for agentName := range externalAgentNames {
		_, err := registrationService.CreateExternalAgent(agentName)
		require.NoError(t, err)
	}

	internalAgentNames := map[string]bool{
		"internal/1": true,
		"internal/2": true,
		"internal/3": true,
	}

	for agentName := range internalAgentNames {
		_, err := registrationService.CreateInternalAgent(agentName)
		require.NoError(t, err)
	}

	actualExternalAgents := registrationService.GetExternalAgents()
	actualInternalAgents := registrationService.GetInternalAgents()

	assert.Equal(t, len(actualExternalAgents), 2)
	assert.Equal(t, len(actualInternalAgents), 3)

	for _, agent := range actualExternalAgents {
		delete(externalAgentNames, agent.Name())
	}

	assert.Equal(t, 0, len(externalAgentNames), "external agents: %v are not retrieved back from registration service", externalAgentNames)

	for _, agent := range actualInternalAgents {
		delete(internalAgentNames, agent.Name())
	}

	assert.Equal(t, 0, len(internalAgentNames), "internal agents: %v are not retrieved back from registration service", internalAgentNames)
}

func TestMapErrorToAgentInfoErrorType(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectedErr model.ErrorType
	}{
		{
			name:        "ErrTooManyExtensions returns too many extensions error",
			err:         ErrTooManyExtensions,
			expectedErr: model.ErrorAgentTooManyExtensions,
		},
		{
			name:        "wrapped permission error with fmt.Errorf returns permission denied error",
			err:         fmt.Errorf("failed to start command: %w", os.ErrPermission),
			expectedErr: model.ErrorAgentPermissionDenied,
		},
		{
			name:        "random error returns extension launch error",
			err:         errors.New("some random error"),
			expectedErr: model.ErrorAgentExtensionLaunch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapErrorToAgentInfoErrorType(tt.err)
			assert.Equal(t, tt.expectedErr, result, "Expected %s but got %s for error: %v", tt.expectedErr, result, tt.err)
		})
	}
}
