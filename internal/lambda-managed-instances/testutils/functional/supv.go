// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build test

package functional

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/supervisor/model"
)

type process interface {
	Exec(request *model.ExecRequest) (doneCh <-chan struct{}, err error)
	Terminate() error
	Kill() error
}

type MockSupervisor struct {
	t         *testing.T
	ps        map[string]process
	eventsCh  chan model.Event
	eventsErr error
}

func NewMockSupervisor(t *testing.T, runtime process, extensions ExtensionsEnv, eventsErr error) *MockSupervisor {
	ps := map[string]process{
		"runtime": runtime,
	}
	for name, proc := range extensions {
		ps["extension-"+name] = proc
	}
	return &MockSupervisor{
		t:         t,
		ps:        ps,
		eventsCh:  make(chan model.Event),
		eventsErr: eventsErr,
	}
}

func (m *MockSupervisor) Exec(_ context.Context, request *model.ExecRequest) error {
	proc, ok := m.ps[request.Name]
	if !ok {
		m.t.Fatalf("unknown process name: %s", request.Name)
	}
	doneCh, err := proc.Exec(request)
	if err != nil {
		return err
	}
	go func() {
		<-doneCh
		var zero int32
		event := model.Event{
			Time: time.Now().UnixMilli(),
			Event: model.EventData{
				EvType:     model.ProcessTerminationType,
				Name:       request.Name,
				Cause:      model.Exited,
				ExitStatus: &zero,
			},
		}
		m.eventsCh <- event
	}()
	return nil
}

func (m *MockSupervisor) Terminate(_ context.Context, request *model.TerminateRequest) error {
	proc, ok := m.ps[request.Name]
	if !ok {
		m.t.Fatalf("unknown process name: %s", request.Name)
	}
	return proc.Terminate()
}

func (m *MockSupervisor) Kill(_ context.Context, request *model.KillRequest) error {
	proc, ok := m.ps[request.Name]
	if !ok {
		m.t.Fatalf("unknown process name: %s", request.Name)
	}
	return proc.Kill()
}

func (m *MockSupervisor) Events(_ context.Context) (<-chan model.Event, error) {
	return m.eventsCh, m.eventsErr
}
