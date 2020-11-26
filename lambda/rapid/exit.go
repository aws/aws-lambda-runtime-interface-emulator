// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"fmt"
	"os"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/extensions"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi/model"

	log "github.com/sirupsen/logrus"
)

func checkInteropError(format string, err error) {
	if err == interop.ErrInvalidInvokeID || err == interop.ErrResponseSent {
		log.Warnf(format, err)
	} else {
		log.Panicf(format, err)
	}
}

func trySendDefaultErrorResponse(interopServer interop.Server, invokeID string, errorType fatalerror.ErrorType, err error) {
	resp := model.ErrorResponse{
		ErrorType:    string(errorType),
		ErrorMessage: fmt.Sprintf("Error: %v", err),
	}

	if invokeID != "" {
		resp.ErrorMessage = fmt.Sprintf("RequestId: %s Error: %v", invokeID, err)
	}

	if err := interopServer.SendErrorResponse(invokeID, resp.AsInteropError()); err != nil {
		checkInteropError("Failed to send default error response: %s", err)
	}
}

func reportErrorAndExit(appCtx appctx.ApplicationContext, execCtx *rapidContext, invokeID string, interopServer interop.Server, err error, correlationID string) {
	// This function maintains compatibility of exit sequence behaviour
	// with Sandbox Factory in the absence of extensions
	errorType, found := appctx.LoadFirstFatalError(appCtx)
	if !found {
		errorType = fatalerror.Unknown
	}

	// NOTE this check will prevent us from sending FAULT message in case
	// response (positive or negative) has already been sent. This is done
	// to maintain legacy behavior of RAPID.
	// ALSO NOTE, this works in case of positive response because this will
	// be followed by RAPID exit.
	if !interopServer.IsResponseSent() {
		trySendDefaultErrorResponse(interopServer, invokeID, errorType, err)
	}

	if err := interopServer.CommitResponse(); err != nil {
		checkInteropError("Failed to commit error response: %s", err)
	}

	doneMsg := &interop.Done{
		WaitForExit:    true,
		RuntimeRelease: appctx.GetRuntimeRelease(appCtx),
		CorrelationID:  correlationID, // required for standalone mode
	}
	if execCtx.telemetryAPIEnabled {
		doneMsg.LogsAPIMetrics = execCtx.telemetryService.FlushMetrics()
	}

	if err := interopServer.SendDone(doneMsg); err != nil {
		checkInteropError("Failed to send DONE during exit: %s", err)
	}

	os.Exit(1)
}

func reportErrorAndRequestReset(appCtx appctx.ApplicationContext, execCtx *rapidContext, invokeID string, interopServer interop.Server, err error, correlationID string) {
	errorType, found := appctx.LoadFirstFatalError(appCtx)
	if !found {
		errorType = fatalerror.Unknown
	}

	trySendDefaultErrorResponse(interopServer, invokeID, errorType, err)

	if err := interopServer.CommitResponse(); err != nil {
		checkInteropError("Failed to commit error response: %s", err)
	}

	doneFailMsg := &interop.DoneFail{
		ErrorType:           string(errorType),
		RuntimeRelease:      appctx.GetRuntimeRelease(appCtx),
		CorrelationID:       correlationID, // required for standalone mode
		NumActiveExtensions: execCtx.registrationService.CountAgents(),
	}
	if execCtx.telemetryAPIEnabled {
		doneFailMsg.LogsAPIMetrics = execCtx.telemetryService.FlushMetrics()
	}

	if err := interopServer.SendDoneFail(doneFailMsg); err != nil {
		checkInteropError("Failed to send DONEFAIL: %s", err)
	}
}

func handleError(appCtx appctx.ApplicationContext, execCtx *rapidContext, invokeID string, interopServer interop.Server, err error, correlationID string) {
	if err == errResetReceived {
		// errResetReceived is returned when execution flow was interrupted by the Reset message,
		// hence this error deserves special handling and we yield to main receive loop to handle it
		return
	}

	reportErrorAndRequestReset(appCtx, execCtx, invokeID, interopServer, err, correlationID)
}

func handleInitError(appCtx appctx.ApplicationContext, execCtx *rapidContext, invokeID string, interopServer interop.Server, err error, correlationID string) {
	if execCtx.standaloneMode {
		handleError(appCtx, execCtx, invokeID, interopServer, err, correlationID)
		return
	}

	if !execCtx.HasActiveExtensions() {
		// we don't expect Slicer to send RESET during INIT, that's why we Exit here
		reportErrorAndExit(appCtx, execCtx, invokeID, interopServer, err, correlationID)
	}

	handleError(appCtx, execCtx, invokeID, interopServer, err, correlationID)
}

func handleInvokeError(appCtx appctx.ApplicationContext, execCtx *rapidContext, invokeID string, interopServer interop.Server, err error, correlationID string) {
	if execCtx.standaloneMode {
		handleError(appCtx, execCtx, invokeID, interopServer, err, correlationID)
		return
	}

	// Invoke with extensions disabled maintains behaviour parity with pre-extensions rapid
	if !extensions.AreEnabled() {
		reportErrorAndExit(appCtx, execCtx, invokeID, interopServer, err, correlationID)
	}

	handleError(appCtx, execCtx, invokeID, interopServer, err, correlationID)
}
