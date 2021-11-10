// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go.amzn.com/lambda/core/statejson"
	"go.amzn.com/lambda/extensions"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/logging"
	"go.amzn.com/lambda/rapid"
	"go.amzn.com/lambda/rapidcore/env"
	"go.amzn.com/lambda/telemetry"

	log "github.com/sirupsen/logrus"
)

const (
	defaultSigtermResetTimeoutMs = int64(2000)
)

type Sandbox interface {
	Init(i *interop.Init, invokeTimeoutMs int64)
	Invoke(responseWriter http.ResponseWriter, invoke *interop.Invoke) error
	InteropServer() InteropServer
}

type ReserveResponse struct {
	Token         interop.Token
	InternalState *statejson.InternalStateDescription
}

type InteropServer interface {
	FastInvoke(w http.ResponseWriter, i *interop.Invoke, direct bool) error
	Reserve(id string, traceID, lambdaSegmentID string) (*ReserveResponse, error)
	Reset(reason string, timeoutMs int64) (*statejson.ResetDescription, error)
	AwaitRelease() (*statejson.InternalStateDescription, error)
	Shutdown(shutdown *interop.Shutdown) *statejson.InternalStateDescription
	InternalState() (*statejson.InternalStateDescription, error)
	CurrentToken() *interop.Token
}

type SandboxBuilder struct {
	sandbox                *rapid.Sandbox
	defaultInteropServer   *Server
	useCustomInteropServer bool
	shutdownFuncs          []context.CancelFunc
	debugTailLogWriter     io.Writer
	platformLogWriter      io.Writer
}

type logSink int

const (
	RuntimeLogSink logSink = iota
	ExtensionLogSink
)

func NewSandboxBuilder(bootstrap *Bootstrap) *SandboxBuilder {
	defaultInteropServer := NewServer(context.Background())
	signalCtx, cancelSignalCtx := context.WithCancel(context.Background())
	logsEgressAPI := &telemetry.NoOpLogsEgressAPI{}
	runtimeStdoutWriter, runtimeStderrWriter, _ := logsEgressAPI.GetRuntimeSockets()

	b := &SandboxBuilder{
		sandbox: &rapid.Sandbox{
			Bootstrap:           bootstrap,
			PreLoadTimeNs:       0, // TODO
			StandaloneMode:      true,
			RuntimeStdoutWriter: runtimeStdoutWriter,
			RuntimeStderrWriter: runtimeStderrWriter,
			LogsEgressAPI:       logsEgressAPI,
			EnableTelemetryAPI:  false,
			Environment:         env.NewEnvironment(),
			Tracer:              telemetry.NewNoOpTracer(),
			SignalCtx:           signalCtx,
			EventsAPI:           &telemetry.NoOpEventsAPI{},
			InitCachingEnabled:  false,
		},
		defaultInteropServer: defaultInteropServer,
		shutdownFuncs:        []context.CancelFunc{},
		debugTailLogWriter:   ioutil.Discard,
		platformLogWriter:    ioutil.Discard,
	}

	b.AddShutdownFunc(context.CancelFunc(func() {
		log.Info("Shutting down...")
		defaultInteropServer.Reset("SandboxTerminated", defaultSigtermResetTimeoutMs)
		cancelSignalCtx()
	}))

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

func (b *SandboxBuilder) SetEnvironmentVariables(environment *env.Environment) *SandboxBuilder {
	b.sandbox.Environment = environment
	return b
}

func (b *SandboxBuilder) SetPlatformLogOutput(w io.Writer) *SandboxBuilder {
	b.platformLogWriter = w
	return b
}

func (b *SandboxBuilder) SetTailLogOutput(w io.Writer) *SandboxBuilder {
	b.debugTailLogWriter = w
	return b
}

func (b *SandboxBuilder) SetLogsSubscriptionAPI(logsSubscriptionAPI telemetry.LogsSubscriptionAPI) *SandboxBuilder {
	b.sandbox.EnableTelemetryAPI = true
	b.sandbox.LogsSubscriptionAPI = logsSubscriptionAPI
	return b
}

func (b *SandboxBuilder) SetLogsEgressAPI(logsEgressAPI telemetry.LogsEgressAPI) *SandboxBuilder {
	runtimeStdoutWriter, runtimeStderrWriter, err := logsEgressAPI.GetRuntimeSockets()

	if err != nil {
		log.WithError(err).Fatal("failed to get the Runtime sockets from the logs egress API")
	}

	b.sandbox.LogsEgressAPI = logsEgressAPI
	b.sandbox.RuntimeStdoutWriter = runtimeStdoutWriter
	b.sandbox.RuntimeStderrWriter = runtimeStderrWriter
	return b
}

func (b *SandboxBuilder) SetHandler(handler string) *SandboxBuilder {
	b.sandbox.Handler = handler
	return b
}

func (b *SandboxBuilder) AddShutdownFunc(shutdownFunc context.CancelFunc) *SandboxBuilder {
	b.shutdownFuncs = append(b.shutdownFuncs, shutdownFunc)
	return b
}

func (b *SandboxBuilder) setupLoggingWithDebugLogs() {
	// Compose debug log writer with all log sinks. Debug log writer w
	// will not write logs when disabled by invoke parameter
	b.sandbox.DebugTailLogger = logging.NewTailLogWriter(b.debugTailLogWriter)
	b.sandbox.PlatformLogger = logging.NewPlatformLogger(b.platformLogWriter, b.sandbox.DebugTailLogger)
	b.sandbox.RuntimeStdoutWriter = io.MultiWriter(b.sandbox.DebugTailLogger, b.sandbox.RuntimeStdoutWriter)
	b.sandbox.RuntimeStderrWriter = io.MultiWriter(b.sandbox.DebugTailLogger, b.sandbox.RuntimeStderrWriter)
}

func (b *SandboxBuilder) Create() {
	if len(b.sandbox.Handler) > 0 {
		b.sandbox.Environment.SetHandler(b.sandbox.Handler)
	}

	if !b.useCustomInteropServer {
		b.sandbox.InteropServer = b.defaultInteropServer
	}

	b.setupLoggingWithDebugLogs()

	go signalHandler(b.shutdownFuncs)

	rapid.Start(b.sandbox)
}

func (b *SandboxBuilder) Init(i *interop.Init, timeoutMs int64) {
	b.sandbox.InteropServer.Init(&interop.Start{
		Handler:                      i.Handler,
		CorrelationID:                i.CorrelationID,
		AwsKey:                       i.AwsKey,
		AwsSecret:                    i.AwsSecret,
		AwsSession:                   i.AwsSession,
		XRayDaemonAddress:            i.XRayDaemonAddress,
		FunctionName:                 i.FunctionName,
		FunctionVersion:              i.FunctionVersion,
		CustomerEnvironmentVariables: i.CustomerEnvironmentVariables,
	}, timeoutMs)
}

func (b *SandboxBuilder) Invoke(w http.ResponseWriter, i *interop.Invoke) error {
	return b.sandbox.InteropServer.Invoke(w, i)
}

func (b *SandboxBuilder) InteropServer() InteropServer {
	return b.defaultInteropServer
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
