// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/testdata"

	"github.com/stretchr/testify/assert"
)

const nextAvailablePort = 0 // net.Listener convention for next available port
const serverAddress = "127.0.0.1"

func createTestServer(handlerFunc http.HandlerFunc) (*Server, error) {
	host, server := serverAddress, &http.Server{Handler: handlerFunc}
	s := &Server{
		host:     host,
		port:     nextAvailablePort,
		server:   server,
		listener: nil,
		exit:     make(chan error, 1),
	}
	err := s.Listen()
	return s, err
}

func TestServerReturnsSuccessfulResponse(t *testing.T) {
	expectedResponse := "foo"
	testHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, expectedResponse)
		})
	s, err := createTestServer(testHandler)
	if err != nil {
		assert.FailNowf(t, "Server failed to listen", err.Error())
	}
	go func() {
		s.Serve(context.Background())
	}()

	resp, err := http.Get(fmt.Sprintf("http://%s/", s.listener.Addr()))
	if err != nil {
		assert.FailNowf(t, "Failed to get response", err.Error())
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		assert.FailNowf(t, "Failed to read response body", err.Error())
	}
	resp.Body.Close()
	s.Close()

	assert.Equal(t, expectedResponse, string(body))
}

func TestServerExitsOnContextCancelation(t *testing.T) {
	testHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "foo")
		})
	ctx, cancel := context.WithCancel(context.Background())
	errChan := make(chan error, 1)
	s, err := createTestServer(testHandler)
	if err != nil {
		assert.FailNowf(t, "Server failed to listen", err.Error())
	}
	go func() {
		errChan <- s.Serve(ctx)
	}()

	pingRequestFunc := func() (bool, error) {
		if _, err := http.Get(s.URL("/ping")); err != nil {
			return false, err
		}
		return true, nil
	}
	serverStarted := testdata.Eventually(t, pingRequestFunc, 10*time.Millisecond, 10)
	assert.True(t, serverStarted)

	cancel()
	error := testdata.WaitForErrorWithTimeout(errChan, 2*time.Second)
	s.Close()
	assert.Error(t, error)
	assert.Contains(t, error.Error(), ctx.Err().Error())
}

func TestServerExitsOnExitSignalFromHandler(t *testing.T) {
	testHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "foo")
		})
	errChan := make(chan error, 1)
	s, err := createTestServer(testHandler)
	if err != nil {
		assert.FailNowf(t, "Server failed to listen", err.Error())
	}
	go func() {
		errChan <- s.Serve(context.Background())
	}()

	exitError := errors.New("foo bar error")
	s.exit <- exitError

	error := testdata.WaitForErrorWithTimeout(errChan, time.Second)
	s.Close()

	assert.Error(t, error)
	assert.Contains(t, error.Error(), exitError.Error())
}
