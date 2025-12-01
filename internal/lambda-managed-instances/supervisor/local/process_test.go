// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/supervisor/model"
)

func TestRuntimeExec(t *testing.T) {
	supv := NewProcessSupervisor(WithProcessCredential(nil), WithLowerPriorities(false))
	err := supv.Exec(context.Background(), &model.ExecRequest{
		Name: "agent",
		Path: "/bin/bash",
	})

	assert.Nil(t, err)
}

func TestInvalidRuntimeExec(t *testing.T) {
	supv := NewProcessSupervisor()
	err := supv.Exec(context.Background(), &model.ExecRequest{
		Name: "agent",
		Path: "/bin/none",
	})

	require.Error(t, err)
}

func TestEvents(t *testing.T) {
	supv := NewProcessSupervisor(WithProcessCredential(nil), WithLowerPriorities(false))
	sync := make(chan struct{})
	go func() {
		eventCh, err := supv.Events(context.Background())
		require.NoError(t, err)

		evt, ok := <-eventCh
		require.True(t, ok)
		termination := evt.Event.ProcessTerminated()
		require.NotNil(t, termination)
		assert.Equal(t, "agent", termination.Name)
		sync <- struct{}{}
	}()

	err := supv.Exec(context.Background(), &model.ExecRequest{
		Name: "agent",
		Path: "/bin/bash",
	})
	require.NoError(t, err)
	<-sync
}

func TestTerminate(t *testing.T) {
	supv := NewProcessSupervisor(WithProcessCredential(nil), WithLowerPriorities(false))
	err := supv.Exec(context.Background(), &model.ExecRequest{
		Name: "agent",
		Path: "/bin/bash",
		Args: []string{"-c", "sleep 10s"},
	})
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	err = supv.Terminate(context.Background(), &model.TerminateRequest{
		Name: "agent",
	})
	require.NoError(t, err)

	eventCh, err := supv.Events(context.Background())
	require.NoError(t, err)
	ev := <-eventCh
	require.NotNil(t, ev.Event.ProcessTerminated())

}

func TestStopAll(t *testing.T) {
	supv := NewProcessSupervisor(WithProcessCredential(nil), WithLowerPriorities(false))

	err := supv.Exec(context.Background(), &model.ExecRequest{
		Name: "agent1",
		Path: "/bin/bash",
		Args: []string{"-c", "sleep 10s"},
	})
	require.NoError(t, err)

	err = supv.Exec(context.Background(), &model.ExecRequest{
		Name: "agent2",
		Path: "/bin/bash",
		Args: []string{"-c", "sleep 10s"},
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	err = supv.StopAll(context.Background(), time.Now().Add(1*time.Second))
	require.NoError(t, err)
}

func TestTerminateExited(t *testing.T) {
	supv := NewProcessSupervisor(WithProcessCredential(nil), WithLowerPriorities(false))
	err := supv.Exec(context.Background(), &model.ExecRequest{
		Name: "agent",
		Path: "/bin/bash",
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	err = supv.Terminate(context.Background(), &model.TerminateRequest{
		Name: "agent",
	})
	require.NoError(t, err)
}

func TestKill(t *testing.T) {
	supv := NewProcessSupervisor(WithProcessCredential(nil), WithLowerPriorities(false))
	err := supv.Exec(context.Background(), &model.ExecRequest{
		Name: "agent",
		Path: "/bin/bash",
		Args: []string{"-c", "sleep 10s"},
	})
	require.NoError(t, err)
	err = supv.Kill(context.Background(), &model.KillRequest{
		Name:     "agent",
		Deadline: time.Now().Add(time.Second),
	})
	require.NoError(t, err)
	timer := time.NewTimer(50 * time.Millisecond)
	eventCh, err := supv.Events(context.Background())
	require.NoError(t, err)

	select {
	case _, ok := <-eventCh:
		assert.True(t, ok)
	case <-timer.C:
		require.Fail(t, "Process should have exited by the time kill returns")
	}
}

func TestKillExited(t *testing.T) {
	supv := NewProcessSupervisor(WithProcessCredential(nil), WithLowerPriorities(false))
	err := supv.Exec(context.Background(), &model.ExecRequest{
		Name: "agent",
		Path: "/bin/bash",
	})
	require.NoError(t, err)

	eventCh, err := supv.Events(context.Background())
	require.NoError(t, err)
	<-eventCh
	err = supv.Kill(context.Background(), &model.KillRequest{
		Name:     "agent",
		Deadline: time.Now().Add(time.Second),
	})
	require.NoError(t, err, "Kill should succeed for exited processes")
}

func TestKillUnknown(t *testing.T) {
	supv := NewProcessSupervisor()
	err := supv.Kill(context.Background(), &model.KillRequest{
		Name:     "unknown",
		Deadline: time.Now().Add(time.Second),
	})
	require.Error(t, err)
	var supvError *model.SupervisorError
	assert.True(t, errors.As(err, &supvError))
	assert.Equal(t, supvError.Reason(), "ProcessNotFound")
}

func TestTerminateUnknown(t *testing.T) {
	supv := NewProcessSupervisor()
	err := supv.Terminate(context.Background(), &model.TerminateRequest{
		Name: "unknown",
	})
	require.Error(t, err)
	var supvError *model.SupervisorError
	assert.True(t, errors.As(err, &supvError))
	assert.Equal(t, supvError.Reason(), "ProcessNotFound")
}

func TestEventsChannelShouldReturnEventsForRuntime(t *testing.T) {
	supv := NewProcessSupervisor(WithProcessCredential(nil), WithLowerPriorities(false))
	ch, err := supv.Events(context.Background())
	assert.NoError(t, err)
	err = supv.Exec(context.Background(), &model.ExecRequest{
		Path: "sleep",
		Args: []string{"0.001s"},
	})
	assert.NoError(t, err)

	timeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	select {
	case <-ch:
	case <-timeout.Done():
		t.Error("We should get a message from the sleep binary")
	}
}

func TestTerminateCheckStatus(t *testing.T) {
	supv := NewProcessSupervisor(WithProcessCredential(nil), WithLowerPriorities(false))
	err := supv.Exec(context.Background(), &model.ExecRequest{
		Name: "agent",
		Path: "/bin/bash",
		Args: []string{"-c", "sleep 10s"},
	})
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	err = supv.Terminate(context.Background(), &model.TerminateRequest{
		Name: "agent",
	})
	require.NoError(t, err)

	eventCh, err := supv.Events(context.Background())
	require.NoError(t, err)
	ev := <-eventCh
	require.NotNil(t, ev.Event.ProcessTerminated())
	term := *ev.Event.ProcessTerminated()
	require.Nil(t, term.Exited())
	require.NotNil(t, term.Signaled())
	require.EqualValues(t, syscall.SIGTERM, *term.Signo)
}
