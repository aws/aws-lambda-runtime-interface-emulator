// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/fatalerror"
	"testing"
)

var errTest = errors.New("ErrTest")

type MockProcess struct {
}

func (s *MockProcess) Wait() error  { return errTest }
func (s *MockProcess) Pid() int     { return 0 }
func (s *MockProcess) Name() string { return "" }

func TestWatchdogCallback(t *testing.T) {
	initFlow := &mockInitFlowSynchronization{}
	invokeFlow := &mockInvokeFlowSynchronization{}
	initFlow.On("CancelWithError", mock.Anything)
	invokeFlow.On("CancelWithError", mock.Anything)

	pidChan := make(chan int)
	appCtx := appctx.NewApplicationContext()
	w := NewWatchdog(initFlow, invokeFlow, pidChan, appCtx)

	w.GoWait(&MockProcess{}, fatalerror.AgentExitError)
	w.GoWait(&MockProcess{}, fatalerror.AgentExitError)

	<-pidChan
	initFlow.AssertCalled(t, "CancelWithError", errTest)
	initFlow.AssertNumberOfCalls(t, "CancelWithError", 1)
	invokeFlow.AssertCalled(t, "CancelWithError", errTest)
	invokeFlow.AssertNumberOfCalls(t, "CancelWithError", 1)

	<-pidChan
	initFlow.AssertNumberOfCalls(t, "CancelWithError", 1)
	invokeFlow.AssertNumberOfCalls(t, "CancelWithError", 1)

	err, found := appctx.LoadFirstFatalError(appCtx)
	require.True(t, found)
	require.Equal(t, err, fatalerror.AgentExitError)
}
