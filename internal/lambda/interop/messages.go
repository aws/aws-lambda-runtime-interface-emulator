// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

// conversion from internal data structure into well defined messages

func DoneFromInvokeSuccess(successMsg InvokeSuccess) *Done {
	return &Done{
		Meta: DoneMetadata{
			RuntimeRelease:          successMsg.RuntimeRelease,
			NumActiveExtensions:     successMsg.NumActiveExtensions,
			ExtensionNames:          successMsg.ExtensionNames,
			InvokeRequestReadTimeNs: successMsg.InvokeMetrics.InvokeRequestReadTimeNs,
			InvokeRequestSizeBytes:  successMsg.InvokeMetrics.InvokeRequestSizeBytes,
			RuntimeReadyTime:        successMsg.InvokeMetrics.RuntimeReadyTime,

			InvokeCompletionTimeNs:       successMsg.InvokeCompletionTimeNs,
			InvokeReceivedTime:           successMsg.InvokeReceivedTime,
			RuntimeResponseLatencyMs:     successMsg.ResponseMetrics.RuntimeResponseLatencyMs,
			RuntimeTimeThrottledMs:       successMsg.ResponseMetrics.RuntimeTimeThrottledMs,
			RuntimeProducedBytes:         successMsg.ResponseMetrics.RuntimeProducedBytes,
			RuntimeOutboundThroughputBps: successMsg.ResponseMetrics.RuntimeOutboundThroughputBps,
			LogsAPIMetrics:               successMsg.LogsAPIMetrics,
			MetricsDimensions: DoneMetadataMetricsDimensions{
				InvokeResponseMode: successMsg.InvokeResponseMode,
			},
		},
	}
}

func DoneFailFromInvokeFailure(failureMsg *InvokeFailure) *DoneFail {
	return &DoneFail{
		ErrorType: failureMsg.ErrorType,
		Meta: DoneMetadata{
			RuntimeRelease:      failureMsg.RuntimeRelease,
			NumActiveExtensions: failureMsg.NumActiveExtensions,
			InvokeReceivedTime:  failureMsg.InvokeReceivedTime,

			RuntimeResponseLatencyMs:     failureMsg.ResponseMetrics.RuntimeResponseLatencyMs,
			RuntimeTimeThrottledMs:       failureMsg.ResponseMetrics.RuntimeTimeThrottledMs,
			RuntimeProducedBytes:         failureMsg.ResponseMetrics.RuntimeProducedBytes,
			RuntimeOutboundThroughputBps: failureMsg.ResponseMetrics.RuntimeOutboundThroughputBps,

			InvokeRequestReadTimeNs: failureMsg.InvokeMetrics.InvokeRequestReadTimeNs,
			InvokeRequestSizeBytes:  failureMsg.InvokeMetrics.InvokeRequestSizeBytes,
			RuntimeReadyTime:        failureMsg.InvokeMetrics.RuntimeReadyTime,

			ExtensionNames: failureMsg.ExtensionNames,
			LogsAPIMetrics: failureMsg.LogsAPIMetrics,

			MetricsDimensions: DoneMetadataMetricsDimensions{
				InvokeResponseMode: failureMsg.InvokeResponseMode,
			},
		},
	}
}

func DoneFailFromInitFailure(initFailure *InitFailure) *DoneFail {
	return &DoneFail{
		ErrorType: initFailure.ErrorType,
		Meta: DoneMetadata{
			RuntimeRelease:      initFailure.RuntimeRelease,
			NumActiveExtensions: initFailure.NumActiveExtensions,
			LogsAPIMetrics:      initFailure.LogsAPIMetrics,
		},
	}
}
