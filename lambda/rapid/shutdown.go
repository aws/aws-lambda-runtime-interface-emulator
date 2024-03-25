// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package rapid implements synchronous even dispatch loop.
package rapid

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/metering"
	"go.amzn.com/lambda/rapi/model"
	"go.amzn.com/lambda/rapi/rendering"
	supvmodel "go.amzn.com/lambda/supervisor/model"

	log "github.com/sirupsen/logrus"
)

const (
	// supervisor shutdown and kill operations block until the exit status of the
	// interested process has been collected, or until the specified deadline expires
	// Note that this deadline is mainly relevant when any of the domain
	// processes are in uninterruptible sleep state (notable examples: syscall
	// to read/write a networked driver)
	//
	// We set a non nil value for these timeouts so that RAPID doesn't block
	// forever in one of the cases above.
	supervisorBlockingMaxMillis = 9000
	runtimeDeadlineShare        = 0.3

	maxProcessExitWait = 2 * time.Second
)

// TODO: aggregate struct's methods into an interface, so that we can mock in tests
type shutdownContext struct {
	// Adding a mutex around shuttingDown because there may be concurrent reads/writes.
	// Because the code in shutdown() and the seperate go routine created in setupEventsWatcher()
	// could be concurrently accessing the field shuttingDown.
	shuttingDownMutex  sync.Mutex
	shuttingDown       bool
	agentsAwaitingExit map[string]*core.ExternalAgent
	// Adding a mutex around runtimeDomainExited because there may be concurrent reads/writes.
	// The first reason this can be caused is by different go routines reading/writing different keys.
	// The second reason this can be caused is between the code shutting down the runtime/extensions and
	// handleProcessExit in a separate go routine, reading and writing to the same key. Caused by
	// unexpected exits.
	runtimeDomainExitedMutex sync.Mutex
	// used to synchronize on processes exits. We create the channel when a
	// process is started and we close it upon exit notification from
	// supervisor. Closing the channel is basically a persistent broadcast of process exit.
	// We never write anything to the channels
	runtimeDomainExited map[string]chan struct{}
}

func newShutdownContext() *shutdownContext {
	return &shutdownContext{
		shuttingDownMutex:        sync.Mutex{},
		shuttingDown:             false,
		agentsAwaitingExit:       make(map[string]*core.ExternalAgent),
		runtimeDomainExited:      make(map[string]chan struct{}),
		runtimeDomainExitedMutex: sync.Mutex{},
	}
}

func (s *shutdownContext) isShuttingDown() bool {
	s.shuttingDownMutex.Lock()
	defer s.shuttingDownMutex.Unlock()
	return s.shuttingDown
}

func (s *shutdownContext) setShuttingDown(value bool) {
	s.shuttingDownMutex.Lock()
	defer s.shuttingDownMutex.Unlock()
	s.shuttingDown = value
}

func (s *shutdownContext) handleProcessExit(termination supvmodel.ProcessTermination) {

	name := *termination.Name
	agent, found := s.agentsAwaitingExit[name]

	// If it is an agent registered to receive a shutdown event.
	if found {
		log.Debugf("Handling termination for %s", name)
		exitStatus := termination.Exited()
		if exitStatus != nil && *exitStatus == 0 {
			// If the agent exited by itself after receiving the shutdown event.
			stateErr := agent.Exited()
			if stateErr != nil {
				log.Warnf("%s failed to transition to EXITED: %s (current state: %s)", agent.String(), stateErr, agent.GetState().Name())
			}
		} else {
			// If the agent did not exit by itself, had to be SIGKILLed (only in standalone mode).
			stateErr := agent.ShutdownFailed()
			if stateErr != nil {
				log.Warnf("%s failed to transition to ShutdownFailed: %s (current state: %s)", agent, stateErr, agent.GetState().Name())
			}
		}
	}

	exitedChannel, found := s.getExitedChannel(name)

	if !found {
		log.Panicf("Unable to find an exitedChannel for '%s', it should have been created just after it was execed.", name)
	}
	// we close the channel so that whoever is blocked on it
	// or will try to block on it in the future unblocks immediately
	close(exitedChannel)
}

func (s *shutdownContext) getExitedChannel(name string) (chan struct{}, bool) {
	s.runtimeDomainExitedMutex.Lock()
	defer s.runtimeDomainExitedMutex.Unlock()
	exitedChannel, found := s.runtimeDomainExited[name]
	return exitedChannel, found
}

func (s *shutdownContext) createExitedChannel(name string) {
	s.runtimeDomainExitedMutex.Lock()
	defer s.runtimeDomainExitedMutex.Unlock()

	_, found := s.runtimeDomainExited[name]

	if found {
		log.Panicf("Tried to create an exited channel for '%s' but one already exists.", name)
	}
	s.runtimeDomainExited[name] = make(chan struct{})
}

// Blocks until all the processes in the runtime domain generation have exited.
// This helps us have a nice sync point on Shutdown where we know for sure that
// all the processes have exited and the state has been cleared. The exception
// to that rule is that if any of the processes don't exit within
// maxProcessExitWait from the beginning of the waiting period, an error is
// returned, in order to prevent it from waiting forever if any of the processes
// cannot be killed.
//
// It is OK not to hold the lock because we know that this is called only during
// shutdown and nobody will start a new process during shutdown
func (s *shutdownContext) clearExitedChannel() error {
	s.runtimeDomainExitedMutex.Lock()
	mapLen := len(s.runtimeDomainExited)
	channels := make([]chan struct{}, 0, mapLen)
	for _, v := range s.runtimeDomainExited {
		channels = append(channels, v)
	}
	s.runtimeDomainExitedMutex.Unlock()

	exitTimeout := time.After(maxProcessExitWait)
	for _, v := range channels {
		select {
		case <-v:
		case <-exitTimeout:
			return errors.New("timed out waiting for runtime processes to exit")
		}
	}

	s.runtimeDomainExitedMutex.Lock()
	s.runtimeDomainExited = make(map[string]chan struct{}, mapLen)
	s.runtimeDomainExitedMutex.Unlock()
	return nil
}

func (s *shutdownContext) shutdownRuntime(execCtx *rapidContext, start time.Time, deadline time.Time) {
	// If runtime is started:
	// 1. SIGTERM and wait until deadline
	// 2. SIGKILL on deadline
	log.Debug("Shutting down the runtime.")
	name := fmt.Sprintf("%s-%d", runtimeProcessName, execCtx.runtimeDomainGeneration)
	exitedChannel, found := s.getExitedChannel(name)

	if found {

		err := execCtx.supervisor.Terminate(context.Background(), &supvmodel.TerminateRequest{
			Domain: RuntimeDomain,
			Name:   name,
		})
		if err != nil {
			// We are not reporting the error upstream because we will anyway
			// shut the domain out at the end of the shutdown sequence
			log.WithError(err).Warn("Failed sending Termination signal to runtime")
		}

		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		select {
		case <-ctx.Done():
			log.Warnf("Deadline: The runtime did not exit after deadline %s; Killing it.", deadline)

			err = execCtx.supervisor.Kill(context.Background(), &supvmodel.KillRequest{
				Domain:   RuntimeDomain,
				Name:     name,
				Deadline: time.Now().Add(time.Millisecond * supervisorBlockingMaxMillis),
			})

			if err != nil {
				// We are not reporting the error upstream because we will anyway
				// shut the domain out at the end of the shutdown sequence
				log.WithError(err).Warn("Failed sending Kill signal to runtime")
			}
		case <-exitedChannel:
		}
	} else {
		log.Warn("The runtime was not started.")
	}
	log.Debug("Shutdown the runtime.")
}

func (s *shutdownContext) shutdownAgents(execCtx *rapidContext, start time.Time, deadline time.Time, reason string) {
	// For each external agent, if agent is launched:
	// 1. Send Shutdown event if subscribed for it, else send SIGKILL to process group
	// 2. Wait for all Shutdown-subscribed agents to exit with deadline
	// 3. Send SIGKILL to process group for Shutdown-subscribed agents on deadline

	log.Debug("Shutting down the agents.")
	execCtx.renderingService.SetRenderer(
		&rendering.ShutdownRenderer{
			AgentEvent: model.AgentShutdownEvent{
				AgentEvent: &model.AgentEvent{
					EventType:  "SHUTDOWN",
					DeadlineMs: deadline.UnixNano() / (1000 * 1000),
				},
				ShutdownReason: reason,
			},
		})

	var wg sync.WaitGroup

	// clear agentsAwaitingExit from last shutdownAgents
	s.agentsAwaitingExit = make(map[string]*core.ExternalAgent)

	for _, a := range execCtx.registrationService.GetExternalAgents() {
		name := fmt.Sprintf("extension-%s-%d", a.Name, execCtx.runtimeDomainGeneration)
		exitedChannel, found := s.getExitedChannel(name)

		if !found {
			log.Warnf("Agent %s failed to launch, therefore skipping shutting it down.", a)
			continue
		}

		wg.Add(1)

		if a.IsSubscribed(core.ShutdownEvent) {
			log.Debugf("Agent %s is registered for the shutdown event.", a)
			s.agentsAwaitingExit[name] = a

			go func(name string, agent *core.ExternalAgent) {
				defer wg.Done()

				agent.Release()

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if execCtx.standaloneMode {
					ctx, cancel = context.WithDeadline(ctx, deadline)
					defer cancel()
				}

				select {
				case <-ctx.Done():
					log.Warnf("Deadline: the agent %s did not exit after deadline %s; Killing it.", name, deadline)
					err := execCtx.supervisor.Kill(context.Background(), &supvmodel.KillRequest{
						Domain:   RuntimeDomain,
						Name:     name,
						Deadline: time.Now().Add(time.Millisecond * supervisorBlockingMaxMillis),
					})
					if err != nil {
						// We are not reporting the error upstream because we will anyway
						// shut the domain out at the end of the shutdown sequence
						log.WithError(err).Warn("Failed sending Kill signal to agent")
					}
				case <-exitedChannel:
				}
			}(name, a)
		} else {
			log.Debugf("Agent %s is not registered for the shutdown event, so just killing it.", a)

			go func(name string) {
				defer wg.Done()

				err := execCtx.supervisor.Kill(context.Background(), &supvmodel.KillRequest{
					Domain:   RuntimeDomain,
					Name:     name,
					Deadline: time.Now().Add(time.Millisecond * supervisorBlockingMaxMillis),
				})
				if err != nil {
					log.WithError(err).Warn("Failed sending Kill signal to agent")
				}
			}(name)
		}
	}

	// Wait on the agents subscribed to the shutdown event to voluntary shutting down after receiving the shutdown event or be sigkilled.
	// In addition to waiting on the agents not subscribed to the shutdown event being sigkilled.
	wg.Wait()
	log.Debug("Shutdown the agents.")
}

func (s *shutdownContext) shutdown(execCtx *rapidContext, deadlineNs int64, reason string) (int64, bool, error) {
	var err error
	s.setShuttingDown(true)
	defer s.setShuttingDown(false)

	// Fatal errors such as Runtime exit and Extension.Crash
	// are ignored by the events watcher when shutting down
	execCtx.appCtx.Delete(appctx.AppCtxFirstFatalErrorKey)

	runtimeDomainProfiler := &metering.ExtensionsResetDurationProfiler{}

	// We do not spend any compute time on runtime graceful shutdown if there are no agents
	if execCtx.registrationService.CountAgents() == 0 {
		name := fmt.Sprintf("%s-%d", runtimeProcessName, execCtx.runtimeDomainGeneration)

		_, found := s.getExitedChannel(name)

		if found {
			log.Debug("SIGKILLing the runtime as no agents are registered.")
			err = execCtx.supervisor.Kill(context.Background(), &supvmodel.KillRequest{
				Domain:   RuntimeDomain,
				Name:     name,
				Deadline: time.Now().Add(time.Millisecond * supervisorBlockingMaxMillis),
			})
			if err != nil {
				// We are not reporting the error upstream because we will anyway
				// shut the domain out at the end of the shutdown sequence
				log.WithError(err).Warn("Failed sending Kill signal to runtime")
			}
		} else {
			log.Debugf("Could not find runtime process %s in processes map. Already exited/never started", name)
		}
	} else {
		mono := metering.Monotime()
		availableNs := deadlineNs - mono

		if availableNs < 0 {
			log.Warnf("Deadline is in the past: %v, %v, %v", mono, deadlineNs, availableNs)
			availableNs = 0
		}

		start := time.Now()

		runtimeDeadline := start.Add(time.Duration(float64(availableNs) * runtimeDeadlineShare))
		agentsDeadline := start.Add(time.Duration(availableNs))

		runtimeDomainProfiler.AvailableNs = availableNs
		runtimeDomainProfiler.Start()

		s.shutdownRuntime(execCtx, start, runtimeDeadline)
		s.shutdownAgents(execCtx, start, agentsDeadline, reason)

		runtimeDomainProfiler.NumAgentsRegisteredForShutdown = len(s.agentsAwaitingExit)
	}

	log.Info("Waiting for runtime domain processes termination")
	if err := s.clearExitedChannel(); err != nil {
		log.Error(err)
	}

	runtimeDomainProfiler.Stop()
	extensionsResetMs, timeout := runtimeDomainProfiler.CalculateExtensionsResetMs()
	return extensionsResetMs, timeout, err
}
