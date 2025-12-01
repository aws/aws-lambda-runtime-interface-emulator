// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"syscall"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
)

type EventType string

const (
	ProcessTerminationType EventType = "process_termination"
	EventLossType          EventType = "event_loss"
)

type ProcessTerminationCause string

const (
	Signaled  ProcessTerminationCause = "signaled"
	Exited    ProcessTerminationCause = "exited"
	OomKilled ProcessTerminationCause = "oom_killed"
)

type ProcessSupervisor interface {
	Exec(context.Context, *ExecRequest) error
	Terminate(context.Context, *TerminateRequest) error
	Kill(context.Context, *KillRequest) error
	Events(context.Context) (<-chan Event, error)
}

type ProcessSupervisorClient interface {
	ProcessSupervisor
}

type Event struct {
	Time  int64     `json:"timestamp_millis"`
	Event EventData `json:"event"`
}

func (e Event) String() string {
	return fmt.Sprintf("{Time:%s Event:%s}", time.UnixMilli(e.Time).UTC().Format("2006-01-02T15:04:05.000Z"), e.Event)
}

type EventData struct {
	EvType     EventType               `json:"type"`
	Name       string                  `json:"name"`
	Cause      ProcessTerminationCause `json:"cause"`
	Signo      *int32                  `json:"signo"`
	ExitStatus *int32                  `json:"exit_status"`
	Size       *uint64                 `json:"size"`
}

func (d EventData) String() string {
	signo := "<nil>"
	if d.Signo != nil {
		signo = strconv.FormatInt(int64(*d.Signo), 10)
	}
	exitStatus := "<nil>"
	if d.ExitStatus != nil {
		exitStatus = strconv.FormatInt(int64(*d.ExitStatus), 10)
	}
	size := "<nil>"
	if d.Size != nil {
		size = strconv.FormatUint(*d.Size, 10)
	}
	return fmt.Sprintf("{EvType:%s Name:%s Cause:%s Signo:%s ExitStatus:%s Size:%s}", d.EvType, d.Name, d.Cause, signo, exitStatus, size)
}

func (d EventData) ProcessTerminated() *ProcessTermination {
	return &ProcessTermination{
		Name:       d.Name,
		Cause:      d.Cause,
		Signo:      d.Signo,
		ExitStatus: d.ExitStatus,
	}
}

type ProcessTermination struct {
	Name       string
	Cause      ProcessTerminationCause
	Signo      *int32
	ExitStatus *int32
}

func (t ProcessTermination) Signaled() *int32 {
	if t.Cause != Signaled {
		return nil
	}
	return t.Signo
}

func (t ProcessTermination) Exited() *int32 {
	if t.Cause != Exited {
		return nil
	}
	return t.ExitStatus
}

func (t ProcessTermination) ExitedWithZeroExitCode() bool {
	if t.Cause != Exited {
		return false
	}
	return t.ExitStatus != nil && *t.ExitStatus == 0
}

func (t ProcessTermination) OomKilled() bool {
	return t.Cause == OomKilled
}

func (t ProcessTermination) String() string {
	if t.ExitStatus != nil {
		return fmt.Sprintf("exit status %d", *t.ExitStatus)
	}
	if t.Signo != nil {
		sig := syscall.Signal(*t.Signo)
		return fmt.Sprintf("signal: %s", sig.String())
	}

	return "signal: killed"
}

type ExecRequest struct {
	Name string `json:"name"`

	Path string   `json:"path"`
	Args []string `json:"args,omitempty"`

	Cwd          *string      `json:"cwd,omitempty"`
	Env          *model.KVMap `json:"env,omitempty"`
	Logging      Logging      `json:"log_config"`
	StdoutWriter io.Writer    `json:"-"`
	StderrWriter io.Writer    `json:"-"`
}

type Logging struct {
	Managed ManagedLogging `json:"managed"`
}

type ManagedLogging struct {
	Topic   ManagedLoggingTopic    `json:"topic"`
	Formats []ManagedLoggingFormat `json:"formats"`
}

type ManagedLoggingTopic string

const (
	RuntimeManagedLoggingTopic     ManagedLoggingTopic = "runtime"
	RtExtensionManagedLoggingTopic ManagedLoggingTopic = "runtime_extension"
)

type ManagedLoggingFormat string

const (
	LineBasedManagedLogging    ManagedLoggingFormat = "line"
	MessageBasedManagedLogging ManagedLoggingFormat = "message"
)

type LockHardError interface {
	Source() string

	Reason() string

	Cause() string

	HookName() string
}

var _ error = (*SupervisorError)(nil)

type ErrorExitCode uint8

type SupervisorError struct {
	SourceErr   ErrorSource   `json:"source"`
	HookNameErr string        `json:"hook_name"`
	ExitCodeErr ErrorExitCode `json:"exit_code"`
	ReasonErr   string        `json:"reason"`
	CauseErr    string        `json:"cause"`
}

type ErrorSource string

const (
	ErrorSourceClient   ErrorSource = "Client"
	ErrorSourceServer   ErrorSource = "Server"
	ErrorSourceFunction ErrorSource = "Customer"
	ErrorSourceHook     ErrorSource = "Hook"
)

type ErrorReason string

func (l *SupervisorError) Reason() string {
	return l.ReasonErr
}

func (l *SupervisorError) Source() ErrorSource {
	return l.SourceErr
}

func (l *SupervisorError) Cause() string {
	return l.CauseErr
}

func (l *SupervisorError) HookName() string {
	return l.HookNameErr
}

func (l *SupervisorError) ExitCode() ErrorExitCode {
	return l.ExitCodeErr
}

func (l *SupervisorError) Error() string {
	return string(l.ReasonErr)
}

type TerminateRequest struct {
	Name string `json:"name"`
}

type KillRequest struct {
	Name     string    `json:"name"`
	Deadline time.Time `json:"deadline"`
}
