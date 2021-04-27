// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapid

import (
	"fmt"
	"os"

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

func reportErrorAndExit(doneFailMsg *interop.DoneFail, invokeID string, interopServer interop.Server, err error) {
	// This function maintains compatibility of exit sequence behaviour
	// with Sandbox Factory in the absence of extensions

	// NOTE this check will prevent us from sending FAULT message in case
	// response (positive or negative) has already been sent. This is done
	// to maintain legacy behavior of RAPID.
	// ALSO NOTE, this works in case of positive response because this will
	// be followed by RAPID exit.
	if !interopServer.IsResponseSent() {
		trySendDefaultErrorResponse(interopServer, invokeID, doneFailMsg.ErrorType, err)
	}

	if err := interopServer.CommitResponse(); err != nil {
		checkInteropError("Failed to commit error response: %s", err)
	}

	// old behavior: no DoneFails
	doneMsg := &interop.Done{
		WaitForExit:   true,
		CorrelationID: doneFailMsg.CorrelationID, // required for standalone mode
		Meta:          doneFailMsg.Meta,
	}

	if err := interopServer.SendDone(doneMsg); err != nil {
		checkInteropError("Failed to send DONE during exit: %s", err)
	}

	os.Exit(1)
}

func reportErrorAndRequestReset(doneFailMsg *interop.DoneFail, invokeID string, interopServer interop.Server, err error) {
	if err == errResetReceived {
		// errResetReceived is returned when execution flow was interrupted by the Reset message,
		// hence this error deserves special handling and we yield to main receive loop to handle it
		return
	}

	trySendDefaultErrorResponse(interopServer, invokeID, doneFailMsg.ErrorType, err)

	if err := interopServer.CommitResponse(); err != nil {
		checkInteropError("Failed to commit error response: %s", err)
	}

	if err := interopServer.SendDoneFail(doneFailMsg); err != nil {
		checkInteropError("Failed to send DONEFAIL: %s", err)
	}
}

func handleInitError(doneFailMsg *interop.DoneFail, execCtx *rapidContext, invokeID string, interopServer interop.Server, err error) {
	if execCtx.standaloneMode {
		reportErrorAndRequestReset(doneFailMsg, invokeID, interopServer, err)
		return
	}

	if !execCtx.HasActiveExtensions() {
		// we don't expect Slicer to send RESET during INIT, that's why we Exit here
		reportErrorAndExit(doneFailMsg, invokeID, interopServer, err)
	}

	reportErrorAndRequestReset(doneFailMsg, invokeID, interopServer, err)
}

func handleInvokeError(doneFailMsg *interop.DoneFail, execCtx *rapidContext, invokeID string, interopServer interop.Server, err error) {
	if execCtx.standaloneMode {
		reportErrorAndRequestReset(doneFailMsg, invokeID, interopServer, err)
		return
	}

	// Invoke with extensions disabled maintains behaviour parity with pre-extensions rapid
	if !extensions.AreEnabled() {
		reportErrorAndExit(doneFailMsg, invokeID, interopServer, err)
	}

	reportErrorAndRequestReset(doneFailMsg, invokeID, interopServer, err)
}
