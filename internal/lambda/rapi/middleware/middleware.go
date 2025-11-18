// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"net/http"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/extensions"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapi/handler"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapi/rendering"
	"github.com/google/uuid"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/appctx"
	"github.com/go-chi/chi/v5"

	log "github.com/sirupsen/logrus"
)

// AwsRequestIDValidator validates that {awsrequestid} parameter
// is present in the URL and matches to the currently active id.
func AwsRequestIDValidator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appCtx := appctx.FromRequest(r)
		interopServer := appctx.LoadInteropServer(appCtx)

		if interopServer == nil {
			log.Panic("Invalid state, cannot access interop server")
		}

		invokeID := chi.URLParam(r, "awsrequestid")
		if invokeID == "" || invokeID != interopServer.GetCurrentInvokeID() {
			rendering.RenderInvalidRequestID(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AgentUniqueIdentifierHeaderValidator validates that the request contains a valid agent unique identifier in the headers
func AgentUniqueIdentifierHeaderValidator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentIdentifier := r.Header.Get(handler.LambdaAgentIdentifier)
		if len(agentIdentifier) == 0 {
			rendering.RenderForbiddenWithTypeMsg(w, r, handler.ErrAgentIdentifierMissing, "Missing Lambda-Extension-Identifier header")
			return
		}
		agentID, e := uuid.Parse(agentIdentifier)
		if e != nil {
			rendering.RenderForbiddenWithTypeMsg(w, r, handler.ErrAgentIdentifierInvalid, "Invalid Lambda-Extension-Identifier")
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), handler.AgentIDCtxKey, agentID))
		next.ServeHTTP(w, r)
	})
}

// AppCtxMiddleware injects application context into request context.
func AppCtxMiddleware(appCtx appctx.ApplicationContext) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			r = appctx.RequestWithAppCtx(r, appCtx)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

// AccessLogMiddleware writes api access log.
func AccessLogMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			log.Debug("API request - ", r.Method, " ", r.URL, ", Headers:", r.Header)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func AllowIfExtensionsEnabled(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !extensions.AreEnabled() {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RuntimeReleaseMiddleware places runtime_release into app context.
func RuntimeReleaseMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			appCtx := appctx.FromRequest(r)
			// Place runtime_release into app context.
			appctx.UpdateAppCtxWithRuntimeRelease(r, appCtx)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
