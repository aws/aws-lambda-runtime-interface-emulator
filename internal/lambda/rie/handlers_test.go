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
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapidcore"
	"github.com/stretchr/testify/require"
)

type delayedInitSandbox struct {
	initDelay       time.Duration
	invokeDelay     time.Duration
	invokeCalled    bool
	initCompletedAt time.Time
	invokeErr       error
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
	time.Sleep(s.initDelay)
	s.initCompletedAt = time.Now()
	time.Sleep(s.invokeDelay)
	return s.invokeErr
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
	sandbox := &delayedInitSandbox{initDelay: 50 * time.Millisecond}

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

func TestInvokeHandlerReportsInitDurationWhenInitTimesOut(t *testing.T) {
	initMutex.Lock()
	initDone = false
	initMutex.Unlock()
	t.Cleanup(func() {
		initMutex.Lock()
		initDone = false
		initMutex.Unlock()
	})
	t.Setenv("AWS_LAMBDA_FUNCTION_TIMEOUT", "1")

	request := httptest.NewRequest(http.MethodPost, "/2015-03-31/functions/function/invocations", nil)
	response := httptest.NewRecorder()
	sandbox := &delayedInitSandbox{
		initDelay: 50 * time.Millisecond,
		invokeErr: rapidcore.ErrInvokeTimeout,
	}

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

	require.Equal(t, "Task timed out after 1.00 seconds", response.Body.String())
	matches := regexp.MustCompile(`Init Duration: ([0-9.]+) ms`).FindStringSubmatch(string(output))
	require.Len(t, matches, 2)
	initDurationMilliseconds, err := strconv.ParseFloat(matches[1], 64)
	require.NoError(t, err)
	require.Greater(t, initDurationMilliseconds, float64(0))
	require.LessOrEqual(t, initDurationMilliseconds, float64(1000))

	matches = regexp.MustCompile(`\tDuration: ([0-9.]+) ms`).FindStringSubmatch(string(output))
	require.Len(t, matches, 2)
	durationMilliseconds, err := strconv.ParseFloat(matches[1], 64)
	require.NoError(t, err)
	require.Less(t, durationMilliseconds, initDurationMilliseconds)
	require.LessOrEqual(t, initDurationMilliseconds+durationMilliseconds, float64(1020))
}

func TestInvokeHandlerSeparatesInitFromTimedOutInvocation(t *testing.T) {
	initMutex.Lock()
	initDone = false
	initMutex.Unlock()
	t.Cleanup(func() {
		initMutex.Lock()
		initDone = false
		initMutex.Unlock()
	})
	t.Setenv("AWS_LAMBDA_FUNCTION_TIMEOUT", "1")

	request := httptest.NewRequest(http.MethodPost, "/2015-03-31/functions/function/invocations", nil)
	response := httptest.NewRecorder()
	sandbox := &delayedInitSandbox{
		initDelay:   200 * time.Millisecond,
		invokeDelay: 200 * time.Millisecond,
		invokeErr:   rapidcore.ErrInvokeTimeout,
	}

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	originalStdout := os.Stdout
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = originalStdout })

	start := time.Now()
	InvokeHandler(response, request, sandbox, nil)
	elapsedMilliseconds := float64(time.Since(start)) / float64(time.Millisecond)
	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	require.Equal(t, "Task timed out after 1.00 seconds", response.Body.String())
	initMatches := regexp.MustCompile(`Init Duration: ([0-9.]+) ms`).FindStringSubmatch(string(output))
	require.Len(t, initMatches, 2)
	initDurationMilliseconds, err := strconv.ParseFloat(initMatches[1], 64)
	require.NoError(t, err)
	require.GreaterOrEqual(t, initDurationMilliseconds, float64(40))

	durationMatches := regexp.MustCompile(`\tDuration: ([0-9.]+) ms`).FindStringSubmatch(string(output))
	require.Len(t, durationMatches, 2)
	durationMilliseconds, err := strconv.ParseFloat(durationMatches[1], 64)
	require.NoError(t, err)
	require.GreaterOrEqual(t, durationMilliseconds, float64(190))
	require.LessOrEqual(t, initDurationMilliseconds+durationMilliseconds, elapsedMilliseconds)
}

type panicOnAwaitSandbox struct {
	delayedInitSandbox
}

func (s *panicOnAwaitSandbox) AwaitInitCompletion() time.Time {
	panic("AwaitInitCompletion should not be called on warm invokes")
}

type hangingInitSandbox struct {
	delayedInitSandbox
}

func (s *hangingInitSandbox) AwaitInitCompletion() time.Time {
	select {}
}

type writeNotifyRecorder struct {
	*httptest.ResponseRecorder
	wrote chan struct{}
	once  sync.Once
}

func (r *writeNotifyRecorder) Write(p []byte) (int, error) {
	n, err := r.ResponseRecorder.Write(p)
	r.once.Do(func() { close(r.wrote) })
	return n, err
}

func TestInvokeHandlerReportsWarmTimeoutWithoutInitDuration(t *testing.T) {
	initMutex.Lock()
	initDone = true
	initMutex.Unlock()
	t.Cleanup(func() {
		initMutex.Lock()
		initDone = false
		initMutex.Unlock()
	})
	t.Setenv("AWS_LAMBDA_FUNCTION_TIMEOUT", "1")

	request := httptest.NewRequest(http.MethodPost, "/2015-03-31/functions/function/invocations", nil)
	response := httptest.NewRecorder()
	sandbox := &panicOnAwaitSandbox{
		delayedInitSandbox: delayedInitSandbox{
			invokeDelay: 50 * time.Millisecond,
			invokeErr:   rapidcore.ErrInvokeTimeout,
		},
	}

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

	require.Equal(t, "Task timed out after 1.00 seconds", response.Body.String())
	require.NotContains(t, string(output), "Init Duration:")
	durationMatches := regexp.MustCompile(`\tDuration: ([0-9.]+) ms`).FindStringSubmatch(string(output))
	require.Len(t, durationMatches, 2)
	durationMilliseconds, err := strconv.ParseFloat(durationMatches[1], 64)
	require.NoError(t, err)
	require.GreaterOrEqual(t, durationMilliseconds, float64(40))
}

func TestInvokeHandlerOmitsInitDurationWhenCompletionHangs(t *testing.T) {
	initMutex.Lock()
	initDone = false
	initMutex.Unlock()
	t.Cleanup(func() {
		initMutex.Lock()
		initDone = false
		initMutex.Unlock()
	})
	t.Setenv("AWS_LAMBDA_FUNCTION_TIMEOUT", "1")

	originalGrace := initReportGracePeriod
	initReportGracePeriod = 300 * time.Millisecond
	t.Cleanup(func() { initReportGracePeriod = originalGrace })

	request := httptest.NewRequest(http.MethodPost, "/2015-03-31/functions/function/invocations", nil)
	response := &writeNotifyRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		wrote:            make(chan struct{}),
	}
	sandbox := &hangingInitSandbox{
		delayedInitSandbox: delayedInitSandbox{
			initDelay: 20 * time.Millisecond,
			invokeErr: rapidcore.ErrInvokeTimeout,
		},
	}

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	originalStdout := os.Stdout
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = originalStdout })

	done := make(chan struct{})
	go func() {
		InvokeHandler(response, request, sandbox, nil)
		close(done)
	}()

	select {
	case <-response.wrote:
	case <-time.After(150 * time.Millisecond):
		require.Fail(t, "timeout body was not written before init-completion wait")
	}
	require.Equal(t, "Task timed out after 1.00 seconds", response.Body.String())

	select {
	case <-done:
		require.Fail(t, "handler returned before the init-completion grace period")
	case <-time.After(100 * time.Millisecond):
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		require.Fail(t, "handler hung waiting for init completion")
	}

	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	require.NotContains(t, string(output), "Init Duration:")
}
