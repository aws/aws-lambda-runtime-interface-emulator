// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime/debug"

	"github.com/jessevdk/go-flags"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapidcore"

	log "github.com/sirupsen/logrus"
)

const (
	optBootstrap     = "/opt/bootstrap"
	runtimeBootstrap = "/var/runtime/bootstrap"
)

type options struct {
	LogLevel           string `long:"log-level" description:"The level of AWS Lambda Runtime Interface Emulator logs to display. Can also be set by the environment variable 'LOG_LEVEL'. Defaults to the value 'info'."`
	InitCachingEnabled bool   `long:"enable-init-caching" description:"Enable support for Init Caching"`
	// Do not have a default value so we do not need to keep it in sync with the default value in lambda/rapidcore/sandbox_builder.go
	RuntimeAPIAddress               string `long:"runtime-api-address" description:"The address of the AWS Lambda Runtime API to communicate with the Lambda execution environment."`
	RuntimeInterfaceEmulatorAddress string `long:"runtime-interface-emulator-address" default:"0.0.0.0:8080" description:"The address for the AWS Lambda Runtime Interface Emulator to accept HTTP request upon."`
}

func main() {
	// More frequent GC reduces the tail latencies, equivalent to export GOGC=33
	debug.SetGCPercent(33)

	opts, args := getCLIArgs()

	logLevel := "info"

	// If you specify an option by using a parameter on the CLI command line, it overrides any value from either the corresponding environment variable.
	//
	// https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html
	if opts.LogLevel != "" {
		logLevel = opts.LogLevel
	} else if envLogLevel, envLogLevelSet := os.LookupEnv("LOG_LEVEL"); envLogLevelSet {
		logLevel = envLogLevel
	}

	rapidcore.SetLogLevel(logLevel)

	if opts.RuntimeAPIAddress != "" {
		_, _, err := net.SplitHostPort(opts.RuntimeAPIAddress)

		if err != nil {
			log.WithError(err).Fatalf("The command line value for \"--runtime-api-address\" is not a valid network address %q.", opts.RuntimeAPIAddress)
		}
	}

	_, _, err := net.SplitHostPort(opts.RuntimeInterfaceEmulatorAddress)

	if err != nil {
		log.WithError(err).Fatalf("The command line value for \"--runtime-interface-emulator-address\" is not a valid network address %q.", opts.RuntimeInterfaceEmulatorAddress)
	}

	bootstrap, handler := getBootstrap(args, opts)
	sandbox := rapidcore.
		NewSandboxBuilder().
		AddShutdownFunc(context.CancelFunc(func() { os.Exit(0) })).
		SetExtensionsFlag(true).
		SetInitCachingFlag(opts.InitCachingEnabled)

	if len(handler) > 0 {
		sandbox.SetHandler(handler)
	}

	if opts.RuntimeAPIAddress != "" {
		sandbox.SetRuntimeAPIAddress(opts.RuntimeAPIAddress)
	}

	sandboxContext, internalStateFn := sandbox.Create()
	// Since we have not specified a custom interop server for standalone, we can
	// directly reference the default interop server, which is a concrete type
	sandbox.DefaultInteropServer().SetSandboxContext(sandboxContext)
	sandbox.DefaultInteropServer().SetInternalStateGetter(internalStateFn)

	startHTTPServer(opts.RuntimeInterfaceEmulatorAddress, sandbox, bootstrap)
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

func isBootstrapFileExist(filePath string) bool {
	file, err := os.Stat(filePath)
	return !os.IsNotExist(err) && !file.IsDir()
}

func getBootstrap(args []string, opts options) (interop.Bootstrap, string) {
	var bootstrapLookupCmd []string
	var handler string
	currentWorkingDir := "/var/task" // default value

	if len(args) <= 1 {
		// set default value to /var/task/bootstrap, but switch to the other options if it doesn't exist
		bootstrapLookupCmd = []string{
			fmt.Sprintf("%s/bootstrap", currentWorkingDir),
		}

		if !isBootstrapFileExist(bootstrapLookupCmd[0]) {
			var bootstrapCmdCandidates = []string{
				optBootstrap,
				runtimeBootstrap,
			}

			for i, bootstrapCandidate := range bootstrapCmdCandidates {
				if isBootstrapFileExist(bootstrapCandidate) {
					bootstrapLookupCmd = []string{bootstrapCmdCandidates[i]}
					break
				}
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

	return NewSimpleBootstrap(bootstrapLookupCmd, currentWorkingDir), handler
}
