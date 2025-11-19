// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

// String values of possibles runtime states
const (
	RuntimeStartedStateName   = "Started"
	RuntimeInitErrorStateName = "InitError"
	RuntimeReadyStateName     = "Ready"
	RuntimeRunningStateName   = "Running"
	// RuntimeStartedState -> RuntimeRestoreReadyState
	RuntimeRestoreReadyStateName = "RestoreReady"
	// RuntimeRestoreReadyState -> RuntimeRestoringState
	RuntimeRestoringStateName               = "Restoring"
	RuntimeInvocationResponseStateName      = "InvocationResponse"
	RuntimeInvocationErrorResponseStateName = "InvocationErrorResponse"
	RuntimeResponseSentStateName            = "RuntimeResponseSentState"
	RuntimeRestoreErrorStateName            = "RuntimeRestoreErrorState"
)
