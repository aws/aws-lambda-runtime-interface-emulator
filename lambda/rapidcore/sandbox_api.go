// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"bytes"

	"go.amzn.com/lambda/extensions"
	"go.amzn.com/lambda/interop"
)

// SandboxContext and other structs form the implementation of the SandboxAPI
// interface defined in interop/sandbox_model.go, using the implementation of
// Init, Invoke and Reset handlers in rapid/sandbox.go
type SandboxContext struct {
	rapidCtx          interop.RapidContext
	handler           string
	runtimeAPIAddress string
}

// initContext and its methods model the initialization lifecycle
// of the Sandbox, which persist across invocations
type initContext struct {
	initSuccessChan     chan interop.InitSuccess
	initFailureChan     chan interop.InitFailure
	rapidCtx            interop.RapidContext
	sbInfoFromInit      interop.SandboxInfoFromInit // contains data that needs to be persisted from init for suppressed inits during invoke
	invokeRequestBuffer *bytes.Buffer               // byte buffer used to store the invoke request rendered to runtime (reused until reset)
}

// invokeContext and its methods model the invocation lifecycle
type invokeContext struct {
	rapidCtx            interop.RapidContext
	invokeRequestChan   chan *interop.Invoke
	invokeSuccessChan   chan interop.InvokeSuccess
	invokeFailureChan   chan interop.InvokeFailure
	sbInfoFromInit      interop.SandboxInfoFromInit // contains data that needs to be persisted from init for suppressed inits during invoke
	invokeRequestBuffer *bytes.Buffer               // byte buffer used to store the invoke request rendered to runtime (reused until reset)
}

// Validate interface compliance
var _ interop.SandboxContext = (*SandboxContext)(nil)
var _ interop.InitContext = (*initContext)(nil)
var _ interop.InvokeContext = (*invokeContext)(nil)

// Init starts the runtime domain initialization in a separate goroutine.
// Return value indicates that init request has been accepted and started.
func (s SandboxContext) Init(init *interop.Init, timeoutMs int64) interop.InitContext {
	initSuccessResponseChan := make(chan interop.InitSuccess)
	initFailureResponseChan := make(chan interop.InitFailure)

	if len(s.handler) > 0 {
		init.EnvironmentVariables.SetHandler(s.handler)
	}

	init.EnvironmentVariables.StoreRuntimeAPIEnvironmentVariable(s.runtimeAPIAddress)
	extensions.DisableViaMagicLayer()

	// We start initialization handling in a separate goroutine so that control can be returned back to
	// caller, which can do work (e.g. notifying further upstream that initialization has started), and
	// and call initCtx.Wait() to wait async for completion of initialization phase.
	go s.rapidCtx.HandleInit(init, initSuccessResponseChan, initFailureResponseChan)

	sbMetadata := interop.SandboxInfoFromInit{
		EnvironmentVariables: init.EnvironmentVariables,
		SandboxType:          init.SandboxType,
		RuntimeBootstrap:     init.Bootstrap,
	}
	return newInitContext(s.rapidCtx, sbMetadata, initSuccessResponseChan, initFailureResponseChan)
}

// Reset triggers a reset. In case of timeouts, the reset handler cancels all flows which triggers
// ongoing invoke handlers to return before proceeding with invoke
// TODO: move this method to the initialization context, since reset is conceptually on RT domain
func (s SandboxContext) Reset(reset *interop.Reset) (interop.ResetSuccess, *interop.ResetFailure) {
	defer s.rapidCtx.Clear()
	return s.rapidCtx.HandleReset(reset)
}

// Reset triggers a shutdown. This is similar to a reset, except that this is a terminal state
// and no further invokes are allowed
func (s SandboxContext) Shutdown(shutdown *interop.Shutdown) interop.ShutdownSuccess {
	return s.rapidCtx.HandleShutdown(shutdown)
}

func (s SandboxContext) Restore(restore *interop.Restore) (interop.RestoreResult, error) {
	return s.rapidCtx.HandleRestore(restore)
}

func (s *SandboxContext) SetRuntimeStartedTime(runtimeStartedTime int64) {
	s.rapidCtx.SetRuntimeStartedTime(runtimeStartedTime)
}

func (s *SandboxContext) SetInvokeResponseMetrics(metrics *interop.InvokeResponseMetrics) {
	s.rapidCtx.SetInvokeResponseMetrics(metrics)
}

func newInitContext(r interop.RapidContext, sbMetadata interop.SandboxInfoFromInit,
	initSuccessChan chan interop.InitSuccess, initFailureChan chan interop.InitFailure) initContext {

	// Invocation request buffer is initialized once per initialization
	// to reduce memory usage & GC CPU time across invocations
	var requestBuffer bytes.Buffer

	return initContext{
		initSuccessChan:     initSuccessChan,
		initFailureChan:     initFailureChan,
		rapidCtx:            r,
		sbInfoFromInit:      sbMetadata,
		invokeRequestBuffer: &requestBuffer,
	}
}

// Wait awaits until initialization phase is complete, i.e. one of:
// - until all runtime domain process call /next
// - any one of the runtime domain processes exit (init failure)
// Timeout handling is managed upstream entirely
func (i initContext) Wait() (interop.InitSuccess, *interop.InitFailure) {
	select {
	case initSuccess, isOpen := <-i.initSuccessChan:
		if !isOpen {
			// If init has already suceeded, we return quickly
			return interop.InitSuccess{}, nil
		}
		return initSuccess, nil
	case initFailure, isOpen := <-i.initFailureChan:
		if !isOpen {
			// If init has already failed, we return quickly for init to be suppressed
			return interop.InitSuccess{}, &initFailure
		}
		return interop.InitSuccess{}, &initFailure
	}
}

// Reserve is used to initialize invoke-related state
func (i initContext) Reserve() interop.InvokeContext {
	invokeRequestChan := make(chan *interop.Invoke)
	invokeSuccessChan := make(chan interop.InvokeSuccess)
	invokeFailureChan := make(chan interop.InvokeFailure)

	return invokeContext{
		rapidCtx:            i.rapidCtx,
		invokeRequestChan:   invokeRequestChan,
		invokeSuccessChan:   invokeSuccessChan,
		invokeFailureChan:   invokeFailureChan,
		sbInfoFromInit:      i.sbInfoFromInit,
		invokeRequestBuffer: i.invokeRequestBuffer,
	}
}

// SendRequest starts the invocation request handling in a separate goroutine,
// i.e. sending the request payload via /next response,
// and waiting for the synchronization points
func (invCtx invokeContext) SendRequest(invoke *interop.Invoke, responseSender interop.InvokeResponseSender) {
	// Invoke handling needs to be in a separate goroutine so that control can
	// be returned immediately to calling goroutine, which can do work and
	// asynchronously call invCtx.Wait() to await completion of the invoke phase
	go func() {
		// For suppressed inits, invoke needs the runtime and agent env vars
		invokeSuccess, invokeFailure := invCtx.rapidCtx.HandleInvoke(invoke, invCtx.sbInfoFromInit, invCtx.invokeRequestBuffer, responseSender)
		if invokeFailure != nil {
			invCtx.invokeFailureChan <- *invokeFailure
		} else {
			invCtx.invokeSuccessChan <- invokeSuccess
		}
	}()
}

// Wait awaits invoke completion, i.e. one of the following cases:
// - until all runtime domain process call /next
// - until a process exit (that notifies upstream to trigger a reset due to "failure")
// - until a timeout (triggered by a reset from upstream due to "timeout")
func (invCtx invokeContext) Wait() (interop.InvokeSuccess, *interop.InvokeFailure) {
	select {
	case invokeSuccess := <-invCtx.invokeSuccessChan:
		return invokeSuccess, nil
	case invokeFailure := <-invCtx.invokeFailureChan:
		return interop.InvokeSuccess{}, &invokeFailure
	}
}
