// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

// String values of possibles agent states
const (
	AgentStartedStateName        = "Started"
	AgentRegisteredStateName     = "Registered"
	AgentReadyStateName          = "Ready"
	AgentRunningStateName        = "Running"
	AgentInitErrorStateName      = "InitError"
	AgentExitErrorStateName      = "ExitError"
	AgentShutdownFailedStateName = "ShutdownFailed"
	AgentExitedStateName         = "Exited"
	AgentLaunchErrorName         = "LaunchError"
)
