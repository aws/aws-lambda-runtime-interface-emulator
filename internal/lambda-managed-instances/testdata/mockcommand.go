// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package testdata

import (
	"context"
)

type MockCommand struct {
	done chan error
	ctx  context.Context
}

func NewMockCommand(ctx context.Context) MockCommand {
	done := make(chan error)
	return MockCommand{done, ctx}
}

func (c MockCommand) Start() error {
	return nil
}

func (c MockCommand) Wait() error {
	select {
	case <-c.done:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c MockCommand) ForceExit() {
	c.done <- nil
}
