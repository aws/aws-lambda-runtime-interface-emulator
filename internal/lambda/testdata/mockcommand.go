// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

/*
Package testdata holds ancillary data needed by

	the tests. The go tool ignores this directory.
	https://golang.org/pkg/cmd/go/internal/test/
*/
package testdata

import (
	"context"
)

// MockCommand represents a mock of os/exec.Cmd for testing
type MockCommand struct {
	done chan error
	ctx  context.Context
}

// NewMockCommand returns a MockCommand that satisfies
// the command interface
func NewMockCommand(ctx context.Context) MockCommand {
	done := make(chan error)
	return MockCommand{done, ctx}
}

// Start represents a successful cmd.Start() without
// errors
func (c MockCommand) Start() error {
	return nil
}

// Wait represents a cancelable call to cmd.Wait()
func (c MockCommand) Wait() error {
	select {
	case <-c.done:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

// ForceExit tells goroutine blocking on Wait() to exit
func (c MockCommand) ForceExit() {
	c.done <- nil
}
