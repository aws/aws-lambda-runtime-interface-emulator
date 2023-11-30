// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/supervisor/model"
)

// typecheck interface compliance
var _ model.SupervisorClient = (*LocalSupervisor)(nil)

type process struct {
	// pid of the running process
	pid int
	// channel that can be use to block
	// while waiting on process termination.
	termination chan struct{}
}

type LocalSupervisor struct {
	events               chan model.Event
	processMapLock       sync.Mutex
	processMap           map[string]process
	freezeThawCycleStart time.Time

	RootPath string
}

func NewLocalSupervisor() *LocalSupervisor {
	return &LocalSupervisor{
		events:     make(chan model.Event),
		processMap: make(map[string]process),
		RootPath:   "/",
	}
}

func (*LocalSupervisor) Start(ctx context.Context, req *model.StartRequest) error {
	return nil
}
func (*LocalSupervisor) Configure(ctx context.Context, req *model.ConfigureRequest) error {
	return nil
}
func (*LocalSupervisor) Exit(ctx context.Context) {}

func (s *LocalSupervisor) Exec(ctx context.Context, req *model.ExecRequest) error {
	if req.Domain != "runtime" {
		log.Debug("Exec is a no op if domain != runtime")
		return nil
	}
	command := exec.Command(req.Path, req.Args...)

	if req.Env != nil {
		envStrings := make([]string, 0, len(*req.Env))
		for key, value := range *req.Env {
			envStrings = append(envStrings, key+"="+value)
		}
		command.Env = envStrings
	}

	if req.Cwd != nil && *req.Cwd != "" {
		command.Dir = *req.Cwd
	}

	if req.ExtraFiles != nil {
		command.ExtraFiles = *req.ExtraFiles
	}

	command.Stdout = req.StdoutWriter
	command.Stderr = req.StderrWriter

	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	err := command.Start()

	if err != nil {
		return err
		// TODO Use supevisor specific error
	}

	pid := command.Process.Pid
	termination := make(chan struct{})
	s.processMapLock.Lock()
	s.processMap[req.Name] = process{
		pid:         pid,
		termination: termination,
	}
	s.processMapLock.Unlock()

	// The first freeze thaw cycle starts on Exec() at init time
	s.freezeThawCycleStart = time.Now()

	go func() {
		err = command.Wait()
		// close the termination channel to unblock whoever's blocked on
		// it (used to implement kill's blocking behaviour)
		close(termination)

		var cell int32
		var exitStatus *int32
		var signo *int32
		var exitErr *exec.ExitError

		if err == nil {
			exitStatus = &cell
		} else if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if code := status.ExitStatus(); code >= 0 {
					cell = int32(code)
					exitStatus = &cell
				} else {
					cell = int32(status.Signal())
					signo = &cell
				}
			}
		}

		if signo == nil && exitStatus == nil {
			log.Error("Cannot convert process exit status to unix WaitStatus. This is unexpected. Assuming ExitStatus 1")
			cell = 1
			exitStatus = &cell
		}
		s.events <- model.Event{
			Time: uint64(time.Now().UnixMilli()),
			Event: model.EventData{
				Domain:     &req.Domain,
				Name:       &req.Name,
				Signo:      signo,
				ExitStatus: exitStatus,
			},
		}
	}()

	return nil
}

func kill(p process, name string, deadline time.Time) error {
	// kill should report success if the process terminated by the time
	//supervisor receives the request.
	select {
	// if this case is selected, the channel is closed,
	// which means the process is terminated
	case <-p.termination:
		log.Debugf("Process %s already terminated.", name)
		return nil
	default:
		log.Infof("Sending SIGKILL to %s(%d).", name, p.pid)
	}

	if (time.Since(deadline)) > 0 {
		return fmt.Errorf("invalid timeout while killing %s", name)
	}

	pgid, err := syscall.Getpgid(p.pid)

	if err == nil {
		// Negative pid sends signal to all in process group
		syscall.Kill(-pgid, syscall.SIGKILL)
	} else {
		syscall.Kill(p.pid, syscall.SIGKILL)
	}

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	// block until the (main) process exits
	// or the timeout fires
	select {
	case <-p.termination:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timed out while trying to SIGKILL %s", name)
	}
}

func (s *LocalSupervisor) Kill(ctx context.Context, req *model.KillRequest) error {
	if req.Domain != "runtime" {
		log.Debug("Kill is a no op if domain != runtime")
		return nil
	}
	s.processMapLock.Lock()
	process, ok := s.processMap[req.Name]
	s.processMapLock.Unlock()
	if !ok {
		msg := "Unknown process"
		return &model.SupervisorError{
			Kind:    model.NoSuchEntity,
			Message: &msg,
		}
	}

	return kill(process, req.Name, req.Deadline)
}

func (s *LocalSupervisor) Terminate(ctx context.Context, req *model.TerminateRequest) error {
	if req.Domain != "runtime" {
		log.Debug("Terminate is no op if domain != runtime")
		return nil
	}
	s.processMapLock.Lock()
	process, ok := s.processMap[req.Name]
	pid := process.pid
	s.processMapLock.Unlock()
	if !ok {
		msg := "Unknown process"
		err := &model.SupervisorError{
			Kind:    model.NoSuchEntity,
			Message: &msg,
		}
		log.WithError(err).Errorf("Process %s not found in local supervisor map", req.Name)
		return err
	}

	pgid, err := syscall.Getpgid(pid)

	if err == nil {
		// Negative pid sends signal to all in process group
		// best effort, ignore errors
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}

	return nil
}

func (s *LocalSupervisor) Stop(ctx context.Context, req *model.StopRequest) (*model.StopResponse, error) {
	if req.Domain != "runtime" {
		log.Debug("Shutdown is no op if domain != runtime")
		return &model.StopResponse{}, nil
	}

	// shut down kills all the processes in the map
	s.processMapLock.Lock()
	defer s.processMapLock.Unlock()

	nprocs := len(s.processMap)

	successes := make(chan struct{})
	errors := make(chan error)
	for name, proc := range s.processMap {
		go func(n string, p process) {
			log.Debugf("Killing %s", n)
			err := kill(p, n, req.Deadline)
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
	return nil, err
}

func (s *LocalSupervisor) Freeze(ctx context.Context, req *model.FreezeRequest) (*model.FreezeResponse, error) {
	// We return mocked freeze/thaw cycle metrics to mimic usage metrics in standalone mode
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return &model.FreezeResponse{
		CycleDeltaMetrics: model.CycleDeltaMetrics{
			DomainCPURunNs:            uint64(time.Since(s.freezeThawCycleStart).Nanoseconds()),
			DomainRunNs:               uint64(time.Since(s.freezeThawCycleStart).Nanoseconds()),
			DomainMaxMemoryUsageBytes: m.Alloc,
			MicrovmCPURunNs:           uint64(time.Since(s.freezeThawCycleStart).Nanoseconds()),
		},
	}, nil
}
func (s *LocalSupervisor) Thaw(ctx context.Context, req *model.ThawRequest) error {
	s.freezeThawCycleStart = time.Now()
	return nil
}
func (s *LocalSupervisor) Ping(ctx context.Context) error {
	return nil
}

func (s *LocalSupervisor) Events(ctx context.Context, req *model.EventsRequest) (<-chan model.Event, error) {
	return s.events, nil
}
