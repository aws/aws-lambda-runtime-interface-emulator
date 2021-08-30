// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/jessevdk/go-flags"
	"go.amzn.com/lambda/rapidcore"

	log "github.com/sirupsen/logrus"
)

const (
	optBootstrap     = "/opt/bootstrap"
	runtimeBootstrap = "/var/runtime/bootstrap"
)

type options struct {
	LogLevel string `long:"log-level" default:"info" description:"log level"`
}

func main() {
	// More frequent GC reduces the tail latencies, equivalent to export GOGC=33
	debug.SetGCPercent(33)

	opts, args := getCLIArgs()
	rapidcore.SetLogLevel(opts.LogLevel)

	bootstrap, handler := getBootstrap(args, opts)
	sandbox := rapidcore.NewSandboxBuilder(bootstrap).AddShutdownFunc(context.CancelFunc(func() { os.Exit(0) })).SetExtensionsFlag(true)

	if len(handler) > 0 {
		sandbox.SetHandler(handler)
	}

	go sandbox.Create()

	testAPIipport := "0.0.0.0:8080"
	startHTTPServer(testAPIipport, sandbox)
}

func getCLIArgs() (options, []string) {
	var opts options
	parser := flags.NewParser(&opts, flags.IgnoreUnknown)
	args, err := parser.ParseArgs(os.Args)

	if err != nil {
		log.WithError(err).Fatal("Failed to parse command line arguments:", os.Args)
	}

	return opts, args
}

func getBootstrap(args []string, opts options) (*rapidcore.Bootstrap, string) {
	var bootstrapLookupCmd []string
	var handler string
	currentWorkingDir := "/var/task" // default value

	if len(args) <= 1 {
		var bootstrapCmdCandidates = []string{
			fmt.Sprintf("%s/bootstrap", currentWorkingDir),
			optBootstrap,
			runtimeBootstrap,
		}

		// set default value to /var/task/bootstrap, but switch to the other options if it doesn't exist
		bootstrapLookupCmd = []string{bootstrapCmdCandidates[0]}

		for i, bootstrapCandidate := range bootstrapCmdCandidates {
			if file, err := os.Stat(bootstrapCandidate); !os.IsNotExist(err) && !file.IsDir() {
				bootstrapLookupCmd = []string{bootstrapCmdCandidates[i]}
				break
			}
		}

		// handler is used later to set an env var for Lambda Image support
		handler = ""
	} else if len(args) > 1 {

		bootstrapLookupCmd = args[1:]

		if cwd, err := os.Getwd(); err == nil {
			currentWorkingDir = cwd
		}

		if len(args) > 2 {
			// Assume last arg is the handler
			handler = args[len(args)-1]
		}

		log.Infof("exec '%s' (cwd=%s, handler=%s)", args[1], currentWorkingDir, handler)

	} else {
		log.Panic("insufficient arguments: bootstrap not provided")
	}

	return rapidcore.NewBootstrapSingleCmd(bootstrapLookupCmd, currentWorkingDir), handler
}
