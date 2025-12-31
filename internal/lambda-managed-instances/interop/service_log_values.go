// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

const (
	RequestIdProperty         = "RequestId"
	MemorySizeMbProperty      = "MemorySizeMB"
	FunctionArnProperty       = "FunctionArn"
	FunctionVersionIdProperty = "FunctionVersionId"
	RuntimeVersionProperty    = "RuntimeVersion"

	ArtefactTypeDimension     = "ArtefactType"
	AvailabilityZoneDimension = "AvailabilityZoneId"
	WorkerAmiIdDimension      = "WorkerAmiId"

	TotalDurationMetric            = "TotalDuration"
	PlatformOverheadDurationMetric = "PlatformOverheadDuration"
	TotalExtensionsCountMetric     = "TotalExtensionsCount"
	InternalExtensionsCountMetric  = "InternalExtensionsCount"
	ExternalExtensionsCountMetric  = "ExternalExtensionsCount"

	ShutdownAbortInvokesDurationMetric        = "AbortInvokeDuration"
	ShutdownKillProcessDurationMetricTemplate = "Kill%sDuration"
	ShutdownRuntimeDuration                   = "StopRuntimeDuration"
	ShutdownExtensionsDuration                = "StopExtensionsDuration"
	ShutdownWaitAllProcessesDuration          = "WaitCustomerProcessesExitDuration"
	ShutdownRuntimeServerDuration             = "StopRuntimeServerDuration"

	ClientErrorMetric           = "ClientError"
	ClientErrorReasonTemplate   = "ClientErrorReason-%s"
	CustomerErrorMetric         = "CustomerError"
	CustomerErrorReasonTemplate = "CustomerErrorReason-%s"
	PlatformErrorMetric         = "PlatformError"
	PlatformErrorReasonTemplate = "PlatformErrorReason-%s"
	NonCustomerErrorMetric      = "NonCustomerError"
)
