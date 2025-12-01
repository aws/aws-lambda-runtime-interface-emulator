// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	supervisormodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/supervisor/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
)

func TestShutdown(t *testing.T) {
	type Agent struct {
		name       string
		subscribed bool
	}

	tests := []struct {
		name string

		runtimeShutdownTimeout time.Duration

		shutdownTimeout time.Duration

		shutdownReason model.ShutdownReason

		agents []Agent

		internalAgentsCount int

		runtimeTermination func(*shutdownContext, *supervisormodel.MockProcessSupervisor)

		subscribedExtensionTermination func(*shutdownContext, *supervisormodel.MockProcessSupervisor)

		noSubscriptionExtensionTermination func(*shutdownContext, *supervisormodel.MockProcessSupervisor)

		verifyShutdownResult func(*testing.T, error, *shutdownContext)
	}{
		{
			name: "No extensions, runtime kill successful -> shutdown is success",

			runtimeShutdownTimeout: 30 * time.Millisecond,
			shutdownTimeout:        100 * time.Millisecond,
			shutdownReason:         model.Spindown,
			runtimeTermination: func(s *shutdownContext, supv *supervisormodel.MockProcessSupervisor) {

				supv.On("Kill", mock.Anything, mock.Anything).Return(nil).Once()

				exitedChannel, _ := s.getExitedChannel(runtimeProcessName)
				close(exitedChannel)
			},

			verifyShutdownResult: func(t *testing.T, err error, s *shutdownContext) {
				assert.Nil(t, err)
			},
		},
		{
			name: "No extensions, runtime kill fails -> shutdown is a failure",

			runtimeShutdownTimeout: 30 * time.Millisecond,
			shutdownTimeout:        100 * time.Millisecond,
			shutdownReason:         model.Spindown,
			runtimeTermination: func(s *shutdownContext, supv *supervisormodel.MockProcessSupervisor) {

				supv.On("Kill", mock.Anything, mock.Anything).Return(fmt.Errorf("boom")).Once()

				exitedChannel, _ := s.getExitedChannel(runtimeProcessName)
				close(exitedChannel)
			},

			verifyShutdownResult: func(t *testing.T, err error, s *shutdownContext) {
				assert.NotNil(t, err)
				assert.NotEmpty(t, s.processExited)
			},
		},
		{
			name: "No extensions, we Kill runtime, but don't get a signal runtime actually exited -> shutdown is a failure",

			runtimeShutdownTimeout: 30 * time.Millisecond,
			shutdownTimeout:        100 * time.Millisecond,
			shutdownReason:         model.Spindown,
			runtimeTermination: func(s *shutdownContext, supv *supervisormodel.MockProcessSupervisor) {

				supv.On("Kill", mock.Anything, mock.Anything).Return(nil).Once()
			},

			verifyShutdownResult: func(t *testing.T, err error, s *shutdownContext) {
				assert.NotNil(t, err)
				assert.NotEmpty(t, s.processExited)
			},
		},
		{
			name:                   "Runtime and one extensions (subscribed to shutdown) and terminates gracefully -> shutdown is success",
			runtimeShutdownTimeout: 36 * time.Millisecond,
			shutdownTimeout:        120 * time.Millisecond,
			shutdownReason:         model.Spindown,
			agents: []Agent{
				{
					name:       "agent1",
					subscribed: true,
				},
			},

			runtimeTermination: func(s *shutdownContext, supv *supervisormodel.MockProcessSupervisor) {
				supv.On("Terminate", mock.Anything, mock.Anything).Return(nil).Once()

				runtimeExitedChannel, _ := s.getExitedChannel(runtimeProcessName)
				close(runtimeExitedChannel)
			},

			subscribedExtensionTermination: func(s *shutdownContext, _ *supervisormodel.MockProcessSupervisor) {

				go func() {

					extensionExitedChannel, _ := s.getExitedChannel(extensionProcessName("agent1"))
					time.Sleep(20 * time.Millisecond)
					close(extensionExitedChannel)
				}()
			},

			verifyShutdownResult: func(t *testing.T, err error, s *shutdownContext) {
				assert.Nil(t, err)
			},
		},
		{
			name:                   "Runtime and only one internal extension with successful runtime termination -> shutdown is success",
			runtimeShutdownTimeout: 36 * time.Millisecond,
			shutdownTimeout:        120 * time.Millisecond,
			shutdownReason:         model.Spindown,

			internalAgentsCount: 1,

			runtimeTermination: func(s *shutdownContext, supv *supervisormodel.MockProcessSupervisor) {
				supv.On("Terminate", mock.Anything, mock.Anything).Return(nil).Once()

				runtimeExitedChannel, _ := s.getExitedChannel(runtimeProcessName)
				close(runtimeExitedChannel)
			},

			verifyShutdownResult: func(t *testing.T, err error, s *shutdownContext) {
				assert.Nil(t, err)
			},
		},
		{
			name:                   "Runtime and one extensions (subscribed to shutdown) and needs to be killed (fails) -> shutdown is a failure",
			runtimeShutdownTimeout: 8 * time.Millisecond,
			shutdownTimeout:        25 * time.Millisecond,
			shutdownReason:         model.Spindown,
			agents: []Agent{
				{
					name:       "agent1",
					subscribed: true,
				},
			},

			runtimeTermination: func(s *shutdownContext, supv *supervisormodel.MockProcessSupervisor) {
				supv.On("Terminate", mock.Anything, mock.Anything).Return(nil)

				runtimeExitedChannel, _ := s.getExitedChannel(runtimeProcessName)
				close(runtimeExitedChannel)
			},

			subscribedExtensionTermination: func(s *shutdownContext, supv *supervisormodel.MockProcessSupervisor) {

				supv.On("Kill", mock.Anything, mock.Anything).Return(fmt.Errorf("boom")).Once()
			},

			verifyShutdownResult: func(t *testing.T, err error, s *shutdownContext) {
				assert.NotNil(t, err)
				assert.NotEmpty(t, s.processExited)
			},
		},
		{
			name:                   "Runtime and one extensions (subscribed). Runtime doesn't terminate gracefully, but extension shutdown still has dedicated time -> success",
			runtimeShutdownTimeout: 30 * time.Millisecond,
			shutdownTimeout:        100 * time.Millisecond,
			shutdownReason:         model.Spindown,
			agents: []Agent{
				{
					name:       "agent1",
					subscribed: true,
				},
			},

			runtimeTermination: func(s *shutdownContext, supv *supervisormodel.MockProcessSupervisor) {
				supv.On("Terminate", mock.Anything, mock.Anything).Return(nil)
				supv.On("Kill", mock.Anything, mock.Anything).Return(nil)

				go func() {

					runtimeExitedChannel, _ := s.getExitedChannel(runtimeProcessName)
					time.Sleep(50 * time.Millisecond)
					close(runtimeExitedChannel)
				}()
			},
			subscribedExtensionTermination: func(s *shutdownContext, supv *supervisormodel.MockProcessSupervisor) {
				go func() {

					extensionExitedChannel, _ := s.getExitedChannel(extensionProcessName("agent1"))
					close(extensionExitedChannel)
				}()
			},

			verifyShutdownResult: func(t *testing.T, err error, s *shutdownContext) {
				assert.Nil(t, err)
			},
		},
		{
			name:                   "Runtime and one extensions (not subscribed) and is killed successfully -> shutdown is success",
			runtimeShutdownTimeout: 30 * time.Millisecond,
			shutdownTimeout:        100 * time.Millisecond,
			shutdownReason:         model.Spindown,
			agents: []Agent{
				{
					name:       "agent1",
					subscribed: false,
				},
			},

			runtimeTermination: func(s *shutdownContext, supv *supervisormodel.MockProcessSupervisor) {
				supv.On("Terminate", mock.Anything, mock.Anything).Return(nil).
					Run(func(args mock.Arguments) { time.Sleep(10 * time.Millisecond) }).Once()

				runtimeExitedChannel, _ := s.getExitedChannel(runtimeProcessName)
				close(runtimeExitedChannel)
			},

			noSubscriptionExtensionTermination: func(s *shutdownContext, supv *supervisormodel.MockProcessSupervisor) {
				supv.On("Kill", mock.Anything, mock.Anything).Return(nil).Once()

				extensionExitedChannel, _ := s.getExitedChannel(extensionProcessName("agent1"))
				close(extensionExitedChannel)
			},

			verifyShutdownResult: func(t *testing.T, err error, s *shutdownContext) {
				assert.Nil(t, err)
			},
		},
		{
			name:                   "Runtime and one extensions (subscribed). Runtime terminate gracefully, but extension don't and needs to be killed -> success",
			runtimeShutdownTimeout: 30 * time.Millisecond,
			shutdownTimeout:        100 * time.Millisecond,
			shutdownReason:         model.Spindown,
			agents: []Agent{
				{
					name:       "agent1",
					subscribed: true,
				},
			},

			runtimeTermination: func(s *shutdownContext, supv *supervisormodel.MockProcessSupervisor) {
				supv.On("Terminate", mock.Anything, mock.Anything).Return(nil).Once()

				runtimeExitedChannel, _ := s.getExitedChannel(runtimeProcessName)
				close(runtimeExitedChannel)
			},

			subscribedExtensionTermination: func(s *shutdownContext, supv *supervisormodel.MockProcessSupervisor) {

				supv.On("Kill", mock.Anything, mock.Anything).Return(nil)
				go func() {

					extensionExitedChannel, _ := s.getExitedChannel(extensionProcessName("agent1"))
					time.Sleep(120 * time.Millisecond)
					close(extensionExitedChannel)
				}()
			},

			verifyShutdownResult: func(t *testing.T, err error, s *shutdownContext) {
				assert.Nil(t, err)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			supv := &supervisormodel.MockProcessSupervisor{}
			processSupervisor := processSupervisor{ProcessSupervisor: supv}
			renderingService := rendering.NewRenderingService()
			shutdownContext := newShutdownContext()
			shutdownContext.shutdownTimeout = tt.shutdownTimeout
			shutdownContext.runtimeShutdownTimeout = tt.runtimeShutdownTimeout

			defer mock.AssertExpectationsForObjects(t, supv)

			var agents []*core.ExternalAgent
			shutdownContext.createExitedChannel(runtimeProcessName)

			assert.NotNil(t, tt.runtimeTermination)
			tt.runtimeTermination(shutdownContext, supv)

			for _, a := range tt.agents {
				shutdownContext.createExitedChannel(extensionProcessName(a.name))

				initFlow := core.NewInitFlowSynchronization()
				extAgent := core.NewExternalAgent(a.name, initFlow)
				agents = append(agents, extAgent)

				if a.subscribed {
					require.NoError(t, extAgent.Register([]core.Event{core.ShutdownEvent}))

					assert.NotNil(t, tt.subscribedExtensionTermination)
					tt.subscribedExtensionTermination(shutdownContext, supv)
				} else {

					assert.NotNil(t, tt.noSubscriptionExtensionTermination)
					tt.noSubscriptionExtensionTermination(shutdownContext, supv)
				}
			}

			metrics := NewShutdownMetrics(nil, nil)

			err := shutdownContext.shutdown(
				processSupervisor,
				renderingService,
				agents,
				len(agents)+tt.internalAgentsCount,
				tt.shutdownReason,
				metrics,
				&telemetry.NoOpEventsAPI{},
			)

			tt.verifyShutdownResult(t, err, shutdownContext)
		})
	}
}

type mockInitFlowSynchronization struct {
	mock.Mock
}

var _ core.InitFlowSynchronization = (*mockInitFlowSynchronization)(nil)

func (m *mockInitFlowSynchronization) SetExternalAgentsRegisterCount(cnt uint16) error {
	args := m.Called(cnt)
	return args.Error(0)
}

func (m *mockInitFlowSynchronization) SetAgentsReadyCount(cnt uint16) error {
	args := m.Called(cnt)
	return args.Error(0)
}

func (m *mockInitFlowSynchronization) ExternalAgentRegistered() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockInitFlowSynchronization) AwaitExternalAgentsRegistered(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockInitFlowSynchronization) RuntimeReady() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockInitFlowSynchronization) AwaitRuntimeReady(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockInitFlowSynchronization) AgentReady() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockInitFlowSynchronization) AwaitAgentsReady(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockInitFlowSynchronization) CancelWithError(err error) {
	m.Called(err)
}

func (m *mockInitFlowSynchronization) Clear() {
	m.Called()
}
