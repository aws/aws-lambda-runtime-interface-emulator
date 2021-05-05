// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"syscall"
	"time"

	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/metering"
	"go.amzn.com/lambda/rapi/model"
	"go.amzn.com/lambda/rapi/rendering"

	log "github.com/sirupsen/logrus"
)

func sigkillProcessGroup(pid int, sigkilledPids map[int]bool) map[int]bool {
	pgid, err := syscall.Getpgid(pid)
	if err == nil {
		syscall.Kill(-pgid, 9) // Negative pid sends signal to all in process group
	} else {
		syscall.Kill(pid, 9)
	}
	sigkilledPids[pid] = true

	return sigkilledPids
}

func awaitSigkilledProcessesToExit(exitPidChan chan int, processesExited, sigkilledPidsToAwait map[int]bool) {
	for pid := range processesExited {
		delete(sigkilledPidsToAwait, pid)
	}

	for len(sigkilledPidsToAwait) != 0 {
		pid := <-exitPidChan
		_, found := sigkilledPidsToAwait[pid]
		if !found {
			log.Warnf("Unexpected process %d exited while waiting for sigkilled processes to exit", pid)
		} else {
			delete(sigkilledPidsToAwait, pid)
		}
	}
}

func gracefulShutdown(execCtx *rapidContext, watchdog *core.Watchdog, profiler *metering.ExtensionsResetDurationProfiler, deadlineNs int64, killAgents bool, reason string) {
	watchdog.Mute()
	defer watchdog.Unmute()

	if execCtx.registrationService.CountAgents() == 0 {
		// We do not spend any compute time on runtime graceful shutdown if there are no agents
		if runtime := execCtx.registrationService.GetRuntime(); runtime != nil && runtime.Pid != 0 {
			sigkilledPids := sigkillProcessGroup(runtime.Pid, map[int]bool{})
			if execCtx.standaloneMode {
				processesExited := map[int]bool{}
				awaitSigkilledProcessesToExit(execCtx.exitPidChan, processesExited, sigkilledPids)
			}
		}
		return
	}

	mono := metering.Monotime()

	availableNs := deadlineNs - mono

	if availableNs < 0 {
		log.Warnf("Deadline is in the past: %v, %v, %v", mono, deadlineNs, availableNs)
		availableNs = 0
	}

	profiler.AvailableNs = availableNs

	start := time.Now()
	profiler.Start()

	runtimeDeadline := start.Add(time.Duration(float64(availableNs) * runtimeDeadlineShare))
	agentsDeadline := start.Add(time.Duration(availableNs))

	sigkilledPids := make(map[int]bool)   // Track process ids that were sent sigkill
	processesExited := make(map[int]bool) // Don't send sigkill to processes that exit out of order

	processesExited, sigkilledPids = shutdownRuntime(execCtx, start, runtimeDeadline, processesExited, sigkilledPids)
	processesExited, sigkilledPids = shutdownAgents(execCtx, start, profiler, agentsDeadline, killAgents, reason, processesExited, sigkilledPids)
	if execCtx.standaloneMode {
		awaitSigkilledProcessesToExit(execCtx.exitPidChan, processesExited, sigkilledPids)
	}

	profiler.Stop()
}

func shutdownRuntime(execCtx *rapidContext, start time.Time, deadline time.Time, processesExited, sigkilledPids map[int]bool) (map[int]bool, map[int]bool) {
	// If runtime is started:
	// 1. SIGTERM and wait until timeout
	// 2. SIGKILL on timeout

	log.Debug("shutdown runtime")
	runtime := execCtx.registrationService.GetRuntime()
	if runtime == nil || runtime.Pid == 0 {
		log.Warn("Runtime not started")
		return processesExited, sigkilledPids
	}

	syscall.Kill(runtime.Pid, syscall.SIGTERM)

	runtimeTimeout := deadline.Sub(start)
	runtimeTimer := time.NewTimer(runtimeTimeout)

	for {
		select {
		case pid := <-execCtx.exitPidChan:
			processesExited[pid] = true
			if pid == runtime.Pid {
				log.Info("runtime exited")
				return processesExited, sigkilledPids
			}

			log.Warnf("Process %d exited unexpectedly", pid)
		case <-runtimeTimer.C:
			log.Warnf("Timeout: no SIGCHLD from Runtime after %d ms; dispatching SIGKILL to runtime process group", int64(runtimeTimeout/time.Millisecond))
			sigkilledPids = sigkillProcessGroup(runtime.Pid, sigkilledPids)
			return processesExited, sigkilledPids
		}
	}
}

func shutdownAgents(execCtx *rapidContext, start time.Time, profiler *metering.ExtensionsResetDurationProfiler, deadline time.Time, killAgents bool, reason string, processesExited, sigkilledPids map[int]bool) (map[int]bool, map[int]bool) {
	// For each external agent, if agent is launched:
	// 1. Send Shutdown event if subscribed for it, else send SIGKILL to process group
	// 2. Wait for all Shutdown-subscribed agents to exit with timeout
	// 3. Send SIGKILL to process group for Shutdown-subscribed agents on timeout

	log.Debug("shutdown agents")
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

	pidsToShutdown := make(map[int]*core.ExternalAgent)
	for _, a := range execCtx.registrationService.GetExternalAgents() {
		if a.Pid == 0 {
			log.Warnf("Agent %s failed not launched; skipping shutdown", a)
			continue
		}
		if a.IsSubscribed(core.ShutdownEvent) {
			pidsToShutdown[a.Pid] = a
			a.Release()
		} else {
			if !processesExited[a.Pid] {
				sigkilledPids = sigkillProcessGroup(a.Pid, sigkilledPids)
			}
		}
	}
	profiler.NumAgentsRegisteredForShutdown = len(pidsToShutdown)

	var timerChan <-chan time.Time // default timerChan
	if killAgents {
		timerChan = time.NewTimer(deadline.Sub(start)).C // timerChan with deadline
	}

	timeoutExceeded := false
	for !timeoutExceeded && len(pidsToShutdown) != 0 {
		select {
		case pid := <-execCtx.exitPidChan:
			processesExited[pid] = true
			a, found := pidsToShutdown[pid]
			if !found {
				log.Warnf("Process %d exited unexpectedly", pid)
			} else {
				if err := a.Exited(); err != nil {
					log.Warnf("%s failed to transition to EXITED: %s (current state: %s)", a.String(), err, a.GetState().Name())
				}
				delete(pidsToShutdown, pid)
			}
		case <-timerChan:
			timeoutExceeded = true
		}
	}

	if len(pidsToShutdown) != 0 {
		for pid, agent := range pidsToShutdown {
			if err := agent.ShutdownFailed(); err != nil {
				log.Warnf("%s failed to transition to ShutdownFailed: %s (current state: %s)", agent, err, agent.GetState().Name())
			}
			log.Warnf("Killing agent %s which failed to shutdown", agent)
			if !processesExited[pid] {
				sigkilledPids = sigkillProcessGroup(pid, sigkilledPids)
			}
		}
	}

	return processesExited, sigkilledPids
}
