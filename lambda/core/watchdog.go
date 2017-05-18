// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/fatalerror"
	"sync"
)

type WaitableProcess interface {
	// Wait blocks until process exits and returns error in case of non-zero exit code
	Wait() error
	// Pid returnes process ID
	Pid() int
	// Name returnes process executable name (for logging)
	Name() string
}

// Watchdog watches started goroutines.
type Watchdog struct {
	cancelOnce  sync.Once
	initFlow    InitFlowSynchronization
	invokeFlow  InvokeFlowSynchronization
	exitPidChan chan<- int
	appCtx      appctx.ApplicationContext
	mutedMutex  sync.Mutex
	muted       bool
}

func (w *Watchdog) Mute() {
	w.mutedMutex.Lock()
	defer w.mutedMutex.Unlock()
	w.muted = true
}

func (w *Watchdog) Unmute() {
	w.mutedMutex.Lock()
	defer w.mutedMutex.Unlock()
	w.muted = false
}

func (w *Watchdog) Muted() bool {
	w.mutedMutex.Lock()
	defer w.mutedMutex.Unlock()
	return w.muted
}

// GoWait waits for process to complete in separate goroutine and handles the process termination
// Returns PID of the process
func (w *Watchdog) GoWait(p WaitableProcess, errorType fatalerror.ErrorType) int {
	pid := p.Pid()
	name := p.Name()
	appCtx := w.appCtx
	go func() {
		err := p.Wait()

		if !w.Muted() {
			appctx.StoreFirstFatalError(appCtx, errorType)

			if err == nil {
				err = fmt.Errorf("exit code 0")
			}
			log.Warnf("Process %d(%s) exited: %s", pid, name, err)
		}

		w.CancelFlows(err)
		w.exitPidChan <- pid
	}()

	return pid
}

// CancelFlows cancels init and invoke flows with error.
func (w *Watchdog) CancelFlows(err error) {
	// The following block protects us from overwriting the error
	// which was first used to cancel flows.
	w.cancelOnce.Do(func() {
		log.Debugf("Canceling flows: %s", err)
		w.initFlow.CancelWithError(err)
		w.invokeFlow.CancelWithError(err)
	})
}

// Clear watchdog state
func (w *Watchdog) Clear() {
	w.cancelOnce = sync.Once{}
}

// NewWatchdog returns new instance of a Watchdog.
func NewWatchdog(initFlow InitFlowSynchronization, invokeFlow InvokeFlowSynchronization, exitPidChan chan<- int, appCtx appctx.ApplicationContext) *Watchdog {
	return &Watchdog{
		initFlow:    initFlow,
		invokeFlow:  invokeFlow,
		exitPidChan: exitPidChan,
		appCtx:      appCtx,
		mutedMutex:  sync.Mutex{},
	}
}
