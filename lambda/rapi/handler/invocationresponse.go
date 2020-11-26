// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi/model"

	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/rapi/rendering"

	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"
)

const (
	functionResponseSizeTooLargeType = "Function.ResponseSizeTooLarge"
)

type invocationResponseHandler struct {
	registrationService core.RegistrationService
}

func readBody(request *http.Request) ([]byte, error) {
	size := request.ContentLength
	if size < 1 {
		return ioutil.ReadAll(request.Body)
	}
	buffer := make([]byte, size)
	_, err := io.ReadFull(request.Body, buffer)
	return buffer, err
}

func (h *invocationResponseHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	appCtx := appctx.FromRequest(request)

	server := appctx.LoadInteropServer(appCtx)
	if server == nil {
		log.Panic("Invalid state, cannot access interop server")
	}

	runtime := h.registrationService.GetRuntime()
	if err := runtime.InvocationResponse(); err != nil {
		log.Warn(err)
		rendering.RenderForbiddenWithTypeMsg(writer, request, rendering.ErrorTypeInvalidStateTransition, StateTransitionFailedForRuntimeMessageFormat,
			runtime.GetState().Name(), core.RuntimeInvocationResponseStateName, err)
		return
	}

	data, err := readBody(request)
	if err != nil {
		log.Error(err)
		rendering.RenderInternalServerError(writer, request)
		return
	}

	if len(data) > interop.MaxPayloadSize {
		log.Warn("Request entity too large")

		resp := model.ErrorResponse{
			ErrorType:    functionResponseSizeTooLargeType,
			ErrorMessage: fmt.Sprintf("Response payload size (%d bytes) exceeded maximum allowed payload size (%d bytes).", len(data), interop.MaxPayloadSize),
		}

		if server.SendErrorResponse(chi.URLParam(request, "awsrequestid"), resp.AsInteropError()) != nil {
			rendering.RenderInteropError(writer, request, err)
			return
		}

		appctx.StoreErrorResponse(appCtx, resp.AsInteropError())

		if err := runtime.ResponseSent(); err != nil {
			log.Panic(err)
		}

		rendering.RenderRequestEntityTooLarge(writer, request)
		return
	}

	response := &interop.Response{
		Payload: data,
	}

	if err := server.SendResponse(chi.URLParam(request, "awsrequestid"), response); err != nil {
		rendering.RenderInteropError(writer, request, err)
		return
	}

	if err := runtime.ResponseSent(); err != nil {
		log.Panic(err)
	}

	rendering.RenderAccepted(writer, request)
}

// NewInvocationResponseHandler returns a new instance of http handler
// for serving /runtime/invocation/{awsrequestid}/response.
func NewInvocationResponseHandler(registrationService core.RegistrationService) http.Handler {
	return &invocationResponseHandler{
		registrationService: registrationService,
	}
}
