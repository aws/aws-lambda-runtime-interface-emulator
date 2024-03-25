// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"net/http"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/core"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapi/rendering"

	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"
)

const (
	StreamingFunctionResponseMode = "streaming"
)

type invocationResponseHandler struct {
	registrationService core.RegistrationService
}

func (h *invocationResponseHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	appCtx := appctx.FromRequest(request)

	server := appctx.LoadResponseSender(appCtx)
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

	invokeID := chi.URLParam(request, "awsrequestid")

	headers := map[string]string{contentTypeHeader: request.Header.Get(contentTypeHeader)}
	if functionResponseMode := request.Header.Get(functionResponseModeHeader); functionResponseMode != "" {
		switch functionResponseMode {
		case StreamingFunctionResponseMode:
			headers[functionResponseModeHeader] = functionResponseMode
		default:
			errHeaders := interop.InvokeResponseHeaders{
				ContentType: request.Header.Get(contentTypeHeader),
			}
			fnError := interop.FunctionError{Type: fatalerror.RuntimeInvalidResponseModeHeader}
			response := &interop.ErrorInvokeResponse{
				Headers:       errHeaders,
				FunctionError: fnError,
				Payload:       []byte{},
			}

			_ = server.SendErrorResponse(chi.URLParam(request, "awsrequestid"), response)
			rendering.RenderInvalidFunctionResponseMode(writer, request)
			return
		}
	}

	response := &interop.StreamableInvokeResponse{
		Headers:  headers,
		Payload:  request.Body,
		Trailers: request.Trailer,
		Request:  &interop.CancellableRequest{Request: request},
	}

	if err := server.SendResponse(invokeID, response); err != nil {
		switch err := err.(type) {
		case *interop.ErrorResponseTooLarge:
			if server.SendErrorResponse(invokeID, err.AsErrorResponse()) != nil {
				rendering.RenderInteropError(writer, request, err)
				return
			}

			appctx.StoreInvokeErrorTraceData(appCtx, &interop.InvokeErrorTraceData{})

			if err := runtime.ResponseSent(); err != nil {
				log.Panic(err)
			}

			rendering.RenderRequestEntityTooLarge(writer, request)
			return

		case *interop.ErrorResponseTooLargeDI:
			// in DirectInvoke case, the (truncated) response is already sent back to the caller
			if err := runtime.ResponseSent(); err != nil {
				log.Panic(err)
			}

			rendering.RenderRequestEntityTooLarge(writer, request)
			return

		case *interop.ErrTruncatedResponse:
			if err := runtime.ResponseSent(); err != nil {
				log.Panic(err)
			}

			rendering.RenderTruncatedHTTPRequestError(writer, request)
			return

		case *interop.ErrInternalPlatformError:
			rendering.RenderInternalServerError(writer, request)
			return

		default:
			rendering.RenderInteropError(writer, request, err)
			return
		}
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
