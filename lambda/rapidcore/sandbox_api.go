// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"go.amzn.com/lambda/interop"
)

// SandboxContext and other structs form the implementation of the SandboxAPI
// interface defined in interop/sandbox_model.go, using the implementation of
// Init, Invoke and Reset handlers in rapid/sandbox.go
type SandboxContext struct {
	rapidCtx          interop.RapidContext
	handler           string
	runtimeAPIAddress string

	InvokeReceivedTime    int64
	InvokeResponseMetrics *interop.InvokeResponseMetrics
}

type initContext struct {
	initSuccessChan chan interop.InitSuccess
	initFailureChan chan interop.InitFailure
	rapidCtx        interop.RapidContext
	sbInfoFromInit  interop.SandboxInfoFromInit // contains data that needs to be persisted from init for suppressed inits during invoke
}

type invokeContext struct {
	rapidCtx          interop.RapidContext
	invokeRequestChan chan *interop.Invoke
	invokeSuccessChan chan interop.InvokeSuccess
	invokeFailureChan chan interop.InvokeFailure
}

// Validate interface compliance
var _ interop.SandboxContext = (*SandboxContext)(nil)
var _ interop.InitContext = (*initContext)(nil)
var _ interop.InvokeContext = (*invokeContext)(nil)

func (s SandboxContext) Init(init *interop.Init, timeoutMs int64) (interop.InitStarted, interop.InitContext) {
	initStartedResponseChan := make(chan interop.InitStarted)
	initSuccessResponseChan := make(chan interop.InitSuccess)
	initFailureResponseChan := make(chan interop.InitFailure)

	if len(s.handler) > 0 {
		init.EnvironmentVariables.SetHandler(s.handler)
	}

	init.EnvironmentVariables.StoreRuntimeAPIEnvironmentVariable(s.runtimeAPIAddress)

	go s.rapidCtx.HandleInit(init, initStartedResponseChan, initSuccessResponseChan, initFailureResponseChan)
	initStarted := <-initStartedResponseChan

	sbMetadata := interop.SandboxInfoFromInit{
		EnvironmentVariables: init.EnvironmentVariables,
		SandboxType:          init.SandboxType,
		RuntimeBootstrap:     init.Bootstrap,
	}
	return initStarted, newInitContext(s.rapidCtx, sbMetadata, initSuccessResponseChan, initFailureResponseChan)
}

func (s SandboxContext) Reset(reset *interop.Reset) (interop.ResetSuccess, *interop.ResetFailure) {
	defer s.rapidCtx.Clear()
	return s.rapidCtx.HandleReset(reset, s.InvokeReceivedTime, s.InvokeResponseMetrics)
}

func (s SandboxContext) Shutdown(shutdown *interop.Shutdown) interop.ShutdownSuccess {
	return s.rapidCtx.HandleShutdown(shutdown)
}

func (s SandboxContext) Restore(restore *interop.Restore) error {
	return s.rapidCtx.HandleRestore(restore)
}

func (s *SandboxContext) SetInvokeReceivedTime(invokeReceivedTime int64) {
	s.InvokeReceivedTime = invokeReceivedTime
}

func (s *SandboxContext) SetInvokeResponseMetrics(metrics *interop.InvokeResponseMetrics) {
	s.InvokeResponseMetrics = metrics
}

func newInitContext(r interop.RapidContext, sbMetadata interop.SandboxInfoFromInit,
	initSuccessChan chan interop.InitSuccess, initFailureChan chan interop.InitFailure) initContext {
	return initContext{
		initSuccessChan: initSuccessChan,
		initFailureChan: initFailureChan,
		rapidCtx:        r,
		sbInfoFromInit:  sbMetadata,
	}
}

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

func (i initContext) Reserve() interop.InvokeContext {

	invokeRequestChan := make(chan *interop.Invoke)
	invokeSuccessChan := make(chan interop.InvokeSuccess)
	invokeFailureChan := make(chan interop.InvokeFailure)

	go func() {
		invoke := <-invokeRequestChan
		// For suppressed inits, invoke needs the runtime and agent env vars
		invokeSuccess, invokeFailure := i.rapidCtx.HandleInvoke(invoke, i.sbInfoFromInit)
		if invokeFailure != nil {
			invokeFailureChan <- *invokeFailure
		} else {
			invokeSuccessChan <- invokeSuccess
		}
	}()

	return invokeContext{
		rapidCtx:          i.rapidCtx,
		invokeRequestChan: invokeRequestChan,
		invokeSuccessChan: invokeSuccessChan,
		invokeFailureChan: invokeFailureChan,
	}
}

func (invCtx invokeContext) SendRequest(i *interop.Invoke) {
	invCtx.invokeRequestChan <- i
}

func (invCtx invokeContext) Wait() (interop.InvokeSuccess, *interop.InvokeFailure) {
	select {
	case invokeSuccess := <-invCtx.invokeSuccessChan:
		return invokeSuccess, nil
	case invokeFailure := <-invCtx.invokeFailureChan:
		return interop.InvokeSuccess{}, &invokeFailure
	}
}
