// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"

	log "github.com/sirupsen/logrus"
)

type LogFormatter func(error) string
type BootstrapError func() (fatalerror.ErrorType, LogFormatter)

// Bootstrap represents a list of executable bootstrap
// candidates in order of priority and exec metadata
type Bootstrap struct {
	runtimeDomainRoot  string
	orderedLookupPaths []string
	validCmd           []string
	workingDir         string
	cmdCandidates      [][]string
	extraFiles         []*os.File
	bootstrapError     BootstrapError
}

// Validate interface compliance
var _ interop.Bootstrap = (*Bootstrap)(nil)

// NewBootstrap returns an instance of bootstrap defined by given params
func NewBootstrap(cmdCandidates [][]string, currentWorkingDir string, runtimeDomainRoot string) *Bootstrap {
	var orderedLookupBootstrapPaths []string
	for _, args := range cmdCandidates {
		// Empty args is an error, but we want to detect it later (in Cmd() call) when we are able to report a descriptive error
		if len(args) != 0 {
			orderedLookupBootstrapPaths = append(orderedLookupBootstrapPaths, args[0])
		}
	}

	if currentWorkingDir == "" {
		// use the root directory as the default working directory
		currentWorkingDir = "/"
	}

	if runtimeDomainRoot == "" {
		runtimeDomainRoot = "/"
	}

	return &Bootstrap{
		orderedLookupPaths: orderedLookupBootstrapPaths,
		workingDir:         currentWorkingDir,
		cmdCandidates:      cmdCandidates,
		runtimeDomainRoot:  runtimeDomainRoot,
	}
}

func NewBootstrapSingleCmd(cmd []string, currentWorkingDir string, runtimeDomainRoot string) *Bootstrap {
	if currentWorkingDir == "" {
		// use the root directory as the default working directory
		currentWorkingDir = "/"
	}
	if runtimeDomainRoot == "" {
		runtimeDomainRoot = "/"
	}

	// a single candidate command makes it automatically valid
	return &Bootstrap{
		validCmd:          cmd,
		workingDir:        currentWorkingDir,
		runtimeDomainRoot: runtimeDomainRoot,
	}
}

// locateBootstrap sets the first occurrence of an
// actual bootstrap, given a list of possible files
func (b *Bootstrap) locateBootstrap() error {
	for i, bootstrapCandidate := range b.orderedLookupPaths {
		// validate path relatively to the domain's root
		candidatPath := path.Join(b.runtimeDomainRoot, bootstrapCandidate)
		file, err := os.Stat(candidatPath)
		if err != nil {
			if !os.IsNotExist(err) {
				log.WithError(err).Warnf("Could not validate %s. Ignoring it.", bootstrapCandidate)
			}
			continue
		}
		if file.IsDir() {
			log.Warnf("%s is a directory. Ignoring it", bootstrapCandidate)
			continue
		}
		b.validCmd = b.cmdCandidates[i]
		return nil
	}
	log.WithField("bootstrapPathsChecked", b.orderedLookupPaths).Warn("Couldn't find valid bootstrap(s)")
	return fmt.Errorf("Couldn't find valid bootstrap(s): %s", b.orderedLookupPaths)
}

// Cmd returns the args of bootstrap, relative to the
// chroot idenfied by `root`, where args[0]
// is the path to executable
func (b *Bootstrap) Cmd() ([]string, error) {
	if len(b.validCmd) > 0 {
		return b.validCmd, nil
	}

	if err := b.locateBootstrap(); err != nil {
		return []string{}, err
	}

	log.Debug("Located runtime bootstrap", b.validCmd[0])
	return b.validCmd, nil
}

// Env returns the environment variables available to
// the bootstrap process
func (b *Bootstrap) Env(e interop.EnvironmentVariables) map[string]string {
	return e.RuntimeExecEnv()
}

// Cwd returns the working directory of the bootstrap process
// The path is validated against the chroot identified by `root`
func (b *Bootstrap) Cwd() (string, error) {
	if !filepath.IsAbs(b.workingDir) {
		return "", fmt.Errorf("the working directory '%s' is invalid, it needs to be an absolute path", b.workingDir)
	}

	// evaluate the path relatively to the domain's mnt namespace root
	domainPath := path.Join(b.runtimeDomainRoot, b.workingDir)
	if _, err := os.Stat(domainPath); os.IsNotExist(err) {
		return "", fmt.Errorf("the working directory doesn't exist: %s", domainPath)
	}

	return b.workingDir, nil
}

// SetExtraFiles sets the extra file descriptors apart from 1 & 2 to be passed to runtime
func (b *Bootstrap) SetExtraFiles(extraFiles []*os.File) {
	b.extraFiles = extraFiles
}

// ExtraFiles returns the extra file descriptors apart from 1 & 2 to be passed to runtime
func (b *Bootstrap) ExtraFiles() []*os.File {
	return b.extraFiles
}

// CachedFatalError returns a bootstrap error that occurred during startup and before init
// so that it can be reported back to the customer in a later phase
func (b *Bootstrap) CachedFatalError(err error) (fatalerror.ErrorType, string, bool) {
	if b.bootstrapError == nil {
		return fatalerror.ErrorType(""), "", false
	}

	fatalError, logFunc := b.bootstrapError()

	return fatalError, logFunc(err), true
}

// SetCachedFatalError sets a cached fatal error that occurred during startup and before init
// so that it can be reported back to the customer in a later phase
func (b *Bootstrap) SetCachedFatalError(bootstrapErrFn BootstrapError) {
	b.bootstrapError = bootstrapErrFn
}

// BootstrapErrInvalidLCISTaskConfig represents an error while parsing LCIS task config
func BootstrapErrInvalidLCISTaskConfig(err error) BootstrapError {
	return func() (fatalerror.ErrorType, LogFormatter) {
		return fatalerror.InvalidTaskConfig, SupernovaInvalidTaskConfigRepr(err)
	}
}

// BootstrapErrInvalidLCISEntrypoint represents an invalid LCIS entrypoint error
func BootstrapErrInvalidLCISEntrypoint(entrypoint []string, cmd []string, workingdir string) BootstrapError {
	return func() (fatalerror.ErrorType, LogFormatter) {
		return fatalerror.InvalidEntrypoint, SupernovaLaunchErrorRepr(entrypoint, cmd, workingdir)
	}
}

func BootstrapErrInvalidLCISWorkingDir(entrypoint []string, cmd []string, workingdir string) BootstrapError {
	return func() (fatalerror.ErrorType, LogFormatter) {
		return fatalerror.InvalidWorkingDir, SupernovaLaunchErrorRepr(entrypoint, cmd, workingdir)
	}
}

func SupernovaInvalidTaskConfigRepr(err error) func(error) string {
	return func(unused error) string {
		return fmt.Sprintf("IMAGE\tInvalid task config: %s", err)
	}
}

func SupernovaLaunchErrorRepr(entrypoint []string, cmd []string, workingDir string) func(error) string {
	return func(err error) string {
		return fmt.Sprintf("IMAGE\tLaunch error: %s\tEntrypoint: [%s]\tCmd: [%s]\tWorkingDir: [%s]",
			err,
			strings.Join(entrypoint, ","),
			strings.Join(cmd, ","),
			workingDir)
	}
}
