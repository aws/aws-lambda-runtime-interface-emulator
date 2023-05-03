// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.amzn.com/lambda/supervisor/model"
)

func TestRuntimeDomainExec(t *testing.T) {
	supv := NewLocalSupervisor()
	err := supv.Exec(&model.ExecRequest{
		Domain: "runtime",
		Name:   "agent",
		Path:   "/bin/bash",
	})

	assert.Nil(t, err)
}

func TestInvalidRuntimeDomainExec(t *testing.T) {
	supv := NewLocalSupervisor()
	err := supv.Exec(&model.ExecRequest{
		Domain: "runtime",
		Name:   "agent",
		Path:   "/bin/none",
	})

	require.Error(t, err)
}

func TestEvents(t *testing.T) {
	supv := NewLocalSupervisor()
	client := supv.SupervisorClient.(*LocalSupervisor)
	sync := make(chan struct{})
	go func() {
		evt, ok := <-client.events
		require.True(t, ok)
		termination := evt.Event.ProcessTerminated()
		require.NotNil(t, termination)
		assert.Equal(t, "runtime", *termination.Domain)
		assert.Equal(t, "agent", *termination.Name)
		sync <- struct{}{}
	}()

	err := supv.Exec(&model.ExecRequest{
		Domain: "runtime",
		Name:   "agent",
		Path:   "/bin/bash",
	})
	require.NoError(t, err)
	<-sync
}

func TestTerminate(t *testing.T) {
	supv := NewLocalSupervisor()
	client := supv.SupervisorClient.(*LocalSupervisor)
	err := supv.Exec(&model.ExecRequest{
		Domain: "runtime",
		Name:   "agent",
		Path:   "/bin/bash",
		Args:   []string{"-c", "sleep 10s"},
	})
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	err = supv.Terminate(&model.TerminateRequest{
		Domain: "runtime",
		Name:   "agent",
	})
	require.NoError(t, err)
	// wait for process exit notification
	ev := <-client.events
	require.NotNil(t, ev.Event.ProcessTerminated())
	term := *ev.Event.ProcessTerminated()
	require.Nil(t, term.Exited())
	require.NotNil(t, term.Signaled())
	require.EqualValues(t, syscall.SIGTERM, *term.Signo)
}

// Termiante should not fail if the message is not delivered
func TestTerminateExited(t *testing.T) {
	supv := NewLocalSupervisor()
	err := supv.Exec(&model.ExecRequest{
		Domain: "runtime",
		Name:   "agent",
		Path:   "/bin/bash",
	})
	require.NoError(t, err)
	// wait a short bit for bash to exit
	time.Sleep(100 * time.Millisecond)
	err = supv.Terminate(&model.TerminateRequest{
		Domain: "runtime",
		Name:   "agent",
	})
	require.NoError(t, err)
}

func TestKill(t *testing.T) {
	supv := NewLocalSupervisor()
	client := supv.SupervisorClient.(*LocalSupervisor)
	err := supv.Exec(&model.ExecRequest{
		Domain: "runtime",
		Name:   "agent",
		Path:   "/bin/bash",
		Args:   []string{"-c", "sleep 10s"},
	})
	require.NoError(t, err)
	err = supv.Kill(&model.KillRequest{
		Domain: "runtime",
		Name:   "agent",
	})
	require.NoError(t, err)
	timer := time.NewTimer(50 * time.Millisecond)
	select {
	case _, ok := <-client.events:
		assert.True(t, ok)
	case <-timer.C:
		require.Fail(t, "Process should have exited by the time kill returns")
	}
}

func TestKillExited(t *testing.T) {
	supv := NewLocalSupervisor()
	client := supv.SupervisorClient.(*LocalSupervisor)
	err := supv.Exec(&model.ExecRequest{
		Domain: "runtime",
		Name:   "agent",
		Path:   "/bin/bash",
	})
	require.NoError(t, err)
	//wait for natural exit event
	<-client.events
	err = supv.Kill(&model.KillRequest{
		Domain: "runtime",
		Name:   "agent",
	})
	require.NoError(t, err, "Kill should succeed for exited processes")
}

func TestKillUnknown(t *testing.T) {
	supv := NewLocalSupervisor()
	err := supv.Kill(&model.KillRequest{
		Domain: "runtime",
		Name:   "unknown",
	})
	require.Error(t, err)
	var supvError *model.SupervisorError
	assert.True(t, errors.As(err, &supvError))
	assert.Equal(t, supvError.Kind, model.NoSuchEntity)
}

func TestShutdown(t *testing.T) {
	supv := NewLocalSupervisor()
	client := supv.SupervisorClient.(*LocalSupervisor)
	log.Debug("hello")
	// start a bunch of processes, some short running, some longer running
	err := supv.Exec(&model.ExecRequest{
		Domain: "runtime",
		Name:   "agent-0",
		Path:   "/bin/bash",
		Args:   []string{"-c", "sleep 1s"},
	})
	require.NoError(t, err)

	err = supv.Exec(&model.ExecRequest{
		Domain: "runtime",
		Name:   "agent-1",
		Path:   "/bin/bash",
	})
	require.NoError(t, err)

	err = supv.Exec(&model.ExecRequest{
		Domain: "runtime",
		Name:   "agent-2",
		Path:   "/bin/bash",
		Args:   []string{"-c", "sleep 2s"},
	})
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	err = supv.Stop(&model.StopRequest{
		Domain: "runtime",
	})
	require.NoError(t, err)
	// Shutdown is expected to block untill all processes have exited
	expected := map[string]struct{}{
		"agent-0": {},
		"agent-1": {},
		"agent-2": {},
	}
	done := false
	timer := time.NewTimer(200 * time.Millisecond)
	for !done {
		select {
		case ev := <-client.events:
			data := ev.Event.ProcessTerminated()
			assert.NotNil(t, data)
			_, ok := expected[*data.Name]
			assert.True(t, ok)
			delete(expected, *data.Name)
		case <-timer.C:
			fmt.Print(expected)
			assert.Equal(t, 0, len(expected), "All process should terminate at shutdown")
			done = true
		}
	}
}
