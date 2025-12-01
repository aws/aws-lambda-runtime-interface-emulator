// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	rieinvoke "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/aws-lambda-rie/internal/invoke"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/aws-lambda-rie/internal/telemetry"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/invoke"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/invoke/timeout"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/raptor"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/servicelogs"
	supvmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/supervisor/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils"
)

func Run(supv supvmodel.ProcessSupervisor, args []string, fileUtil utils.FileUtil, sigCh chan os.Signal) (*raptor.Server, *HTTPHandler, *raptor.App, error) {

	opts, args, err := ParseCLIArgs(args)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse command line arguments: %w", err)
	}

	ConfigureLogging(opts.LogLevel)

	runtimeAPIAddr, err := ParseAddr(opts.RuntimeAddress, "127.0.0.1:9001")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid runtime API address: %w", err)
	}

	rieAddr, err := ParseAddr(opts.RIEAddress, "0.0.0.0:8080")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid RIE address: %w", err)
	}

	telemetryAPIRelay := telemetry.NewRelay()
	eventsAPI := telemetry.NewEventsAPI(telemetryAPIRelay)

	responderFactoryFunc := func(_ context.Context, invokeReq interop.InvokeRequest) invoke.InvokeResponseSender {
		return rieinvoke.NewResponder(invokeReq)
	}
	invokeRouter := invoke.NewInvokeRouter(rapid.MaxIdleRuntimesQueueSize, eventsAPI, responderFactoryFunc, timeout.NewRecentCache())

	deps := rapid.Dependencies{
		EventsAPI:                eventsAPI,
		LogsEgressAPI:            telemetry.NewLogsEgress(telemetryAPIRelay, os.Stdout),
		TelemetrySubscriptionAPI: telemetry.NewSubscriptionAPI(telemetryAPIRelay, eventsAPI, eventsAPI),
		Supervisor:               supv,
		RuntimeAPIAddrPort:       runtimeAPIAddr,
		FileUtils:                fileUtil,
		InvokeRouter:             invokeRouter,
	}

	raptorApp, err := raptor.StartApp(deps, "", noOpLogger{})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not start runtime api server: %w", err)
	}

	initMsg, err := GetInitRequestMessage(fileUtil, args)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not build initialization parameters: %w", err)
	}

	rieApp := NewHTTPHandler(raptorApp, initMsg)
	s, err := raptor.StartServer(raptorApp, rieApp, &raptor.TCPAddress{AddrPort: rieAddr})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not start RIE server: %w", err)
	}
	slog.Debug("RIE server started")

	go func() {
		<-raptorApp.Done()
		s.Shutdown(raptorApp.Err())
	}()

	s.AttachShutdownSignalHandler(sigCh)

	return s, rieApp, raptorApp, nil
}

type noOpLogger struct{}

func (n noOpLogger) Log(_ servicelogs.Operation, _ time.Time, _ []servicelogs.Property, _ []servicelogs.Dimension, _ []servicelogs.Metric) {
}

func (n noOpLogger) SetInitData(_ interop.InitStaticDataProvider) {}

func (n noOpLogger) Close() error { return nil }
