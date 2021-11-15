// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package runtimecmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"syscall"
)

// CustomRuntimeCmd wraps exec.Cmd
type CustomRuntimeCmd struct {
	*exec.Cmd
}

// NewCustomRuntimeCmd returns a new CustomRuntimeCmd
func NewCustomRuntimeCmd(ctx context.Context, bootstrapCmd []string, dir string, env []string, stdoutWriter io.Writer, stderrWriter io.Writer, extraFiles []*os.File) *CustomRuntimeCmd {
	cmd := exec.CommandContext(ctx, bootstrapCmd[0], bootstrapCmd[1:]...)
	cmd.Dir = dir

	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	cmd.Env = env

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if len(extraFiles) > 0 {
		cmd.ExtraFiles = extraFiles
	}

	return &CustomRuntimeCmd{cmd}
}

// Name returnes runtime executable name
func (cmd *CustomRuntimeCmd) Name() string {
	return path.Base(cmd.Path)
}

// Pid returns the pid of a started runtime process
func (cmd *CustomRuntimeCmd) Pid() int {
	return cmd.Process.Pid
}

// Wait waits for the started customer runtime process to exit
func (cmd *CustomRuntimeCmd) Wait() error {
	if err := cmd.Cmd.Wait(); err != nil {
		return fmt.Errorf("Runtime exited with error: %v", err)
	}

	return fmt.Errorf("Runtime exited without providing a reason")
}
