// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"context"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"go.amzn.com/lambda/extensions"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/logging"
	"go.amzn.com/lambda/rapid"
	"go.amzn.com/lambda/supervisor"
	supvmodel "go.amzn.com/lambda/supervisor/model"
	"go.amzn.com/lambda/telemetry"

	log "github.com/sirupsen/logrus"
)

const (
	defaultSigtermResetTimeoutMs = int64(2000)
)

type SandboxBuilder struct {
	sandbox                *rapid.Sandbox
	sandboxContext         interop.SandboxContext
	lambdaInvokeAPI        LambdaInvokeAPI
	defaultInteropServer   *Server
	useCustomInteropServer bool
	shutdownFuncs          []func()
	handler                string
}

type logSink int

const (
	RuntimeLogSink logSink = iota
	ExtensionLogSink
)

func NewSandboxBuilder() *SandboxBuilder {
	defaultInteropServer := NewServer()

	localSv := supervisor.NewLocalSupervisor()
	b := &SandboxBuilder{
		sandbox: &rapid.Sandbox{
			StandaloneMode:     true,
			LogsEgressAPI:      &telemetry.NoOpLogsEgressAPI{},
			EnableTelemetryAPI: false,
			Tracer:             telemetry.NewNoOpTracer(),
			EventsAPI:          &telemetry.NoOpEventsAPI{},
			InitCachingEnabled: false,
			Supervisor:         localSv,
			RuntimeFsRootPath:  localSv.RootPath,
			RuntimeAPIHost:     "127.0.0.1",
			RuntimeAPIPort:     9001,
		},
		defaultInteropServer: defaultInteropServer,
		shutdownFuncs:        []func(){},
		lambdaInvokeAPI:      NewEmulatorAPI(defaultInteropServer),
	}

	b.AddShutdownFunc(func() {
		log.Info("Shutting down...")
		defaultInteropServer.Reset("SandboxTerminated", defaultSigtermResetTimeoutMs)
	})

	return b
}

func (b *SandboxBuilder) SetSupervisor(supervisor supvmodel.ProcessSupervisor) *SandboxBuilder {
	b.sandbox.Supervisor = supervisor
	return b
}

func (b *SandboxBuilder) SetRuntimeFsRootPath(rootPath string) *SandboxBuilder {
	b.sandbox.RuntimeFsRootPath = rootPath
	return b
}

func (b *SandboxBuilder) SetRuntimeAPIAddress(runtimeAPIAddress string) *SandboxBuilder {
	host, port, err := net.SplitHostPort(runtimeAPIAddress)
	if err != nil {
		log.WithError(err).Warnf("Failed to parse RuntimeApiAddress: %s:", runtimeAPIAddress)
		return b
	}

	portInt, err := strconv.Atoi(port)
	if err != nil {
		log.WithError(err).Warnf("Failed to parse RuntimeApiPort: %s:", port)
		return b
	}

	b.sandbox.RuntimeAPIHost = host
	b.sandbox.RuntimeAPIPort = portInt
	return b
}

func (b *SandboxBuilder) SetInteropServer(interopServer interop.Server) *SandboxBuilder {
	b.sandbox.InteropServer = interopServer
	b.useCustomInteropServer = true
	return b
}

func (b *SandboxBuilder) SetEventsAPI(eventsAPI interop.EventsAPI) *SandboxBuilder {
	b.sandbox.EventsAPI = eventsAPI
	return b
}

func (b *SandboxBuilder) SetTracer(tracer telemetry.Tracer) *SandboxBuilder {
	b.sandbox.Tracer = tracer
	return b
}

func (b *SandboxBuilder) DisableStandaloneMode() *SandboxBuilder {
	b.sandbox.StandaloneMode = false
	return b
}

func (b *SandboxBuilder) SetExtensionsFlag(extensionsEnabled bool) *SandboxBuilder {
	if extensionsEnabled {
		extensions.Enable()
	} else {
		extensions.Disable()
	}
	return b
}

func (b *SandboxBuilder) SetInitCachingFlag(initCachingEnabled bool) *SandboxBuilder {
	b.sandbox.InitCachingEnabled = initCachingEnabled
	return b
}

func (b *SandboxBuilder) SetTelemetrySubscription(logsSubscriptionAPI telemetry.SubscriptionAPI, telemetrySubscriptionAPI telemetry.SubscriptionAPI) *SandboxBuilder {
	b.sandbox.EnableTelemetryAPI = true
	b.sandbox.LogsSubscriptionAPI = logsSubscriptionAPI
	b.sandbox.TelemetrySubscriptionAPI = telemetrySubscriptionAPI
	return b
}

func (b *SandboxBuilder) SetLogsEgressAPI(logsEgressAPI telemetry.StdLogsEgressAPI) *SandboxBuilder {
	b.sandbox.LogsEgressAPI = logsEgressAPI
	return b
}

func (b *SandboxBuilder) SetHandler(handler string) *SandboxBuilder {
	b.handler = handler
	return b
}

func (b *SandboxBuilder) AddShutdownFunc(shutdownFunc func()) *SandboxBuilder {
	b.shutdownFuncs = append(b.shutdownFuncs, shutdownFunc)
	return b
}

func (b *SandboxBuilder) Create() (interop.SandboxContext, interop.InternalStateGetter) {
	if !b.useCustomInteropServer {
		b.sandbox.InteropServer = b.defaultInteropServer
	}

	ctx, cancel := context.WithCancel(context.Background())

	// cancel is called when handling termination signals as a cancellation
	// signal to the Runtime API sever to terminate gracefully
	go signalHandler(cancel, b.shutdownFuncs)

	// rapid.Start, among other things, starts the Runtime API server and
	// terminates it gracefully if the cxt is canceled
	rapidCtx, internalStateFn, runtimeAPIAddr := rapid.Start(ctx, b.sandbox)

	b.sandboxContext = &SandboxContext{
		rapidCtx:          rapidCtx,
		handler:           b.handler,
		runtimeAPIAddress: runtimeAPIAddr,
	}

	return b.sandboxContext, internalStateFn
}

func (b *SandboxBuilder) DefaultInteropServer() *Server {
	return b.defaultInteropServer
}

func (b *SandboxBuilder) LambdaInvokeAPI() LambdaInvokeAPI {
	return b.lambdaInvokeAPI
}

// SetLogLevel sets the log level for internal logging. Needs to be called very
// early during startup to configure logs emitted during initialization
func SetLogLevel(logLevel string) {
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.WithError(err).Fatal("Failed to set log level. Valid log levels are:", log.AllLevels)
	}

	log.SetLevel(level)
	log.SetFormatter(&logging.InternalFormatter{})
}

func SetInternalLogOutput(w io.Writer) {
	logging.SetOutput(w)
}

// Trap SIGINT and SIGTERM signals, call shutdown function, and cancel the
// ctx to terminate gracefully the Runtime API server
func signalHandler(cancel context.CancelFunc, shutdownFuncs []func()) {
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	sigReceived := <-sig
	log.WithField("signal", sigReceived.String()).Info("Received signal")
	for _, shutdownFunc := range shutdownFuncs {
		shutdownFunc()
	}
}
