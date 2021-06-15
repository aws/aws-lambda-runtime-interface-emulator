// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"context"
	"fmt"
	"io"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/logging"
	"go.amzn.com/lambda/metering"
	"go.amzn.com/lambda/rapi"
	"go.amzn.com/lambda/rapi/rendering"
	"go.amzn.com/lambda/telemetry"

	log "github.com/sirupsen/logrus"
)

type EnvironmentVariables interface {
	AgentExecEnv() []string
	RuntimeExecEnv() []string
	SetHandler(handler string)
	StoreRuntimeAPIEnvironmentVariable(runtimeAPIAddress string)
	StoreEnvironmentVariablesFromInit(customerEnv map[string]string,
		handler, awsKey, awsSecret, awsSession, funcName, funcVer string)
}

type Sandbox struct {
	EnableTelemetryAPI bool
	StandaloneMode     bool
	Bootstrap          Bootstrap
	InteropServer      interop.Server
	Tracer             telemetry.Tracer
	TelemetryService   telemetry.LogsAPIService
	Environment        EnvironmentVariables
	DebugTailLogger    *logging.TailLogWriter
	PlatformLogger     logging.PlatformLogger
	ExtensionLogWriter io.Writer
	RuntimeLogWriter   io.Writer
	PreLoadTimeNs      int64
	Handler            string
	SignalCtx          context.Context
	RuntimeAPIHost     string
	RuntimeAPIPort     int
}

// Start is a public version of start() that exports only configurable parameters
func Start(s *Sandbox) {
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	invokeFlow := core.NewInvokeFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow, invokeFlow)
	renderingService := rendering.NewRenderingService()

	if s.StandaloneMode {
		s.InteropServer.SetInternalStateGetter(registrationService.GetInternalStateDescriptor(appCtx))
	}
	server := rapi.NewServer(s.RuntimeAPIHost, s.RuntimeAPIPort, appCtx, registrationService, renderingService, s.EnableTelemetryAPI, s.TelemetryService)

	postLoadTimeNs := metering.Monotime()

	// Start Runtime API Server
	// If the runtime port is 0, listen will set `port`
	err := server.Listen()
	if err != nil {
		log.WithError(err).Panic("Runtime API Server failed to listen")
	}

	runtimeAPIAddr := fmt.Sprintf("%s:%d", server.Host(), server.Port())
	s.Environment.StoreRuntimeAPIEnvironmentVariable(runtimeAPIAddr)

	appctx.StoreInteropServer(appCtx, s.InteropServer)

	start(s.SignalCtx, &rapidContext{
		server:              server,
		appCtx:              appCtx,
		postLoadTimeNs:      postLoadTimeNs,
		initDone:            false,
		initFlow:            initFlow,
		invokeFlow:          invokeFlow,
		registrationService: registrationService,
		renderingService:    renderingService,
		exitPidChan:         make(chan int),
		resetChan:           make(chan *interop.Reset),

		telemetryAPIEnabled: s.EnableTelemetryAPI,
		telemetryService:    s.TelemetryService,
		bootstrap:           s.Bootstrap,
		interopServer:       s.InteropServer,
		xray:                s.Tracer,
		environment:         s.Environment,
		standaloneMode:      s.StandaloneMode,
		debugTailLogger:     s.DebugTailLogger,
		platformLogger:      s.PlatformLogger,
		extensionLogWriter:  s.ExtensionLogWriter,
		runtimeLogWriter:    s.RuntimeLogWriter,
		preLoadTimeNs:       s.PreLoadTimeNs,
	})
}
