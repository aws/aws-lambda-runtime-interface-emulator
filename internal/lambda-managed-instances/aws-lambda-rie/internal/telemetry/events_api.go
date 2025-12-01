// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/aws-lambda-rie/internal/telemetry/internal"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/telemetry"
)

type EventsAPI struct {
	eventRelay relay
	stdout     io.Writer
}

func NewEventsAPI(eventRelay relay) *EventsAPI {
	return &EventsAPI{
		eventRelay: eventRelay,
		stdout:     os.Stdout,
	}
}

func (api *EventsAPI) SendInitStart(data interop.InitStartData) error {
	api.eventRelay.broadcast(data, internal.CategoryPlatform, internal.TypePlatformInitStart)
	return nil
}

func (api *EventsAPI) SendInitRuntimeDone(data interop.InitRuntimeDoneData) error {
	api.eventRelay.broadcast(data, internal.CategoryPlatform, internal.TypePlatformInitRuntimeDone)
	return nil
}

func (api *EventsAPI) SendInitReport(data interop.InitReportData) error {
	api.eventRelay.broadcast(data, internal.CategoryPlatform, internal.TypePlatformInitReport)
	return nil
}

func (api *EventsAPI) SendExtensionInit(data interop.ExtensionInitData) error {
	api.eventRelay.broadcast(data, internal.CategoryPlatform, internal.TypePlatformExtension)
	return nil
}

func (api *EventsAPI) SendImageError(errLog interop.ImageErrorLogData) {
	slog.Error(telemetry.FormatImageError(errLog))
}

func (api *EventsAPI) SendInvokeStart(data interop.InvokeStartData) error {
	_, _ = fmt.Fprintf(api.stdout, "START RequestId: %s\tVersion: %s\n", data.InvokeID, data.Version)

	api.eventRelay.broadcast(data, internal.CategoryPlatform, internal.TypePlatformStart)
	return nil
}

func (api *EventsAPI) SendInternalXRayErrorCause(data interop.InternalXRayErrorCauseData) error {
	return nil
}

func (api *EventsAPI) SendReport(data interop.ReportData) error {
	_, _ = fmt.Fprintf(api.stdout, "END RequestId: %s\n", data.InvokeID)
	_, _ = fmt.Fprintf(api.stdout, "REPORT RequestId: %s\tDuration: %.2f ms\n", data.InvokeID, float64(data.Metrics.DurationMs))

	api.eventRelay.broadcast(data, internal.CategoryPlatform, internal.TypePlatformReport)
	return nil
}

func (api *EventsAPI) SendPlatformLogsDropped(droppedBytes, droppedRecords int, reason string) error {
	record := map[string]any{
		"droppedBytes":   droppedBytes,
		"droppedRecords": droppedRecords,
		"reason":         reason,
	}

	api.eventRelay.broadcast(record, internal.CategoryPlatform, internal.TypePlatformLogsDropped)
	return nil
}

func (api *EventsAPI) sendTelemetrySubscription(agentName, state string, types []internal.EventCategory) error {
	record := map[string]any{
		"name":  agentName,
		"state": state,
		"types": types,
	}

	api.eventRelay.broadcast(record, internal.CategoryPlatform, internal.TypePlatformTelemetrySubscription)
	return nil
}

func (api *EventsAPI) Flush() {

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	api.eventRelay.flush(ctx)
}
