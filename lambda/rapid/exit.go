// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"time"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/extensions"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/telemetry"

	log "github.com/sirupsen/logrus"
)

func handleInvokeError(execCtx *rapidContext, invokeRequest *interop.Invoke, invokeMx *invokeMetrics, err error) *interop.InvokeFailure {
	invokeFailure := newInvokeFailureMsg(execCtx, invokeRequest, invokeMx, err)

	// This is the default error response that gets sent back as the function response in failure cases
	invokeFailure.DefaultErrorResponse = interop.GetErrorResponseWithFormattedErrorMessage(invokeFailure.ErrorType, invokeFailure.ErrorMessage, invokeRequest.ID)

	// Invoke with extensions disabled maintains behaviour parity with pre-extensions rapid
	if !extensions.AreEnabled() {
		invokeFailure.RequestReset = false
		return invokeFailure
	}

	if err == errResetReceived {
		// errResetReceived is returned when execution flow was interrupted by the Reset message,
		// hence this error deserves special handling and we yield to main receive loop to handle it
		invokeFailure.ResetReceived = true
		return invokeFailure
	}

	invokeFailure.RequestReset = true
	return invokeFailure
}

func newInvokeFailureMsg(execCtx *rapidContext, invokeRequest *interop.Invoke, invokeMx *invokeMetrics, err error) *interop.InvokeFailure {
	errorType, found := appctx.LoadFirstFatalError(execCtx.appCtx)
	if !found {
		errorType = fatalerror.SandboxFailure
	}

	invokeFailure := &interop.InvokeFailure{
		ErrorType:           errorType,
		ErrorMessage:        err,
		RequestReset:        true,
		ResetReceived:       false,
		RuntimeRelease:      appctx.GetRuntimeRelease(execCtx.appCtx),
		NumActiveExtensions: execCtx.registrationService.CountAgents(),
		InvokeReceivedTime:  invokeRequest.InvokeReceivedTime,
	}

	if invokeRequest.InvokeResponseMetrics != nil && interop.IsResponseStreamingMetrics(invokeRequest.InvokeResponseMetrics) {
		invokeFailure.ResponseMetrics.RuntimeResponseLatencyMs = telemetry.CalculateDuration(execCtx.RuntimeStartedTime, invokeRequest.InvokeResponseMetrics.StartReadingResponseMonoTimeMs)
		invokeFailure.ResponseMetrics.RuntimeTimeThrottledMs = invokeRequest.InvokeResponseMetrics.TimeShapedNs / int64(time.Millisecond)
		invokeFailure.ResponseMetrics.RuntimeProducedBytes = invokeRequest.InvokeResponseMetrics.ProducedBytes
		invokeFailure.ResponseMetrics.RuntimeOutboundThroughputBps = invokeRequest.InvokeResponseMetrics.OutboundThroughputBps
	}

	if invokeMx != nil {
		invokeFailure.InvokeMetrics.InvokeRequestReadTimeNs = invokeMx.rendererMetrics.ReadTime.Nanoseconds()
		invokeFailure.InvokeMetrics.InvokeRequestSizeBytes = int64(invokeMx.rendererMetrics.SizeBytes)
		invokeFailure.InvokeMetrics.RuntimeReadyTime = int64(invokeMx.runtimeReadyTime)
		invokeFailure.ExtensionNames = execCtx.GetExtensionNames()
	}

	if execCtx.telemetryAPIEnabled {
		invokeFailure.LogsAPIMetrics = interop.MergeSubscriptionMetrics(execCtx.logsSubscriptionAPI.FlushMetrics(), execCtx.telemetrySubscriptionAPI.FlushMetrics())
	}

	invokeFailure.InvokeResponseMode = invokeRequest.InvokeResponseMode

	return invokeFailure
}

func generateInitFailureMsg(execCtx *rapidContext, err error) interop.InitFailure {
	errorType, found := appctx.LoadFirstFatalError(execCtx.appCtx)
	if !found {
		errorType = fatalerror.SandboxFailure
	}

	initFailureMsg := interop.InitFailure{
		RequestReset:        true,
		ErrorType:           errorType,
		ErrorMessage:        err,
		RuntimeRelease:      appctx.GetRuntimeRelease(execCtx.appCtx),
		NumActiveExtensions: execCtx.registrationService.CountAgents(),
		Ack:                 make(chan struct{}),
	}

	if execCtx.telemetryAPIEnabled {
		initFailureMsg.LogsAPIMetrics = interop.MergeSubscriptionMetrics(execCtx.logsSubscriptionAPI.FlushMetrics(), execCtx.telemetrySubscriptionAPI.FlushMetrics())
	}

	return initFailureMsg
}

func handleInitError(execCtx *rapidContext, invokeID string, err error, initFailureResponse chan<- interop.InitFailure) {
	log.WithError(err).WithField("InvokeID", invokeID).Error("Init failed")
	initFailureMsg := generateInitFailureMsg(execCtx, err)

	if err == errResetReceived {
		// errResetReceived is returned when execution flow was interrupted by the Reset message,
		// hence this error deserves special handling and we yield to main receive loop to handle it
		initFailureMsg.ResetReceived = true
		initFailureResponse <- initFailureMsg
		<-initFailureMsg.Ack
		return
	}

	if !execCtx.HasActiveExtensions() && !execCtx.standaloneMode {
		// different behaviour when no extensions are present,
		// for compatibility with previous implementations
		initFailureMsg.RequestReset = false
	} else {
		initFailureMsg.RequestReset = true
	}

	initFailureResponse <- initFailureMsg
	<-initFailureMsg.Ack
}
