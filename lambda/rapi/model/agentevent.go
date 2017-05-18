// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

// AgentEvent is one of INVOKE, SHUTDOWN agent events
type AgentEvent struct {
	EventType  string `json:"eventType"`
	DeadlineMs int64  `json:"deadlineMs"`
}

// AgentInvokeEvent is the response to agent's get next request
type AgentInvokeEvent struct {
	*AgentEvent
	RequestID          string  `json:"requestId"`
	InvokedFunctionArn string  `json:"invokedFunctionArn"`
	Tracing            Tracing `json:"tracing"`
}

// AgentShutdownEvent is the response to agent's get next request
type AgentShutdownEvent struct {
	*AgentEvent
	ShutdownReason string `json:"shutdownReason"`
}
