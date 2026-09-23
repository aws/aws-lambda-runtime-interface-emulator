// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rie

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/interop"
	standalonetelemetry "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapidcore/standalone/telemetry"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/telemetry"
)

// TelemetrySubscriptionService implements the Telemetry API for the emulator: it
// accepts subscriptions from extensions and delivers the sandbox's events to
// them in the format the real service uses.
//
// Event records are produced by the same code that already reports platform
// events, so subscribers receive the documented types (platform.initStart,
// platform.start, platform.runtimeDone, platform.report, function, extension,
// ...) rather than a single undifferentiated stream.
type TelemetrySubscriptionService struct {
	lock          sync.Mutex
	subscriptions map[string]*subscription
	metrics       map[string]int
	off           bool

	// Events produced before any extension has subscribed, replayed to each new
	// subscriber. Initialization telemetry is emitted before extensions have had
	// the chance to subscribe, and is exactly what an extension reporting cold
	// start needs, so dropping it would lose the most valuable records.
	earlyEvents []telemetryRecord
}

// maxEarlyEvents bounds the pre-subscription buffer, so an environment whose
// extensions never subscribe doesn't accumulate records for its whole life.
const maxEarlyEvents = 1000

// Defaults from the Telemetry API documentation, applied when a subscription
// omits the corresponding buffering field.
const (
	defaultTimeoutMs = 1000
	defaultMaxItems  = 1000
	defaultMaxBytes  = 262144
)

type subscribeRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	Types         []string
	Buffering     struct {
		TimeoutMs int   `json:"timeoutMs"`
		MaxItems  int   `json:"maxItems"`
		MaxBytes  int64 `json:"maxBytes"`
	}
	Destination struct {
		Protocol string `json:"protocol"`
		URI      string `json:"URI"`
	}
}

// telemetryRecord is one delivered event, in the shape the Telemetry API defines.
// The record is an object for platform events and a string for log lines.
type telemetryRecord struct {
	Time   string      `json:"time"`
	Type   string      `json:"type"`
	Record interface{} `json:"record"`
}

type subscription struct {
	agentName   string
	destination string
	types       map[string]bool
	timeout     time.Duration
	maxItems    int
	maxBytes    int64

	lock    sync.Mutex
	pending []telemetryRecord
	bytes   int64
	timer   *time.Timer
}

func NewTelemetrySubscriptionService() *TelemetrySubscriptionService {
	return &TelemetrySubscriptionService{
		subscriptions: map[string]*subscription{},
		metrics:       map[string]int{},
	}
}

// Subscribe registers an extension's interest in some subset of telemetry.
func (s *TelemetrySubscriptionService) Subscribe(agentName string, body io.Reader, headers map[string][]string, remoteAddr string) ([]byte, int, map[string][]string, error) {
	s.lock.Lock()
	off := s.off
	s.lock.Unlock()
	if off {
		return nil, http.StatusBadRequest, nil, fmt.Errorf("%s", s.GetServiceClosedErrorMessage())
	}

	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, http.StatusInternalServerError, nil, err
	}

	var request subscribeRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return errorResponse("InvalidRequest", "Invalid subscription request"), http.StatusBadRequest, nil, nil
	}
	if !strings.EqualFold(request.Destination.Protocol, "HTTP") {
		return errorResponse("InvalidRequest", "Only the HTTP destination protocol is supported"), http.StatusBadRequest, nil, nil
	}
	destination, err := resolveDestination(request.Destination.URI)
	if err != nil {
		return errorResponse("InvalidRequest", "Invalid destination URI"), http.StatusBadRequest, nil, nil
	}
	if len(request.Types) == 0 {
		return errorResponse("InvalidRequest", "At least one telemetry type is required"), http.StatusBadRequest, nil, nil
	}

	sub := &subscription{
		agentName:   agentName,
		destination: destination,
		types:       map[string]bool{},
		timeout:     durationOrDefault(request.Buffering.TimeoutMs, defaultTimeoutMs),
		maxItems:    intOrDefault(request.Buffering.MaxItems, defaultMaxItems),
		maxBytes:    int64OrDefault(request.Buffering.MaxBytes, defaultMaxBytes),
	}
	for _, name := range request.Types {
		sub.types[strings.ToLower(name)] = true
	}

	s.lock.Lock()
	s.subscriptions[agentName] = sub
	early := make([]telemetryRecord, len(s.earlyEvents))
	copy(early, s.earlyEvents)
	s.lock.Unlock()

	for _, record := range early {
		if sub.wants(record.Type) {
			sub.enqueue(record)
		}
	}

	log.Infof("Telemetry API: extension %s subscribed to %v, delivering to %s", agentName, request.Types, destination)

	// The real service reports each subscription as telemetry in its own right.
	s.Dispatch(standalonetelemetry.SandboxEvent{
		Time: time.Now().Format(time.RFC3339),
		Type: "platform.telemetrySubscription",
		PlatformEvent: map[string]interface{}{
			"name":  agentName,
			"state": "Subscribed",
			"types": request.Types,
		},
	})

	return []byte("OK"), http.StatusOK, map[string][]string{}, nil
}

// Dispatch queues a sandbox event for every subscriber that asked for its type.
//
// Deliberately not gated on off: the sandbox closes the subscription window as
// soon as initialization finishes, which stops new subscriptions but must leave
// delivery to existing ones running for the life of the environment.
func (s *TelemetrySubscriptionService) Dispatch(event standalonetelemetry.SandboxEvent) {
	record := telemetryRecord{Time: event.Time, Type: event.Type}
	if event.PlatformEvent != nil {
		record.Record = event.PlatformEvent
	} else {
		record.Record = event.LogMessage
	}

	// Buffering and delivery are chosen under a single held lock. Deciding in one
	// lock section and acting in another lets a Subscribe interleave: it would
	// insert itself and replay a copy of the buffer that does not yet hold this
	// event, and the event would then be buffered for nobody.
	s.lock.Lock()
	if len(s.subscriptions) == 0 {
		if len(s.earlyEvents) < maxEarlyEvents {
			s.earlyEvents = append(s.earlyEvents, record)
		}
		s.lock.Unlock()
		return
	}
	subscriptions := make([]*subscription, 0, len(s.subscriptions))
	for _, sub := range s.subscriptions {
		subscriptions = append(subscriptions, sub)
	}
	s.lock.Unlock()

	for _, sub := range subscriptions {
		if sub.wants(event.Type) {
			sub.enqueue(record)
		}
	}
}

// wants reports whether a subscription covers an event type. Subscriptions name
// categories ("platform", "function", "extension"); platform events carry a
// dotted subtype, such as platform.runtimeDone.
func (sub *subscription) wants(eventType string) bool {
	category := eventType
	if index := strings.IndexRune(eventType, '.'); index != -1 {
		category = eventType[:index]
	}
	return sub.types[category]
}

// enqueue buffers a record, flushing when the subscription's limits are reached
// and otherwise scheduling a flush for its timeout.
func (sub *subscription) enqueue(record telemetryRecord) {
	sub.lock.Lock()

	sub.pending = append(sub.pending, record)
	if encoded, err := json.Marshal(record); err == nil {
		sub.bytes += int64(len(encoded))
	}

	if len(sub.pending) >= sub.maxItems || sub.bytes >= sub.maxBytes {
		batch := sub.take()
		sub.lock.Unlock()
		sub.deliver(batch)
		return
	}

	if sub.timer == nil {
		sub.timer = time.AfterFunc(sub.timeout, func() {
			sub.lock.Lock()
			batch := sub.take()
			sub.lock.Unlock()
			sub.deliver(batch)
		})
	}
	sub.lock.Unlock()
}

// take removes and returns everything buffered. Callers must hold the lock.
func (sub *subscription) take() []telemetryRecord {
	batch := sub.pending
	sub.pending = nil
	sub.bytes = 0
	if sub.timer != nil {
		sub.timer.Stop()
		sub.timer = nil
	}
	return batch
}

func (sub *subscription) deliver(batch []telemetryRecord) {
	if len(batch) == 0 {
		return
	}
	body, err := json.Marshal(batch)
	if err != nil {
		log.WithError(err).Warn("Telemetry API: could not encode a batch")
		return
	}
	response, err := deliveryClient.Post(sub.destination, "application/json", bytes.NewReader(body))
	if err != nil {
		log.WithError(err).Warnf("Telemetry API: could not deliver to %s", sub.destination)
		return
	}
	defer response.Body.Close()
	io.Copy(io.Discard, response.Body)
	if response.StatusCode >= 300 {
		log.Warnf("Telemetry API: %s answered %s", sub.destination, response.Status)
	}
}

// Flush delivers everything buffered, so that telemetry produced late in an
// invocation isn't stranded when the sandbox is about to be frozen.
func (s *TelemetrySubscriptionService) Flush() {
	s.lock.Lock()
	subscriptions := make([]*subscription, 0, len(s.subscriptions))
	for _, sub := range s.subscriptions {
		subscriptions = append(subscriptions, sub)
	}
	s.lock.Unlock()

	for _, sub := range subscriptions {
		sub.lock.Lock()
		batch := sub.take()
		sub.lock.Unlock()
		sub.deliver(batch)
	}
}

func (s *TelemetrySubscriptionService) RecordCounterMetric(metricName string, count int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.metrics[metricName] += count
}

func (s *TelemetrySubscriptionService) FlushMetrics() interop.TelemetrySubscriptionMetrics {
	s.lock.Lock()
	defer s.lock.Unlock()
	metrics := interop.TelemetrySubscriptionMetrics{}
	for name, count := range s.metrics {
		metrics[name] = count
	}
	s.metrics = map[string]int{}
	return metrics
}

// Clear drops all subscriptions, which the sandbox does when an execution
// environment is reset and its extensions are gone.
func (s *TelemetrySubscriptionService) Clear() {
	s.Flush()
	s.lock.Lock()
	defer s.lock.Unlock()
	s.subscriptions = map[string]*subscription{}
	s.earlyEvents = nil
	s.off = false
}

// TurnOff closes the subscription window. Existing subscriptions keep receiving
// telemetry; only further Subscribe calls are refused.
func (s *TelemetrySubscriptionService) TurnOff() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.off = true
}

func (s *TelemetrySubscriptionService) GetEndpointURL() string { return telemetryEndpointPath }

func (s *TelemetrySubscriptionService) GetServiceClosedErrorMessage() string {
	return "Telemetry API is no longer accepting subscriptions"
}

func (s *TelemetrySubscriptionService) GetServiceClosedErrorType() string {
	return "Telemetry.SubscriptionClosed"
}

const telemetryEndpointPath = "/2022-07-01/telemetry"

// Batches that reach a size limit are delivered on the goroutine producing the
// event, so a subscriber that accepts a connection and never answers would stall
// the sandbox's event pipeline. The default client has no timeout; this one gives
// up instead.
var deliveryClient = &http.Client{Timeout: 5 * time.Second}

// resolveDestination rewrites the hostname extensions are told to use. Inside a
// real execution environment "sandbox" resolves to the host running the runtime;
// in the emulator everything shares one network namespace.
func resolveDestination(uri string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" {
		return "", fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	if parsed.Hostname() == "sandbox" {
		host := "localhost"
		if port := parsed.Port(); port != "" {
			host += ":" + port
		}
		parsed.Host = host
	}
	return parsed.String(), nil
}

func errorResponse(errorType, message string) []byte {
	body, _ := json.Marshal(map[string]string{"errorType": errorType, "errorMessage": message})
	return body
}

func durationOrDefault(value, fallback int) time.Duration {
	if value <= 0 {
		value = fallback
	}
	return time.Duration(value) * time.Millisecond
}

func intOrDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func int64OrDefault(value, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	return value
}

var _ telemetry.SubscriptionAPI = (*TelemetrySubscriptionService)(nil)
