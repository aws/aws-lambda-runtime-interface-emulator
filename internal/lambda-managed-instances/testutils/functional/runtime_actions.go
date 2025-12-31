// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build test

package functional

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
)

type WaitInvokeResponseAction struct {
	wg *sync.WaitGroup
}

func (a WaitInvokeResponseAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	a.wg.Wait()
	return nil, nil
}

func (a WaitInvokeResponseAction) ValidateStatus(t *testing.T, resp *http.Response) {}

func (a WaitInvokeResponseAction) String() string {
	return "WaitInvokeResponseAction"
}

type SleepAction struct {
	Duration time.Duration
}

func (a SleepAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	if a.Duration == 0 {
		a.Duration = 100 * time.Millisecond
	}
	t.Logf("runtime sleeping for %v\n", a.Duration)
	time.Sleep(a.Duration)
	return nil, nil
}

func (a SleepAction) ValidateStatus(t *testing.T, resp *http.Response) {}

func (a SleepAction) String() string {
	return fmt.Sprintf("Sleep(duration=%v)", a.Duration)
}

type NextAction struct {
	Payload        string
	ExpectedStatus int
}

func (a NextAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	var body io.Reader
	if a.Payload != "" {
		body = strings.NewReader(a.Payload)
	}
	resp := client.Next(body)
	return resp, nil
}

func (a NextAction) ValidateStatus(t *testing.T, resp *http.Response) {
	if a.ExpectedStatus != 0 {
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, a.ExpectedStatus, resp.StatusCode, "response", string(body))
	}
}

func (a NextAction) String() string {
	return "Next()"
}

type StdoutAction struct {
	Payload string
	stdout  io.Writer
}

func (a StdoutAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	_, err := a.stdout.Write([]byte(a.Payload))
	require.NoError(t, err)
	return nil, err
}

func (a StdoutAction) ValidateStatus(t *testing.T, resp *http.Response) {}

func (a StdoutAction) String() string {
	return "Stdout()"
}

type StderrAction struct {
	Payload string
	stderr  io.Writer
}

func (a StderrAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	_, err := a.stderr.Write([]byte(a.Payload))
	require.NoError(t, err)
	return nil, err
}

func (a StderrAction) ValidateStatus(t *testing.T, resp *http.Response) {}

func (a StderrAction) String() string {
	return "Stderr()"
}

type InitErrorAction struct {
	Payload        string
	ContentType    string
	ErrorType      string
	ExpectedStatus int
}

func (a InitErrorAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	return client.InitError(a.Payload, a.ContentType, a.ErrorType)
}

func (a InitErrorAction) ValidateStatus(t *testing.T, resp *http.Response) {
	if a.ExpectedStatus != 0 {
		assert.Equal(t, a.ExpectedStatus, resp.StatusCode, "InitErrorAction expected status code %d", a.ExpectedStatus)
	}
}

func (a InitErrorAction) String() string {
	return fmt.Sprintf("InitError(type=%s)", a.ErrorType)
}

type ExitAction struct {
	ExitCode            int32
	exitProcessOnce     func()
	CrashingProcessName string
}

func (a ExitAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	a.exitProcessOnce()

	return nil, nil
}

func (a ExitAction) ValidateStatus(t *testing.T, resp *http.Response) {}

func (a ExitAction) String() string {
	return fmt.Sprintf("Exit(code=%d)", a.ExitCode)
}

type InvocationResponseAction struct {
	Payload            io.Reader
	ContentType        string
	InvokeID           interop.InvokeID
	ResponseModeHeader string
	ExpectedStatus     int
	ExpectedBody       string
	Trailers           map[string]string
}

func (a InvocationResponseAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	return client.Response(a.InvokeID, a.Payload, a.ContentType, a.ResponseModeHeader, a.Trailers)
}

func (a InvocationResponseAction) ValidateStatus(t *testing.T, resp *http.Response) {

	if a.ExpectedStatus != 0 && resp != nil {
		dump, err := httputil.DumpResponse(resp, true)
		require.NoError(t, err)
		assert.Equal(t, a.ExpectedStatus, resp.StatusCode, "Expected status %d for RequestId=%s, got %s", resp.StatusCode, a.InvokeID, string(dump))
	}
	if a.ExpectedBody != "" {
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equalf(t, a.ExpectedBody, string(body), "invokeID=%s", a.InvokeID)
	}
}

func (a InvocationResponseAction) String() string {
	return fmt.Sprintf("InvocationResponse(%s)", a.InvokeID)
}

type InvocationResponseErrorAction struct {
	Payload        string
	ContentType    string
	InvokeID       interop.InvokeID
	ErrorType      string
	ErrorCause     string
	ExpectedStatus int
	ExpectedBody   string
}

func (a InvocationResponseErrorAction) Execute(t *testing.T, client *Client) (*http.Response, error) {
	return client.ResponseError(a.InvokeID, a.Payload, a.ContentType, a.ErrorType, a.ErrorCause)
}

func (a InvocationResponseErrorAction) ValidateStatus(t *testing.T, resp *http.Response) {
	if a.ExpectedStatus != 0 {
		assert.Equal(t, a.ExpectedStatus, resp.StatusCode, "InvocationResponseErrorAction expected status %d", a.ExpectedStatus)
	}
	if a.ExpectedBody != "" {
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equalf(t, a.ExpectedBody, string(body), "invokeID=%s", a.InvokeID)
	}
}

func (a InvocationResponseErrorAction) String() string {
	return "InvocationResponse()"
}

type InvocationStreamingResponseAction struct {
	Chunks             []string
	ContentType        string
	InvokeID           interop.InvokeID
	ResponseModeHeader string
	ChunkDelay         time.Duration
	Trailers           map[string]string
}

func (a InvocationStreamingResponseAction) Execute(t *testing.T, client *Client) (*http.Response, error) {

	chunkedReader := NewChunkedReader(a.Chunks, a.ChunkDelay)

	return client.Response(a.InvokeID, chunkedReader, a.ContentType, a.ResponseModeHeader, a.Trailers)
}

func (a InvocationStreamingResponseAction) ValidateStatus(t *testing.T, resp *http.Response) {}

func (a InvocationStreamingResponseAction) String() string {
	return fmt.Sprintf("InvocationStreamingResponse(%s, %d chunks)", a.InvokeID, len(a.Chunks))
}
