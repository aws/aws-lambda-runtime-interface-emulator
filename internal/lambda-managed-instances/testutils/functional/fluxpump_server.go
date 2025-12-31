// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build test

package functional

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"
)

const (
	passphraseHeader        = "Passphrase"
	extensionNameHeader     = "Lambda-Extension-Name"
	forwardedHeader         = "Forwarded"
	subscriptionAPIendpoint = "/subscribeV2"
)

type FluxPumpServer struct {
	requests []SubscribeRequestLog
	mutex    sync.Mutex
	server   *http.Server
	addrPort netip.AddrPort
}

type SubscribeRequestLog struct {
	AgentName  string
	Passphrase string
	Body       []byte
	Headers    map[string][]string
	RemoteAddr string
	Timestamp  time.Time
}

func NewFluxPumpServer() *FluxPumpServer {
	return &FluxPumpServer{
		requests: make([]SubscribeRequestLog, 0),
		mutex:    sync.Mutex{},
	}
}

func (s *FluxPumpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	slog.Debug("FluxPump received request",
		"method", r.Method,
		"path", r.URL.Path,
		"remoteAddr", r.RemoteAddr)

	if r.Method != http.MethodPut || r.URL.Path != subscriptionAPIendpoint {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("Error reading request body", "err", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	passphrase := r.Header.Get(passphraseHeader)
	agentName := r.Header.Get(extensionNameHeader)
	forwarded := r.Header.Get(forwardedHeader)

	SubscribeRequestLog := SubscribeRequestLog{
		AgentName:  agentName,
		Passphrase: passphrase,
		Body:       body,
		Headers:    r.Header,
		RemoteAddr: r.RemoteAddr,
		Timestamp:  time.Now(),
	}
	s.requests = append(s.requests, SubscribeRequestLog)

	slog.Debug("Subscription request details",
		"agentName", agentName,
		"passphrase", passphrase,
		"forwarded", forwarded,
		"bodySize", len(body))

	slog.Debug("Request body", "body", string(body))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(`{"message":"Subscription accepted"}`))
	if err != nil {
		panic(err)
	}
}

func (s *FluxPumpServer) GetRequests() []SubscribeRequestLog {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.requests
}

func (s *FluxPumpServer) GetAddrPort() netip.AddrPort {
	return s.addrPort
}

func (s *FluxPumpServer) Start(fxPumpAddrPort netip.AddrPort) error {

	listener, err := net.Listen("tcp", fxPumpAddrPort.String())
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	_ = listener.Close()

	s.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           s,
		ReadHeaderTimeout: 15 * time.Second,
	}

	s.addrPort = netip.MustParseAddrPort(fmt.Sprintf("127.0.0.1:%d", port))

	slog.Info("FluxPump server starting", "addrPort", s.addrPort.String())

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("FluxPump server error", "err", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	return nil
}

func (s *FluxPumpServer) Stop() error {
	if s.server == nil {
		return nil
	}
	slog.Info("Shutting down FluxPump server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}
