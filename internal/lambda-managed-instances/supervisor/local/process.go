// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapidcore/env"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/supervisor/model"
)

var _ model.ProcessSupervisorClient = (*ProcessSupervisor)(nil)

type process struct {
	pid int

	termination chan struct{}
}

type ProcessSupervisor struct {
	events               chan model.Event
	processMapLock       sync.Mutex
	credential           *syscall.Credential
	processMap           map[string]process
	freezeThawCycleStart time.Time
	msgLogFile           *os.File
	lowerPriorities      bool
}

type ProcessSupervisorOption func(ls *ProcessSupervisor)

func MsgLogFile(file *os.File) ProcessSupervisorOption {
	return func(ls *ProcessSupervisor) {
		ls.msgLogFile = file
	}
}

func WithProcessCredential(credential *syscall.Credential) ProcessSupervisorOption {
	return func(ls *ProcessSupervisor) {
		ls.credential = credential
	}
}

func WithLowerPriorities(lowerPriorities bool) ProcessSupervisorOption {
	return func(ls *ProcessSupervisor) {
		ls.lowerPriorities = lowerPriorities
	}
}

func NewProcessSupervisor(opts ...ProcessSupervisorOption) *ProcessSupervisor {
	ls := &ProcessSupervisor{
		events:     make(chan model.Event),
		processMap: make(map[string]process),

		credential: &syscall.Credential{
			Uid: uint32(993),
			Gid: uint32(990),
		},
		lowerPriorities: true,
	}

	for _, option := range opts {
		option(ls)
	}

	return ls
}

func (*ProcessSupervisor) Exit(ctx context.Context) {}

func (s *ProcessSupervisor) Exec(ctx context.Context, req *model.ExecRequest) error {

	command := exec.Command(req.Path, req.Args...)
	if req.Env != nil {
		command.Env = env.MapToKVPairStrings(*req.Env)
	}

	if req.Cwd != nil && *req.Cwd != "" {
		command.Dir = *req.Cwd
	}

	for _, format := range req.Logging.Managed.Formats {
		if format == model.MessageBasedManagedLogging {
			if s.msgLogFile == nil {
				return errors.New("invalid message logging setup: could not find message log fd")
			}

			command.ExtraFiles = []*os.File{s.msgLogFile}
			command.Env = append(command.Env, "_LAMBDA_TELEMETRY_LOG_FD=3")
		}
	}

	command.Stdout = req.StdoutWriter
	command.Stderr = req.StderrWriter

	command.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:    true,
		Credential: s.credential,
	}

	err := command.Start()
	if err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	slog.Info("LocalProcessSupervisor.Exec",
		"pid", strconv.Itoa(command.Process.Pid),
		"path", command.Path,
		"args", command.Args)

	pid := command.Process.Pid
	termination := make(chan struct{})
	s.processMapLock.Lock()
	s.processMap[req.Name] = process{
		pid:         pid,
		termination: termination,
	}
	s.processMapLock.Unlock()

	s.freezeThawCycleStart = time.Now()

	if err := s.setProcessGroupPriorities(pid); err != nil {
		return fmt.Errorf("could not set process priorities: %w", err)
	}

	go func() {
		err = command.Wait()

		close(termination)

		var cell int32
		var exitStatus *int32
		var signo *int32
		var exitErr *exec.ExitError
		var cause model.ProcessTerminationCause

		if err == nil {

			exitStatus = &cell
			cause = model.Exited
		} else if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if code := status.ExitStatus(); code >= 0 {

					cell = int32(code)
					exitStatus = &cell
					cause = model.Exited
				} else {

					cell = int32(status.Signal())
					signo = &cell
					cause = model.Signaled
				}
			}
		}

		if signo == nil && exitStatus == nil {
			slog.Error("Cannot convert process exit status to unix WaitStatus. This is unexpected. Assuming ExitStatus 1")
			cell = 1
			exitStatus = &cell
			cause = model.Exited
		}

		s.events <- model.Event{
			Time: time.Now().UnixMilli(),
			Event: model.EventData{
				EvType:     model.ProcessTerminationType,
				Name:       req.Name,
				Cause:      cause,
				Signo:      signo,
				ExitStatus: exitStatus,
			},
		}
	}()

	return nil
}

func (s *ProcessSupervisor) StopAll(ctx context.Context, deadline time.Time) error {

	s.processMapLock.Lock()
	defer s.processMapLock.Unlock()

	nprocs := len(s.processMap)

	successes := make(chan struct{})
	errors := make(chan error)
	for name, proc := range s.processMap {
		go func(n string, p process) {
			slog.Debug("Killing", "name", n)
			err := kill(p, n, deadline)
			if err != nil {
				errors <- err
			} else {
				successes <- struct{}{}
			}
		}(name, proc)
	}

	var err error
	for i := 0; i < nprocs; i++ {
		select {
		case <-successes:
		case e := <-errors:
			if err == nil {
				err = fmt.Errorf("shutdown failed: %s", e.Error())
			}
		}
	}

	s.processMap = make(map[string]process)

	return err
}

func kill(p process, name string, deadline time.Time) error {

	select {

	case <-p.termination:
		slog.Debug("Process already terminated", "name", name)
		return nil
	default:
		slog.Info("Sending SIGKILL to process", "name", name, "pid", p.pid)
	}

	if (time.Since(deadline)) > 0 {
		return fmt.Errorf("invalid timeout while killing %s", name)
	}

	pgid, err := syscall.Getpgid(p.pid)

	if err == nil {

		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	} else {
		_ = syscall.Kill(p.pid, syscall.SIGKILL)
	}

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	select {
	case <-p.termination:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timed out while trying to SIGKILL %s", name)
	}
}

func (s *ProcessSupervisor) Kill(ctx context.Context, req *model.KillRequest) error {
	s.processMapLock.Lock()
	process, ok := s.processMap[req.Name]
	s.processMapLock.Unlock()
	if !ok {
		msg := "Unknown process"
		return &model.SupervisorError{
			SourceErr: model.ErrorSourceClient,
			ReasonErr: "ProcessNotFound",
			CauseErr:  msg,
		}
	}

	return kill(process, req.Name, req.Deadline)
}

func (s *ProcessSupervisor) Terminate(ctx context.Context, req *model.TerminateRequest) error {
	s.processMapLock.Lock()
	process, ok := s.processMap[req.Name]
	pid := process.pid
	s.processMapLock.Unlock()
	if !ok {
		msg := "Unknown process"
		err := &model.SupervisorError{
			SourceErr: model.ErrorSourceClient,
			ReasonErr: "ProcessNotFound",
			CauseErr:  msg,
		}
		slog.Error("Process not found in local supervisor map", "name", req.Name, "err", err)
		return err
	}

	pgid, err := syscall.Getpgid(pid)

	if err == nil {

		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}

	return nil
}

func (s *ProcessSupervisor) Events(ctx context.Context) (<-chan model.Event, error) {
	return s.events, nil
}

func (s *ProcessSupervisor) setProcessGroupPriorities(pid int) error {
	if !s.lowerPriorities {
		return nil
	}

	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return fmt.Errorf("could not get pgid for pid %d: %w", pid, err)
	}

	if err := syscall.Setpriority(syscall.PRIO_PGRP, pgid, 10); err != nil {
		return fmt.Errorf("failed to set nice score for %d: %w", pgid, err)
	}

	return nil
}
