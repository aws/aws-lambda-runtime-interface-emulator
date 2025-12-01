// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"

type Protocol = string

const (
	protocolTCP  Protocol = "TCP"
	protocolHTTP Protocol = "HTTP"
)

type SubscriptionDestination struct {
	Protocol Protocol `json:"protocol"`
	URI      string   `json:"URI,omitempty"`
	Port     uint16   `json:"port,omitempty"`
}

type BufferingConfig struct {
	MaxItems int              `json:"maxItems"`
	MaxBytes int              `json:"maxBytes"`
	Timeout  model.DurationMS `json:"timeoutMs"`
}

type EventCategory = string

const (
	CategoryPlatform  EventCategory = "platform"
	CategoryFunction  EventCategory = "function"
	CategoryExtension EventCategory = "extension"
)

type EventType = string

const (
	TypePlatformInitStart             EventType = "platform.initStart"
	TypePlatformInitRuntimeDone       EventType = "platform.initRuntimeDone"
	TypePlatformExtension             EventType = "platform.extension"
	TypePlatformInitReport            EventType = "platform.initReport"
	TypePlatformStart                 EventType = "platform.start"
	TypePlatformRuntimeDone           EventType = "platform.runtimeDone"
	TypePlatformReport                EventType = "platform.report"
	TypePlatformTelemetrySubscription EventType = "platform.telemetrySubscription"
	TypePlatformLogsDropped           EventType = "platform.logsDropped"
	TypeFunction                      EventType = CategoryFunction
	TypeExtension                     EventType = CategoryExtension
)
