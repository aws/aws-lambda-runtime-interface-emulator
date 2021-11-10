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
)

type EnvironmentVariables interface {
	AgentExecEnv() []string
	RuntimeExecEnv() []string
	SetHandler(handler string)
	StoreRuntimeAPIEnvironmentVariable(runtimeAPIAddress string)
	StoreEnvironmentVariablesFromInit(customerEnv map[string]string,
		handler, awsKey, awsSecret, awsSession, funcName, funcVer string)
	StoreEnvironmentVariablesFromInitForInitCaching(host string, port int, customerEnv map[string]string, handler, funcName, funcVer, token string)
}

type Sandbox struct {
	EnableTelemetryAPI  bool
	StandaloneMode      bool
	Bootstrap           Bootstrap
	InteropServer       interop.Server
	Tracer              telemetry.Tracer
	LogsSubscriptionAPI telemetry.LogsSubscriptionAPI
	LogsEgressAPI       telemetry.LogsEgressAPI
	Environment         EnvironmentVariables
	DebugTailLogger     *logging.TailLogWriter
	PlatformLogger      logging.PlatformLogger
	RuntimeStdoutWriter io.Writer
	RuntimeStderrWriter io.Writer
	PreLoadTimeNs       int64
	Handler             string
	SignalCtx           context.Context
	EventsAPI           telemetry.EventsAPI
	InitCachingEnabled  bool
}

// Start is a public version of start() that exports only configurable parameters
func Start(s *Sandbox) {
	appCtx := appctx.NewApplicationContext()
	initFlow := core.NewInitFlowSynchronization()
	invokeFlow := core.NewInvokeFlowSynchronization()
	registrationService := core.NewRegistrationService(initFlow, invokeFlow)
	renderingService := rendering.NewRenderingService()
	credentialsService := core.NewCredentialsService()

	if s.StandaloneMode {
		s.InteropServer.SetInternalStateGetter(registrationService.GetInternalStateDescriptor(appCtx))
	}
	server := rapi.NewServer(RuntimeAPIHost, RuntimeAPIPort, appCtx, registrationService, renderingService, s.EnableTelemetryAPI, s.LogsSubscriptionAPI, s.InitCachingEnabled, credentialsService)

	postLoadTimeNs := metering.Monotime()

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
		credentialsService:  credentialsService,

		telemetryAPIEnabled: s.EnableTelemetryAPI,
		logsSubscriptionAPI: s.LogsSubscriptionAPI,
		logsEgressAPI:       s.LogsEgressAPI,
		bootstrap:           s.Bootstrap,
		interopServer:       s.InteropServer,
		xray:                s.Tracer,
		environment:         s.Environment,
		standaloneMode:      s.StandaloneMode,
		debugTailLogger:     s.DebugTailLogger,
		platformLogger:      s.PlatformLogger,
		runtimeStdoutWriter: s.RuntimeStdoutWriter,
		runtimeStderrWriter: s.RuntimeStderrWriter,
		preLoadTimeNs:       s.PreLoadTimeNs,
		eventsAPI:           s.EventsAPI,
		initCachingEnabled:  s.InitCachingEnabled,
	})
}
