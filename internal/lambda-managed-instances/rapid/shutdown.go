// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
	supvmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/supervisor/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils/invariant"
)

const (
	supervisorBlockingTimeout = 1 * time.Second

	maxProcessExitWait = 2 * time.Second

	defaultShutdownTimeout = 2 * time.Second

	defaultRuntimeShutdownTimeout = 600 * time.Millisecond
)

type shutdownContext struct {
	mu                 sync.Mutex
	shuttingDown       bool
	agentsAwaitingExit map[string]*core.ExternalAgent

	processExited          map[string]chan struct{}
	shutdownTimeout        time.Duration
	runtimeShutdownTimeout time.Duration
}

func newShutdownContext() *shutdownContext {
	return &shutdownContext{
		shuttingDown:           false,
		agentsAwaitingExit:     make(map[string]*core.ExternalAgent),
		processExited:          make(map[string]chan struct{}),
		shutdownTimeout:        defaultShutdownTimeout,
		runtimeShutdownTimeout: defaultRuntimeShutdownTimeout,
	}
}

func (s *shutdownContext) isShuttingDown() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shuttingDown
}

func (s *shutdownContext) setShuttingDown(value bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shuttingDown = value
}

func (s *shutdownContext) handleProcessExit(termination supvmodel.ProcessTermination) {
	name := termination.Name
	s.mu.Lock()
	agent, found := s.agentsAwaitingExit[name]
	s.mu.Unlock()

	if found {
		slog.Debug("Handling termination", "name", name)
		if termination.ExitedWithZeroExitCode() {

			stateErr := agent.Exited()
			if stateErr != nil {
				slog.Warn("failed to transition to EXITED", "agent", agent.String(), "error", stateErr, "currentState", agent.GetState().Name())
			}
		} else {

			stateErr := agent.ShutdownFailed()
			if stateErr != nil {
				slog.Warn("failed to transition to ShutdownFailed", "agent", agent, "error", stateErr, "currentState", agent.GetState().Name())
			}
		}
	}

	if exitedChannel, found := s.getExitedChannel(name); found {

		close(exitedChannel)
	} else {
		slog.Warn("Unknown process: possibly failed to launch, or it is from previous generation", "name", name)
	}
}

func (s *shutdownContext) getExitedChannel(name string) (chan struct{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	exitedChannel, found := s.processExited[name]
	return exitedChannel, found
}

func (s *shutdownContext) createExitedChannel(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, found := s.processExited[name]
	invariant.Checkf(!found, "Tried to create an exited channel for '%s' but one already exists.", name)

	s.processExited[name] = make(chan struct{})
}

func (s *shutdownContext) waitUntilAllProcessesExit(metrics interop.ShutdownMetrics) error {
	duration := metrics.CreateDurationMetric(interop.ShutdownWaitAllProcessesDuration)
	defer duration.Done()

	s.mu.Lock()
	channels := make([]chan struct{}, 0, len(s.processExited))
	for _, v := range s.processExited {
		channels = append(channels, v)
	}
	s.mu.Unlock()

	exitTimeout := time.After(maxProcessExitWait)
	for _, v := range channels {
		select {
		case <-v:
		case <-exitTimeout:
			return errors.New("timed out waiting for runtime processes to exit")
		}
	}

	return nil
}

func (s *shutdownContext) shutdownRuntime(ctx context.Context, supervisor processSupervisor, metrics interop.ShutdownMetrics) error {
	duration := metrics.CreateDurationMetric(interop.ShutdownRuntimeDuration)
	defer duration.Done()

	slog.Debug("Shutting down the runtime.")
	name := runtimeProcessName

	exitedChannel, found := s.getExitedChannel(name)
	if !found {
		slog.Warn("runtime was not started", "name", name)
		return errors.New("runtime was not started")
	}

	err := supervisor.Terminate(ctx, &supvmodel.TerminateRequest{
		Name: name,
	})
	if err != nil {

		slog.Warn("Failed sending Termination signal to runtime", "err", err)
	}
	err = waitProcessExitedOrKill(ctx, exitedChannel, name, supervisor)
	slog.Debug("Shutdown the runtime.")
	return err
}

func (s *shutdownContext) shutdownAgents(
	ctx context.Context,
	supervisor processSupervisor,
	renderingService *rendering.EventRenderingService,
	agents []*core.ExternalAgent,
	reason model.ShutdownReason,
	extShutdownDeadline time.Time,
	metrics interop.ShutdownMetrics,
) error {
	duration := metrics.CreateDurationMetric(interop.ShutdownExtensionsDuration)
	defer duration.Done()

	slog.Debug("Shutting down the agents.")

	renderingService.SetRenderer(
		&rendering.ShutdownRenderer{
			AgentEvent: model.AgentShutdownEvent{
				AgentEvent: &model.AgentEvent{
					EventType:  "SHUTDOWN",
					DeadlineMs: extShutdownDeadline.UnixMilli(),
				},
				ShutdownReason: reason,
			},
		})

	resultAwaiters := make([]<-chan error, 0, len(agents))

	for _, a := range agents {
		name := extensionProcessName(a.Name())
		exitedChannel, found := s.getExitedChannel(name)

		if !found {
			slog.Warn("Agent failed to launch, therefore skipping shutting it down", "agent", a)
			continue
		}

		awaiter := make(chan error, 1)
		resultAwaiters = append(resultAwaiters, awaiter)

		if a.IsSubscribed(core.ShutdownEvent) {
			slog.Debug("Agent is registered for the shutdown event", "agent", a)
			s.mu.Lock()
			s.agentsAwaitingExit[name] = a
			s.mu.Unlock()

			go func(name string, agent *core.ExternalAgent, ch chan<- error) {

				agent.Release()
				ch <- waitProcessExitedOrKill(ctx, exitedChannel, name, supervisor)
			}(name, a, awaiter)
		} else {
			slog.Debug("Agent is not registered for the shutdown event, so just killing it", "agent", a)

			go func(name string, ch chan<- error) {

				defer close(ch)
				if err := killProcess(ctx, name, supervisor, nil); err != nil {
					slog.Warn("Failed to kill process", "name", name, "err", err)
				}
			}(name, awaiter)
		}
	}

	errs := make([]error, 0, len(resultAwaiters))
	for _, ch := range resultAwaiters {
		errs = append(errs, <-ch)
	}
	slog.Debug("Shutdown the agents.")
	return errors.Join(errs...)
}

func killProcess(ctx context.Context, name string, supervisor supvmodel.ProcessSupervisor, metrics interop.ShutdownMetrics) error {
	if metrics != nil {
		duration := metrics.CreateDurationMetric(fmt.Sprintf(interop.ShutdownKillProcessDurationMetricTemplate, name))
		defer duration.Done()

	}

	deadline, hasDeadline := ctx.Deadline()
	invariant.Checkf(hasDeadline, "Context to kill process %q has a deadline", name)

	err := supervisor.Kill(ctx, &supvmodel.KillRequest{
		Name:     name,
		Deadline: deadline,
	})
	if err != nil {
		slog.Warn("Failed sending Kill signal to process", "name", name, "err", err)
	}
	return err
}

func waitProcessExitedOrKill(
	ctx context.Context,
	exitedChannel <-chan struct{},
	name string,
	supervisor supvmodel.ProcessSupervisor,
) error {
	select {
	case <-exitedChannel:
		return nil
	case <-ctx.Done():
		select {
		case <-exitedChannel:

			return nil
		default:
			killCtx, killCtxCancel := context.WithDeadline(context.Background(), time.Now().Add(supervisorBlockingTimeout))
			defer killCtxCancel()
			return killProcess(killCtx, name, supervisor, nil)
		}
	}
}

func (s *shutdownContext) shutdown(
	supervisor processSupervisor,
	renderingService *rendering.EventRenderingService,
	externalAgents []*core.ExternalAgent,
	totalAgentsCount int,
	reason model.ShutdownReason,
	metrics interop.ShutdownMetrics,
	eventsAPI interop.EventsAPI,
) error {
	errs := make([]error, 0, 4)
	s.setShuttingDown(true)

	if totalAgentsCount == 0 {
		name := runtimeProcessName

		_, found := s.getExitedChannel(name)

		if found {
			slog.Debug("SIGKILLing the runtime as no agents are registered.")
			runtimeSigkillCtx, cancel := context.WithTimeout(context.Background(), supervisorBlockingTimeout)
			defer cancel()
			errs = append(errs, killProcess(runtimeSigkillCtx, name, supervisor, metrics))
		} else {
			slog.Debug("Could not find runtime process in processes map. Already exited/never started", "name", name)
		}
	} else {
		shutdownConfig := s.configureShutdownDeadlines()
		extShutdownCtx, extCancel := context.WithDeadline(context.Background(), shutdownConfig.extSigkillDeadline)
		defer extCancel()
		rtShutdownCtx, rtCancel := context.WithDeadline(context.Background(), shutdownConfig.rtShutdownDeadline)
		defer rtCancel()

		errs = append(errs, s.shutdownRuntime(rtShutdownCtx, supervisor, metrics))
		eventsAPI.Flush()
		errs = append(errs, s.shutdownAgents(extShutdownCtx, supervisor, renderingService, externalAgents, reason, shutdownConfig.extShutdownDeadline, metrics))
	}

	if err := errors.Join(errs...); err == nil {
		slog.Info("Waiting for runtime domain processes termination")
		err = s.waitUntilAllProcessesExit(metrics)

		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

type configShutdownDeadlines struct {
	shutdownTimeout     time.Duration
	rtShutdownDeadline  time.Time
	extShutdownDeadline time.Time
	extSigkillDeadline  time.Time
}

func (s *shutdownContext) configureShutdownDeadlines() configShutdownDeadlines {
	now := time.Now()
	runtimeShutdownDeadline := now.Add(s.runtimeShutdownTimeout)
	return configShutdownDeadlines{
		shutdownTimeout:     s.shutdownTimeout,
		rtShutdownDeadline:  runtimeShutdownDeadline,
		extShutdownDeadline: now.Add(s.shutdownTimeout),
		extSigkillDeadline:  now.Add(s.shutdownTimeout),
	}
}

func (s *shutdownContext) processTermination(
	event supvmodel.ProcessTermination,
	execCtx *rapidContext,
) {
	var fatalError rapidmodel.ErrorType
	var err error

	defer s.handleProcessExit(event)

	if !s.isShuttingDown() {

		s.setShuttingDown(true)

		if event.Name == runtimeProcessName {
			switch {
			case event.OomKilled():
				err = fmt.Errorf("runtime exited with error: %s", event.String())
				fatalError = rapidmodel.ErrorRuntimeOutOfMemory
			case event.ExitedWithZeroExitCode():
				err = fmt.Errorf("runtime exited without providing a reason")
				fatalError = rapidmodel.ErrorRuntimeExit
			default:
				err = fmt.Errorf("runtime exited with error: %s", event.String())
				fatalError = rapidmodel.ErrorRuntimeExit
			}
		} else {
			fatalError = rapidmodel.ErrorAgentCrash
			if event.ExitedWithZeroExitCode() {
				err = fmt.Errorf("exit code 0")
			} else {
				err = errors.New(event.String())
			}
		}

		appctx.StoreFirstFatalError(execCtx.appCtx, rapidmodel.WrapErrorIntoCustomerFatalError(nil, fatalError))

		customerError, _ := appctx.LoadFirstFatalError(execCtx.appCtx)

		select {
		case execCtx.processTermChan <- customerError:
		default:

		}
		slog.Warn("Process exited", "name", event.Name, "event", event)
	}

	execCtx.registrationService.CancelFlows(err)
}

func extensionProcessName(extensionName string) string {
	return fmt.Sprintf("extension-%s", extensionName)
}
