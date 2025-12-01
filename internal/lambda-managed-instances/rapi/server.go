// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapi

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"

	"github.com/go-chi/chi"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
)

const (
	version20180601 = "/2018-06-01"
	version20200101 = "/2020-01-01"
	version20220701 = "/2022-07-01"
)

const requestBodyLimitBytes int64 = 1 * 1024 * 1024

type Server struct {
	runtimeAPIAddrPort netip.AddrPort
	server             *http.Server
	listener           net.Listener
	exit               chan error
}

func SaveConnInContext(ctx context.Context, c net.Conn) context.Context {
	return context.WithValue(ctx, interop.HTTPConnKey, c)
}

func NewServer(
	runtimeAPIAddrPort netip.AddrPort,
	appCtx appctx.ApplicationContext,
	registrationService core.RegistrationService,
	renderingService *rendering.EventRenderingService,
	telemetrySubscriptionAPI telemetry.SubscriptionAPI,
	runtimeReqHandler runtimeRequestHandler,
) (*Server, error) {
	exitErrors := make(chan error, 1)

	router := chi.NewRouter()
	router.Mount(version20180601, NewRouter(appCtx, registrationService, renderingService, runtimeReqHandler))
	router.Mount(version20200101, http.MaxBytesHandler(ExtensionsRouter(appCtx, registrationService, renderingService), requestBodyLimitBytes))

	router.Mount(version20220701, http.MaxBytesHandler(TelemetryAPIRouter(registrationService, telemetrySubscriptionAPI), requestBodyLimitBytes))

	listener, err := net.Listen("tcp", runtimeAPIAddrPort.String())
	if err != nil {
		return nil, err
	}

	return &Server{
		listener:           listener,
		runtimeAPIAddrPort: netip.MustParseAddrPort(listener.Addr().String()),

		server: &http.Server{
			Handler:     router,
			ConnContext: SaveConnInContext,
		},
		exit: exitErrors,
	}, nil
}

func (s *Server) Serve(ctx context.Context) error {
	defer func() {
		if err := s.Close(); err != nil {
			slog.Error("Error closing server", "err", err)
		}
	}()

	select {
	case err := <-s.serveAsync():
		return err

	case err := <-s.exit:
		slog.Error("Error triggered exit", "err", err)
		return err

	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) serveAsync() chan error {
	errors := make(chan error)
	go func() {
		errors <- s.server.Serve(s.listener)
	}()

	return errors
}

func (s *Server) AddrPort() netip.AddrPort {
	return s.runtimeAPIAddrPort
}

func (s *Server) URL(endpoint string) string {
	return fmt.Sprintf("http://%s%s%s", s.runtimeAPIAddrPort, version20180601, endpoint)
}

func (s *Server) Close() error {
	err := s.server.Close()
	if err == nil {
		slog.Info("Runtime API Server closed")
	}
	return err
}

func (s *Server) Shutdown() error {
	return s.server.Shutdown(context.Background())
}
