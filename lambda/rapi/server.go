// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapi

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/go-chi/chi"
	"go.amzn.com/lambda/appctx"

	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/rapi/rendering"
	"go.amzn.com/lambda/telemetry"

	log "github.com/sirupsen/logrus"
)

const version20180601 = "/2018-06-01"
const version20200101 = "/2020-01-01"
const version20200815 = "/2020-08-15"

// Server is a Runtime API server
type Server struct {
	host     string
	port     int
	server   *http.Server
	listener net.Listener
	exit     chan error
}

// NewServer creates a new Runtime API Server
//
// Unlike net/http server's ListenAndServe, we separate Listen()
// and Serve(), this is done to guarantee order: call to Listen()
// should happen before provided runtime is started.
//
// When port is 0, OS will dynamically allocate the listening port.
func NewServer(host string, port int, appCtx appctx.ApplicationContext,
	registrationService core.RegistrationService,
	renderingService *rendering.EventRenderingService,
	telemetryAPIEnabled bool,
	telemetryService telemetry.LogsAPIService) *Server {

	exitErrors := make(chan error, 1)

	router := chi.NewRouter()
	router.Mount(version20180601, NewRouter(appCtx, registrationService, renderingService))
	router.Mount(version20200101, ExtensionsRouter(appCtx, registrationService, renderingService))

	if telemetryAPIEnabled {
		router.Mount(version20200815, TelemetryAPIRouter(registrationService, telemetryService))
	} else {
		router.Mount(version20200815, TelemetryAPIStubRouter())
	}

	return &Server{
		host:     host,
		port:     port,
		server:   &http.Server{Handler: router},
		listener: nil,
		exit:     exitErrors,
	}
}

// Listen on port
func (s *Server) Listen() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s.listener = ln
	if s.port == 0 {
		s.port = ln.Addr().(*net.TCPAddr).Port
		log.WithField("port", s.port).Info("Listening port was dynamically allocated")
	}

	log.Debugf("Runtime API Server listening on %s:%d", s.host, s.port)

	return nil
}

func (s *Server) IsListening() bool {
	return s.listener != nil
}

// Serve requests and close on cancelation signals
func (s *Server) Serve(ctx context.Context) error {
	defer s.Close()

	select {
	case err := <-s.serveAsync():
		return err

	case err := <-s.exit:
		log.Errorf("Error triggered exit: %s", err)
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

// Host is server's host
func (s *Server) Host() string {
	return s.host
}

// Port is server's port
func (s *Server) Port() int {
	return s.port
}

// URL is full server url for specified endpoint
func (s *Server) URL(endpoint string) string {
	return fmt.Sprintf("http://%s:%d%s%s", s.Host(), s.Port(), version20180601, endpoint)
}

// Close forcefully closes listeners & connections
func (s *Server) Close() error {
	err := s.server.Close()
	if err == nil {
		log.Info("Runtime API Server closed")
	}
	return err
}

// Shutdown gracefully shuts down server
func (s *Server) Shutdown() error {
	return s.server.Shutdown(context.Background())
}
