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
	shutdownFuncs          []context.CancelFunc
	handler                string
}

type logSink int

const (
	RuntimeLogSink logSink = iota
	ExtensionLogSink
)

func NewSandboxBuilder() *SandboxBuilder {
	defaultInteropServer := NewServer(context.Background())
	signalCtx, cancelSignalCtx := context.WithCancel(context.Background())

	b := &SandboxBuilder{
		sandbox: &rapid.Sandbox{
			PreLoadTimeNs:      0, // TODO
			StandaloneMode:     true,
			LogsEgressAPI:      &telemetry.NoOpLogsEgressAPI{},
			EnableTelemetryAPI: false,
			Tracer:             telemetry.NewNoOpTracer(),
			SignalCtx:          signalCtx,
			EventsAPI:          &telemetry.NoOpEventsAPI{},
			InitCachingEnabled: false,
			Supervisor:         supervisor.NewLocalSupervisor(),
			RuntimeAPIHost:     "127.0.0.1",
			RuntimeAPIPort:     9001,
		},
		defaultInteropServer: defaultInteropServer,
		shutdownFuncs:        []context.CancelFunc{},
		lambdaInvokeAPI:      NewEmulatorAPI(defaultInteropServer),
	}

	b.AddShutdownFunc(context.CancelFunc(func() {
		log.Info("Shutting down...")
		defaultInteropServer.Reset("SandboxTerminated", defaultSigtermResetTimeoutMs)
		cancelSignalCtx()
	}))

	return b
}

func (b *SandboxBuilder) SetSupervisor(supervisor supvmodel.Supervisor) *SandboxBuilder {
	b.sandbox.Supervisor = supervisor
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

func (b *SandboxBuilder) SetEventsAPI(eventsAPI telemetry.EventsAPI) *SandboxBuilder {
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

func (b *SandboxBuilder) SetPreLoadTimeNs(preLoadTimeNs int64) *SandboxBuilder {
	b.sandbox.PreLoadTimeNs = preLoadTimeNs
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

func (b *SandboxBuilder) AddShutdownFunc(shutdownFunc context.CancelFunc) *SandboxBuilder {
	b.shutdownFuncs = append(b.shutdownFuncs, shutdownFunc)
	return b
}

func (b *SandboxBuilder) Create() (interop.SandboxContext, interop.InternalStateGetter) {
	if !b.useCustomInteropServer {
		b.sandbox.InteropServer = b.defaultInteropServer
	}

	go signalHandler(b.shutdownFuncs)

	rapidCtx, internalStateFn, runtimeAPIAddr := rapid.Start(b.sandbox)

	b.sandboxContext = &SandboxContext{
		rapidCtx:              rapidCtx,
		handler:               b.handler,
		runtimeAPIAddress:     runtimeAPIAddr,
		InvokeReceivedTime:    int64(0),
		InvokeResponseMetrics: nil,
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

// Trap SIGINT and SIGTERM signals and call shutdown function
func signalHandler(shutdownFuncs []context.CancelFunc) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	sigReceived := <-sig
	log.WithField("signal", sigReceived.String()).Info("Received signal")
	for _, shutdownFunc := range shutdownFuncs {
		shutdownFunc()
	}
}
