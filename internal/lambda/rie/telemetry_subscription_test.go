// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rie

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	standalonetelemetry "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapidcore/standalone/telemetry"
)

// receiver stands in for a subscribed extension.
type receiver struct {
	server *httptest.Server
	lock   sync.Mutex
	events []map[string]interface{}
}

func newReceiver(t *testing.T) *receiver {
	t.Helper()
	r := &receiver{}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("reading delivery: %v", err)
			return
		}
		var batch []map[string]interface{}
		if err := json.Unmarshal(body, &batch); err != nil {
			t.Errorf("delivered body is not a JSON array: %v (%s)", err, body)
			return
		}
		r.lock.Lock()
		r.events = append(r.events, batch...)
		r.lock.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(r.server.Close)
	return r
}

func (r *receiver) received() []map[string]interface{} {
	r.lock.Lock()
	defer r.lock.Unlock()
	events := make([]map[string]interface{}, len(r.events))
	copy(events, r.events)
	return events
}

// waitFor polls until the receiver has at least count events, so tests don't
// depend on delivery timing.
func (r *receiver) waitFor(t *testing.T, count int) []map[string]interface{} {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if events := r.received(); len(events) >= count {
			return events
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("only %d of %d events delivered", len(r.received()), count)
	return nil
}

func subscribeBody(destination string, types []string, timeoutMs int) string {
	body, _ := json.Marshal(map[string]interface{}{
		"schemaVersion": "2022-12-13",
		"types":         types,
		"buffering":     map[string]int{"timeoutMs": timeoutMs, "maxItems": 1000, "maxBytes": 262144},
		"destination":   map[string]string{"protocol": "HTTP", "URI": destination},
	})
	return string(body)
}

func subscribe(t *testing.T, service *TelemetrySubscriptionService, agent, destination string, types []string) {
	t.Helper()
	response, status, _, err := service.Subscribe(agent, strings.NewReader(subscribeBody(destination, types, 10)), nil, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "subscribe should succeed: %s", response)
}

func platformEvent(eventType string, record map[string]interface{}) standalonetelemetry.SandboxEvent {
	return standalonetelemetry.SandboxEvent{
		Time:          "2026-07-28T12:00:00Z",
		Type:          eventType,
		PlatformEvent: record,
	}
}

func logEvent(eventType, message string) standalonetelemetry.SandboxEvent {
	return standalonetelemetry.SandboxEvent{
		Time:       "2026-07-28T12:00:00Z",
		Type:       eventType,
		LogMessage: message,
	}
}

// A subscriber receives platform events as objects and log lines as strings,
// which is the distinction the Telemetry API defines and that consumers branch on.
func TestDeliversRecordsInTheDocumentedShape(t *testing.T) {
	service := NewTelemetrySubscriptionService()
	extension := newReceiver(t)
	subscribe(t, service, "ext", extension.server.URL, []string{"platform", "function"})

	service.Dispatch(platformEvent("platform.start", map[string]interface{}{"requestId": "abc"}))
	service.Dispatch(logEvent("function", "hello from the handler"))

	// The subscription itself is reported, so expect it alongside the two events.
	events := extension.waitFor(t, 3)

	byType := map[string]map[string]interface{}{}
	for _, event := range events {
		byType[event["type"].(string)] = event
	}

	start := byType["platform.start"]
	require.NotNil(t, start, "platform.start should be delivered")
	record, ok := start["record"].(map[string]interface{})
	require.True(t, ok, "a platform record must be an object, got %T", start["record"])
	assert.Equal(t, "abc", record["requestId"])

	function := byType["function"]
	require.NotNil(t, function, "function telemetry should be delivered")
	assert.Equal(t, "hello from the handler", function["record"], "a log record must be the line itself")

	assert.NotNil(t, byType["platform.telemetrySubscription"], "the subscription should be reported")
}

// Subscribing to one category must not deliver the others, which is what lets an
// extension take function logs without receiving its own output back.
func TestTypeFilteringIsRespected(t *testing.T) {
	service := NewTelemetrySubscriptionService()
	extension := newReceiver(t)
	subscribe(t, service, "ext", extension.server.URL, []string{"function"})

	service.Dispatch(logEvent("function", "wanted"))
	service.Dispatch(logEvent("extension", "not wanted"))
	service.Dispatch(platformEvent("platform.start", map[string]interface{}{"requestId": "abc"}))

	events := extension.waitFor(t, 1)
	time.Sleep(100 * time.Millisecond) // let anything unwanted arrive if it's going to

	for _, event := range extension.received() {
		assert.Equal(t, "function", event["type"], "only function telemetry was subscribed to")
	}
	assert.Len(t, events, 1)
}

// Initialization telemetry is produced before an extension can subscribe. Real
// Lambda delivers it anyway, so events are held and replayed to a new subscriber.
func TestEventsBeforeSubscriptionAreReplayed(t *testing.T) {
	service := NewTelemetrySubscriptionService()

	service.Dispatch(platformEvent("platform.initStart", map[string]interface{}{"phase": "init"}))

	extension := newReceiver(t)
	subscribe(t, service, "ext", extension.server.URL, []string{"platform"})

	events := extension.waitFor(t, 2)
	types := map[string]bool{}
	for _, event := range events {
		types[event["type"].(string)] = true
	}
	assert.True(t, types["platform.initStart"], "initialization telemetry should survive until someone subscribes")
}

// The sandbox closes the subscription window once initialization finishes.
// Delivery to those already subscribed has to continue regardless.
func TestTurnOffStopsSubscriptionsNotDelivery(t *testing.T) {
	service := NewTelemetrySubscriptionService()
	extension := newReceiver(t)
	subscribe(t, service, "ext", extension.server.URL, []string{"platform"})

	service.TurnOff()

	service.Dispatch(platformEvent("platform.runtimeDone", map[string]interface{}{"requestId": "abc"}))
	events := extension.waitFor(t, 2)

	var delivered bool
	for _, event := range events {
		if event["type"] == "platform.runtimeDone" {
			delivered = true
		}
	}
	assert.True(t, delivered, "telemetry must keep flowing after the subscription window closes")

	_, status, _, err := service.Subscribe("late", strings.NewReader(subscribeBody(extension.server.URL, []string{"platform"}, 10)), nil, "")
	assert.Error(t, err, "a late subscription should be refused")
	assert.Equal(t, http.StatusBadRequest, status)
}

// A reset means a fresh execution environment, so nothing should carry over.
func TestClearDropsSubscriptions(t *testing.T) {
	service := NewTelemetrySubscriptionService()
	extension := newReceiver(t)
	subscribe(t, service, "ext", extension.server.URL, []string{"platform"})
	extension.waitFor(t, 1)

	service.Clear()
	before := len(extension.received())
	service.Dispatch(platformEvent("platform.start", map[string]interface{}{"requestId": "abc"}))
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, before, len(extension.received()), "a cleared service has no subscribers to deliver to")
}

// Buffering limits are honored: reaching maxItems delivers without waiting for
// the timeout, which is what keeps a busy function's telemetry moving.
func TestMaxItemsFlushesImmediately(t *testing.T) {
	service := NewTelemetrySubscriptionService()
	extension := newReceiver(t)

	body, _ := json.Marshal(map[string]interface{}{
		"schemaVersion": "2022-12-13",
		"types":         []string{"function"},
		// A long timeout, so only reaching maxItems can cause delivery.
		"buffering":   map[string]int{"timeoutMs": 60000, "maxItems": 2, "maxBytes": 262144},
		"destination": map[string]string{"protocol": "HTTP", "URI": extension.server.URL},
	})
	_, status, _, err := service.Subscribe("ext", strings.NewReader(string(body)), nil, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	service.Dispatch(logEvent("function", "one"))
	service.Dispatch(logEvent("function", "two"))

	events := extension.waitFor(t, 2)
	assert.Len(t, events, 2)
}

func TestSubscribeRejectsUnusableRequests(t *testing.T) {
	extension := newReceiver(t)

	testCases := []struct {
		name string
		body string
	}{
		{"not JSON", `{`},
		{"no types", subscribeBody(extension.server.URL, nil, 10)},
		{"unsupported protocol", `{"types":["function"],"destination":{"protocol":"TCP","URI":"http://localhost:1"}}`},
		{"unparseable destination", `{"types":["function"],"destination":{"protocol":"HTTP","URI":"http://[::1"}}`},
		{"non-http destination", `{"types":["function"],"destination":{"protocol":"HTTP","URI":"file:///tmp/x"}}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewTelemetrySubscriptionService()
			_, status, _, err := service.Subscribe("ext", strings.NewReader(tc.body), nil, "")
			assert.NoError(t, err, "a bad request is reported by status, not by error")
			assert.Equal(t, http.StatusBadRequest, status)
		})
	}
}

// Extensions are told to deliver to the "sandbox" host, which only resolves
// inside a real execution environment.
func TestSandboxHostIsRewritten(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{"http://sandbox:3000", "http://localhost:3000"},
		{"http://sandbox:3000/path", "http://localhost:3000/path"},
		{"http://sandbox", "http://localhost"},
		{"http://localhost:3000", "http://localhost:3000"},
		{"http://127.0.0.1:3000", "http://127.0.0.1:3000"},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := resolveDestination(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
