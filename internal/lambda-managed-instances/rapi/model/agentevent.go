// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

type AgentEvent struct {
	EventType  string `json:"eventType"`
	DeadlineMs int64  `json:"deadlineMs"`
}

type AgentInvokeEvent struct {
	*AgentEvent
	RequestID          string   `json:"requestId"`
	InvokedFunctionArn string   `json:"invokedFunctionArn"`
	Tracing            *Tracing `json:"tracing,omitempty"`
}

type ShutdownReason string

const (
	Spindown ShutdownReason = "spindown"
	Failure  ShutdownReason = "failure"
)

type AgentShutdownEvent struct {
	*AgentEvent
	ShutdownReason ShutdownReason `json:"shutdownReason"`
}
