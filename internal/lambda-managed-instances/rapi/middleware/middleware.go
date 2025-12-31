// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapi/rendering"
)

func AgentUniqueIdentifierHeaderValidator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentIdentifier := r.Header.Get(model.LambdaAgentIdentifier)
		if len(agentIdentifier) == 0 {
			rendering.RenderForbiddenWithTypeMsg(w, r, model.ErrAgentIdentifierMissing, "Missing Lambda-Extension-Identifier header")
			return
		}
		agentID, e := uuid.Parse(agentIdentifier)
		if e != nil {
			rendering.RenderForbiddenWithTypeMsg(w, r, model.ErrAgentIdentifierInvalid, "Invalid Lambda-Extension-Identifier")
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), model.AgentIDCtxKey, agentID))
		next.ServeHTTP(w, r)
	})
}

func AppCtxMiddleware(appCtx appctx.ApplicationContext) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			r = appctx.RequestWithAppCtx(r, appCtx)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func AccessLogMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			slog.Debug("API request", "method", r.Method, "url", r.URL, "headers", r.Header)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func RuntimeReleaseMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			appCtx := appctx.FromRequest(r)

			appctx.UpdateAppCtxWithRuntimeRelease(r, appCtx)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
