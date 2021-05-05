// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package standalone

import (
	"context"
	"net/http"

	"go.amzn.com/lambda/rapidcore"
	"go.amzn.com/lambda/rapidcore/telemetry"

	"github.com/go-chi/chi"
)

func NewHTTPRouter(sandbox rapidcore.Sandbox, eventLog *telemetry.EventLog, shutdownFunc context.CancelFunc) *chi.Mux {
	ipcSrv := sandbox.InteropServer()
	r := chi.NewRouter()
	r.Use(standaloneAccessLogDecorator)
	r.Post("/2015-03-31/functions/*/invocations", func(w http.ResponseWriter, r *http.Request) { Execute(w, r, sandbox) })
	r.Post("/test/init", func(w http.ResponseWriter, r *http.Request) { InitHandler(w, r, sandbox) })
	r.Post("/test/reserve", func(w http.ResponseWriter, r *http.Request) { ReserveHandler(w, r, ipcSrv) })
	r.Post("/test/invoke", func(w http.ResponseWriter, r *http.Request) { InvokeHandler(w, r, ipcSrv) })
	r.Post("/test/waitUntilRelease", func(w http.ResponseWriter, r *http.Request) { WaitUntilReleaseHandler(w, r, ipcSrv) })
	r.Post("/test/reset", func(w http.ResponseWriter, r *http.Request) { ResetHandler(w, r, ipcSrv) })
	r.Post("/test/shutdown", func(w http.ResponseWriter, r *http.Request) { ShutdownHandler(w, r, ipcSrv, shutdownFunc) })
	r.Post("/test/directInvoke/{reservationtoken}", func(w http.ResponseWriter, r *http.Request) { DirectInvokeHandler(w, r, ipcSrv) })
	r.Get("/test/internalState", func(w http.ResponseWriter, r *http.Request) { InternalStateHandler(w, r, ipcSrv) })
	r.Get("/test/eventLog", func(w http.ResponseWriter, r *http.Request) { EventLogHandler(w, r, eventLog) })

	return r
}
