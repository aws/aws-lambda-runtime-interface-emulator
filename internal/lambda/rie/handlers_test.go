// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rie

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/interop"
	"github.com/stretchr/testify/require"
)

type delayedInitSandbox struct {
	delay           time.Duration
	invokeCalled    bool
	initCompletedAt time.Time
}

func (s *delayedInitSandbox) Init(*interop.Init, int64) {}

func (s *delayedInitSandbox) AwaitInitCompletion() time.Time {
	if !s.invokeCalled {
		panic("AwaitInitCompletion called before Invoke")
	}
	return s.initCompletedAt
}

func (s *delayedInitSandbox) Invoke(http.ResponseWriter, *interop.Invoke) error {
	s.invokeCalled = true
	time.Sleep(s.delay)
	s.initCompletedAt = time.Now()
	return nil
}

type panicInitSandbox struct {
	delayedInitSandbox
}

func (s *panicInitSandbox) Init(*interop.Init, int64) {
	panic("init failed")
}

func TestStartInitOnceReleasesLockAfterPanic(t *testing.T) {
	initMutex.Lock()
	initDone = false
	initMutex.Unlock()
	t.Cleanup(func() {
		initMutex.Lock()
		initDone = false
		initMutex.Unlock()
	})

	func() {
		defer func() { require.Equal(t, "init failed", recover()) }()
		startInitOnce(&panicInitSandbox{}, "$LATEST", 1, nil)
	}()

	initStarted := make(chan struct{})
	go func() {
		startInitOnce(&delayedInitSandbox{}, "$LATEST", 1, nil)
		close(initStarted)
	}()
	select {
	case <-initStarted:
	case <-time.After(time.Second):
		require.Fail(t, "init mutex remained locked after panic")
	}
}

func TestInvokeHandlerReportsRuntimeInitDuration(t *testing.T) {
	initMutex.Lock()
	initDone = false
	initMutex.Unlock()
	t.Cleanup(func() {
		initMutex.Lock()
		initDone = false
		initMutex.Unlock()
	})

	request := httptest.NewRequest(http.MethodPost, "/2015-03-31/functions/function/invocations", nil)
	response := httptest.NewRecorder()
	sandbox := &delayedInitSandbox{delay: 50 * time.Millisecond}

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	originalStdout := os.Stdout
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = originalStdout })

	InvokeHandler(response, request, sandbox, nil)
	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	matches := regexp.MustCompile(`Init Duration: ([0-9.]+) ms`).FindStringSubmatch(string(output))
	require.Len(t, matches, 2)
	durationMilliseconds, err := strconv.ParseFloat(matches[1], 64)
	require.NoError(t, err)
	require.GreaterOrEqual(t, durationMilliseconds, float64(40))

	matches = regexp.MustCompile(`\tDuration: ([0-9.]+) ms`).FindStringSubmatch(string(output))
	require.Len(t, matches, 2)
	durationMilliseconds, err = strconv.ParseFloat(matches[1], 64)
	require.NoError(t, err)
	require.Less(t, durationMilliseconds, float64(40))
}
