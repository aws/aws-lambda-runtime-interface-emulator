// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapidcore/env"
)

// the type implement a simpler version of the Bootstrap
// this is useful in the Standalone Core implementation.
type simpleBootstrap struct {
	cmd        []string
	workingDir string
}

func NewSimpleBootstrap(cmd []string, currentWorkingDir string) interop.Bootstrap {
	if currentWorkingDir == "" {
		// use the root directory as the default working directory
		currentWorkingDir = "/"
	}

	// a single candidate command makes it automatically valid
	return &simpleBootstrap{
		cmd:        cmd,
		workingDir: currentWorkingDir,
	}
}

func (b *simpleBootstrap) Cmd() ([]string, error) {
	return b.cmd, nil
}

// Cwd returns the working directory of the bootstrap process
// The path is validated against the chroot identified by `root`
func (b *simpleBootstrap) Cwd() (string, error) {
	if !filepath.IsAbs(b.workingDir) {
		return "", fmt.Errorf("the working directory '%s' is invalid, it needs to be an absolute path", b.workingDir)
	}

	// evaluate the path relatively to the domain's mnt namespace root
	if _, err := os.Stat(b.workingDir); os.IsNotExist(err) {
		return "", fmt.Errorf("the working directory doesn't exist: %s", b.workingDir)
	}

	return b.workingDir, nil
}

// Env returns the environment variables available to
// the bootstrap process
func (b *simpleBootstrap) Env(e *env.Environment) map[string]string {
	return e.RuntimeExecEnv()
}

// ExtraFiles returns the extra file descriptors apart from 1 & 2 to be passed to runtime
func (b *simpleBootstrap) ExtraFiles() []*os.File {
	return make([]*os.File, 0)
}

func (b *simpleBootstrap) CachedFatalError(err error) (fatalerror.ErrorType, string, bool) {
	// not implemented as it is not needed in Core but we need to fullfil the interface anyway
	return fatalerror.ErrorType(""), "", false
}
