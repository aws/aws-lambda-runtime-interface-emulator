// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/aws-lambda-rie/internal/telemetry/internal"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
)

//go:embed schema/telemetry-subscription-schema.json
var subscriptionSchema []byte
var telemetrySchema *jsonschema.Schema

func init() {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("telemetry-subscription-schema.json", bytes.NewReader(subscriptionSchema)); err != nil {
		slog.Error("error adding telemetry schema resource", "err", err)
		panic(err)
	}

	var err error
	telemetrySchema, err = compiler.Compile("telemetry-subscription-schema.json")
	if err != nil {
		slog.Error("error compiling telemetry schema", "err", err)
		panic(err)
	}
}

type SubscriptionAPI struct {
	store                         subscriptionStore
	logsDroppedEventAPI           internal.LogsDroppedEventAPI
	telemetrySubscriptionEventAPI telemetrySubscriptionEventAPI
}

type SubscriptionRequest struct {
	SchemaVersion string                           `json:"schemaVersion"`
	Categories    []string                         `json:"types"`
	Destination   internal.SubscriptionDestination `json:"destination"`
	Buffering     *internal.BufferingConfig        `json:"buffering,omitempty"`
}

func NewSubscriptionAPI(store subscriptionStore, logsDroppedEventAPI internal.LogsDroppedEventAPI, telemetrySubscriptionEventAPI telemetrySubscriptionEventAPI) *SubscriptionAPI {
	return &SubscriptionAPI{
		store:                         store,
		logsDroppedEventAPI:           logsDroppedEventAPI,
		telemetrySubscriptionEventAPI: telemetrySubscriptionEventAPI,
	}
}

func (api *SubscriptionAPI) Subscribe(agentName string, body io.Reader, _ map[string][]string, _ string) (resp []byte, status int, respHeaders map[string][]string, err error) {

	bodyData, err := io.ReadAll(body)
	if err != nil {
		return []byte(fmt.Sprintf(`{"errorType": "ValidationError", "errorMessage": "Failed to read request body: %s"}`, err.Error())),
			http.StatusBadRequest,
			map[string][]string{},
			nil
	}

	if err := api.validateSubscriptionJSON(bodyData); err != nil {
		return []byte(fmt.Sprintf(`{"errorType": "ValidationError", "errorMessage": "%s"}`, err.Error())),
			http.StatusBadRequest,
			map[string][]string{},
			nil
	}

	var req SubscriptionRequest
	if err := json.Unmarshal(bodyData, &req); err != nil {
		return []byte(fmt.Sprintf(`{"errorType": "ValidationError", "errorMessage": "Invalid JSON: %s"}`, err.Error())),
			http.StatusBadRequest,
			map[string][]string{},
			nil
	}

	buffering := internal.BufferingConfig{
		MaxItems: 10000,
		MaxBytes: 256 * 1024,
		Timeout:  model.DurationMS(1 * time.Second),
	}

	if req.Buffering != nil {
		if req.Buffering.MaxItems > 0 {
			buffering.MaxItems = req.Buffering.MaxItems
		}
		if req.Buffering.MaxBytes > 0 {
			buffering.MaxBytes = req.Buffering.MaxBytes
		}
		if req.Buffering.Timeout > 0 {
			buffering.Timeout = req.Buffering.Timeout
		}
	}

	c, err := internal.NewClient(req.Destination)
	if err != nil {
		return []byte(fmt.Sprintf(`{"errorType": "ValidationError", "errorMessage": "Invalid destination: %s"}`, err.Error())),
			http.StatusBadRequest,
			map[string][]string{},
			nil
	}

	categories := make(map[internal.EventCategory]struct{}, len(req.Categories))
	for _, category := range req.Categories {
		categories[category] = struct{}{}
	}

	subscriber := internal.NewSubscriber(agentName, categories, buffering, c, api.logsDroppedEventAPI)

	if err := api.store.addSubscriber(subscriber); err != nil {
		return nil, 0, nil, err
	}

	slog.Info("Telemetry subscription created",
		"agentName", agentName,
		"destinationURI", req.Destination.URI,
		"destinationPort", req.Destination.Port,
		"protocol", req.Destination.Protocol,
		"categories", req.Categories)

	if err := api.telemetrySubscriptionEventAPI.sendTelemetrySubscription(agentName, "Subscribed", req.Categories); err != nil {
		slog.Error("Failed to send platform.telemetrySubscription event", "err", err)
	}

	return []byte(`"OK"`), http.StatusOK, map[string][]string{}, nil
}

func (api *SubscriptionAPI) validateSubscriptionJSON(jsonData []byte) error {
	var rawData map[string]any
	if err := json.Unmarshal(jsonData, &rawData); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	if err := telemetrySchema.Validate(rawData); err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}

	return nil
}

func (api *SubscriptionAPI) RecordCounterMetric(metricName string, count int) {}

func (api *SubscriptionAPI) FlushMetrics() interop.TelemetrySubscriptionMetrics {
	return interop.TelemetrySubscriptionMetrics{}
}

func (api *SubscriptionAPI) Clear() {
	panic("not implemented")
}

func (api *SubscriptionAPI) TurnOff() {
	api.store.disableAddSubscriber()
}

func (api *SubscriptionAPI) GetEndpointURL() string {
	panic("not implemented")
}

func (api *SubscriptionAPI) GetServiceClosedErrorMessage() string {
	return "Telemetry API subscription is closed"
}

func (api *SubscriptionAPI) GetServiceClosedErrorType() string {
	return "Telemetry.SubscriptionClosed"
}

func (api *SubscriptionAPI) Configure(passphrase string, addr netip.AddrPort) {}

type telemetrySubscriptionEventAPI interface {
	sendTelemetrySubscription(agentName, state string, categories []internal.EventCategory) error
}

type subscriptionStore interface {
	addSubscriber(subscriber sub) error
	disableAddSubscriber()
}
