// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package lmds

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type Metadata struct {
	AvailabilityZoneID string `json:"AvailabilityZoneID"`
}

type MetadataConfig struct {
	Data   []byte
	MaxAge time.Duration
}

const URIPath = "/2026-01-15/metadata/execution-environment"

type Metrics struct {
	ClientErrors    uint64
	ServerErrors    uint64
	SuccessfulCalls uint64
}

type MetricStore struct {
	clientErrors    atomic.Uint64
	serverErrors    atomic.Uint64
	successfulCalls atomic.Uint64
}

func (m *MetricStore) incClientErrors() {
	m.clientErrors.Add(1)
}

func (m *MetricStore) incServerErrors() {
	m.serverErrors.Add(1)
}

func (m *MetricStore) incSuccessfulCalls() {
	m.successfulCalls.Add(1)
}

func (m *MetricStore) Take() Metrics {
	return Metrics{
		ClientErrors:    m.clientErrors.Swap(0),
		ServerErrors:    m.serverErrors.Swap(0),
		SuccessfulCalls: m.successfulCalls.Swap(0),
	}
}

type Service struct {
	token    string
	metadata atomic.Value
	Metrics  *MetricStore
}

func NewService(token string) *Service {
	return &Service{
		token:   token,
		Metrics: &MetricStore{},
	}
}

func (s *Service) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		s.Metrics.incClientErrors()
		http.Error(writer, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	authHeader := request.Header.Get("Authorization")
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		s.Metrics.incClientErrors()
		slog.Warn("metadata API handler received authorization header without Bearer prefix")
		http.Error(writer, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, bearerPrefix)

	if token != s.token {
		s.Metrics.incClientErrors()
		slog.Warn("metadata API handler received unexpected auth token")
		http.Error(writer, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	config, ok := s.metadata.Load().(MetadataConfig)
	if !ok {
		s.Metrics.incServerErrors()
		slog.Error("metadata API handler called before UpdateMetadata")
		http.Error(writer, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	cacheControl := fmt.Sprintf("private, max-age=%d, immutable", int(config.MaxAge.Seconds()))

	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", cacheControl)
	writer.WriteHeader(http.StatusOK)
	if _, err := writer.Write(config.Data); err != nil {
		s.Metrics.incClientErrors()
		slog.Error("could not write metadata", "err", err)
		return
	}
	s.Metrics.incSuccessfulCalls()
}

func (s *Service) UpdateMetadata(config MetadataConfig) {
	s.metadata.Store(config)
}
