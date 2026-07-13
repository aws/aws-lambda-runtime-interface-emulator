// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package raptor

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func StartServer(shutdownHandler shutdownHandler, handler http.Handler, addr Address, h2Only bool) (*Server, error) {
	listener, err := net.Listen(addr.Protocol(), addr.String())
	if err != nil {
		return nil, err
	}

	addr.UpdateFromListener(listener)

	s := &Server{
		httpServer: &http.Server{
			Handler:           handler,
			ReadHeaderTimeout: 15 * time.Second,
			Protocols:         &http.Protocols{},
		},
		doneCh:          make(chan struct{}),
		shutdownHandler: shutdownHandler,
		Addr:            addr,
	}
	if h2Only {

		s.httpServer.Protocols.SetUnencryptedHTTP2(true)

		s.httpServer.HTTP2 = &http.HTTP2Config{
			MaxConcurrentStreams: 2048,
		}
	} else {

		s.httpServer.Protocols.SetHTTP1(true)
	}

	go func() {
		if err := s.httpServer.Serve(listener); err != nil {

			s.Shutdown(err)
		}
	}()
	return s, nil
}

func (s *Server) Shutdown(err error) {
	s.shutdownOnce.Do(func() {
		s.shutdownHandler.Shutdown(model.NewClientError(err, model.ErrorSeverityFatal, model.ErrorExecutionEnvironmentShutdown))
		slog.Info("Shutting down HTTP server...")

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			slog.Warn("EA server graceful shutdown timed out, forcing close", "err", err)
			if closeErr := s.httpServer.Close(); closeErr != nil {
				slog.Warn("EA server force close failed", "err", closeErr)
			}
		}
		slog.Info("EA HTTP server closed", "duration", time.Since(start))

		if err != nil {
			s.err.Store(err)
		}
		close(s.doneCh)
	})
}

func (s *Server) Done() <-chan struct{} {
	return s.doneCh
}

func (s *Server) Err() error {
	if err := s.err.Load(); err != nil {
		return err.(error)
	}
	return nil
}

func (s *Server) AttachShutdownSignalHandler(sigCh chan os.Signal) {
	go func() {
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		sig := <-sigCh
		slog.Info("SignalHandler received:", "signal", sig)
		s.Shutdown(nil)
	}()
}

type Address interface {
	String() string
	Protocol() string

	UpdateFromListener(listener net.Listener)
}

type TCPAddress struct {
	AddrPort netip.AddrPort
}

func (t *TCPAddress) String() string {
	return t.AddrPort.String()
}

func (t *TCPAddress) Protocol() string {
	return "tcp"
}

func (t *TCPAddress) UpdateFromListener(listener net.Listener) {
	t.AddrPort = netip.MustParseAddrPort(listener.Addr().String())
}

type UnixAddress struct {
	Path string
}

func (u *UnixAddress) String() string {
	return u.Path
}

func (u *UnixAddress) Protocol() string {
	return "unix"
}

func (u *UnixAddress) UpdateFromListener(listener net.Listener) {

}

type shutdownHandler interface {
	Shutdown(shutdownReason model.AppError)
}

type Server struct {
	httpServer *http.Server
	Addr       Address

	shutdownHandler shutdownHandler

	shutdownOnce sync.Once
	doneCh       chan struct{}
	err          atomic.Value
}
