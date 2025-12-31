// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi"

	rieinvoke "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/aws-lambda-rie/internal/invoke"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/invoke"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
	intmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid"
	rapidmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func NewHTTPHandler(raptorApp raptorApp, initMsg intmodel.InitRequestMessage) *HTTPHandler {
	h := &HTTPHandler{
		app:     raptorApp,
		initMsg: initMsg,
	}

	h.initOnceValue = sync.OnceValue(func() rapidmodel.AppError {
		initCtx, cancel := context.WithTimeout(context.Background(), time.Duration(h.initMsg.InitTimeout))
		defer cancel()

		dummyInitMetrics := rapid.NewInitMetrics(nil)
		res := h.app.Init(initCtx, &h.initMsg, dummyInitMetrics)
		return res
	})

	router := chi.NewRouter()
	router.Post("/2015-03-31/functions/function/invocations", h.invoke)
	h.router = router

	return h
}

type HTTPHandler struct {
	router        *chi.Mux
	app           raptorApp
	initOnceValue func() rapidmodel.AppError
	initMsg       intmodel.InitRequestMessage
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

func (h *HTTPHandler) invoke(w http.ResponseWriter, r *http.Request) {
	if err := h.initOnceValue(); err != nil {
		h.respondWithError(w, err)
		return
	}

	invokeReq := rieinvoke.NewRieInvokeRequest(r, w)
	ctx := logging.WithInvokeID(r.Context(), invokeReq.InvokeID())

	metrics := invoke.NewInvokeMetrics(nil, &noOpCounter{})
	metrics.AttachInvokeRequest(invokeReq)
	if err, responseSent := h.app.Invoke(ctx, invokeReq, metrics); err != nil {
		logging.Err(ctx, "invoke failed", err)
		if !responseSent {
			h.respondWithError(w, err)
		}
	}
}

func (h *HTTPHandler) Init() rapidmodel.AppError {
	return h.initOnceValue()
}

func (h *HTTPHandler) respondWithError(w http.ResponseWriter, err rapidmodel.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Error-Type", string(err.ErrorType()))
	w.WriteHeader(err.ReturnCode())

	if _, encodeErr := w.Write([]byte(err.ErrorDetails())); encodeErr != nil {
		slog.Error("could not encode error response", "err", encodeErr)
	}
}

type raptorApp interface {
	Init(ctx context.Context, req *intmodel.InitRequestMessage, metrics interop.InitMetrics) rapidmodel.AppError
	Invoke(ctx context.Context, msg interop.InvokeRequest, metrics interop.InvokeMetrics) (err rapidmodel.AppError, responseSent bool)
}

type noOpCounter struct{}

func (c *noOpCounter) AddInvoke(_ uint64) {}
