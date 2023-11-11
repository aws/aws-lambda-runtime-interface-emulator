// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"context"
	"net/http"

	"go.amzn.com/lambda/core/statejson"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/rapidcore"
	"go.amzn.com/lambda/rapidcore/standalone/telemetry"

	"github.com/go-chi/chi"
)

type InteropServer interface {
	Init(i *interop.Init, invokeTimeoutMs int64) error
	AwaitInitialized() error
	FastInvoke(w http.ResponseWriter, i *interop.Invoke, direct bool) error
	Reserve(id string, traceID, lambdaSegmentID string) (*rapidcore.ReserveResponse, error)
	Reset(reason string, timeoutMs int64) (*statejson.ResetDescription, error)
	AwaitRelease() (*statejson.ReleaseResponse, error)
	Shutdown(shutdown *interop.Shutdown) *statejson.InternalStateDescription
	InternalState() (*statejson.InternalStateDescription, error)
	CurrentToken() *interop.Token
	Restore(restore *interop.Restore) (interop.RestoreResult, error)
}

func NewHTTPRouter(ipcSrv InteropServer, lambdaInvokeAPI rapidcore.LambdaInvokeAPI, eventsAPI *telemetry.StandaloneEventsAPI, shutdownFunc context.CancelFunc, bs interop.Bootstrap) *chi.Mux {
	r := chi.NewRouter()
	r.Use(standaloneAccessLogDecorator)

	r.Post("/2015-03-31/functions/*/invocations", func(w http.ResponseWriter, r *http.Request) { Execute(w, r, lambdaInvokeAPI) })
	r.Get("/test/ping", func(w http.ResponseWriter, r *http.Request) { PingHandler(w, r) })
	r.Post("/test/init", func(w http.ResponseWriter, r *http.Request) { InitHandler(w, r, ipcSrv, bs) })
	r.Post("/test/waitUntilInitialized", func(w http.ResponseWriter, r *http.Request) { WaitUntilInitializedHandler(w, r, ipcSrv) })
	r.Post("/test/reserve", func(w http.ResponseWriter, r *http.Request) { ReserveHandler(w, r, ipcSrv) })
	r.Post("/test/invoke", func(w http.ResponseWriter, r *http.Request) { InvokeHandler(w, r, ipcSrv) })
	r.Post("/test/waitUntilRelease", func(w http.ResponseWriter, r *http.Request) { WaitUntilReleaseHandler(w, r, ipcSrv) })
	r.Post("/test/reset", func(w http.ResponseWriter, r *http.Request) { ResetHandler(w, r, ipcSrv) })
	r.Post("/test/shutdown", func(w http.ResponseWriter, r *http.Request) { ShutdownHandler(w, r, ipcSrv, shutdownFunc) })
	r.Post("/test/directInvoke/{reservationtoken}", func(w http.ResponseWriter, r *http.Request) { DirectInvokeHandler(w, r, ipcSrv) })
	r.Get("/test/internalState", func(w http.ResponseWriter, r *http.Request) { InternalStateHandler(w, r, ipcSrv) })
	r.Get("/test/eventLog", func(w http.ResponseWriter, r *http.Request) { EventLogHandler(w, r, eventsAPI) })
	r.Post("/test/restore", func(w http.ResponseWriter, r *http.Request) { RestoreHandler(w, r, ipcSrv) })
	return r
}
