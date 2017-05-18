// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"os"

	"go.amzn.com/lambda/fatalerror"
)

type Bootstrap interface {
	Cmd() ([]string, error)              // returns the args of bootstrap, where args[0] is the path to executable
	Env(e EnvironmentVariables) []string // returns the environment variables to be passed to the bootstrapped process
	Cwd() string                         // returns the working directory of the bootstrap process
	ExtraFiles() []*os.File              // returns the extra file descriptors apart from 1 & 2 to be passed to runtime
	CachedFatalError(err error) (fatalerror.ErrorType, string, bool)
}
