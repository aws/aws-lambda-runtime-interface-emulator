// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

type EventLog struct {
	Events []SandboxEvent `json:"events,omitempty"` // populated by the StandaloneEventLog object
	Traces []TracingEvent `json:"traces,omitempty"`
}

func NewEventLog() *EventLog {
	return &EventLog{}
}
