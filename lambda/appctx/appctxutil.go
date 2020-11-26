// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package appctx

import (
	"context"
	"net/http"
	"strings"

	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"

	log "github.com/sirupsen/logrus"
)

// This package contains a set of utility methods for accessing application
// context and application context data.

// A ReqCtxKey type is used as a key for storing values in the request context.
type ReqCtxKey int

// ReqCtxApplicationContextKey is used for injecting application
// context object into request context.
const ReqCtxApplicationContextKey ReqCtxKey = iota

// FromRequest retrieves application context from the request context.
func FromRequest(request *http.Request) ApplicationContext {
	return request.Context().Value(ReqCtxApplicationContextKey).(ApplicationContext)
}

// RequestWithAppCtx places application context into request context.
func RequestWithAppCtx(request *http.Request, appCtx ApplicationContext) *http.Request {
	return request.WithContext(context.WithValue(request.Context(), ReqCtxApplicationContextKey, appCtx))
}

// GetRuntimeRelease returns runtime_release str extracted from app context.
func GetRuntimeRelease(appCtx ApplicationContext) string {
	return appCtx.GetOrDefault(AppCtxRuntimeReleaseKey, "").(string)
}

// UpdateAppCtxWithRuntimeRelease extracts runtime release info from user agent header and put it into appCtx.
// Sample UA:
// Mozilla/5.0 (Windows NT 6.1; Win64; x64; rv:47.0) Gecko/20100101 Firefox/47.0
func UpdateAppCtxWithRuntimeRelease(request *http.Request, appCtx ApplicationContext) bool {
	// If appCtx has runtime release value already, skip updating for consistency.
	if len(GetRuntimeRelease(appCtx)) > 0 {
		return false
	}

	userAgent := request.Header.Get("User-Agent")
	if len(userAgent) == 0 {
		return false
	}

	// Split around spaces and use only the first token.
	if fields := strings.Fields(userAgent); len(fields) > 0 && len(fields[0]) > 0 {
		appCtx.Store(AppCtxRuntimeReleaseKey,
			fields[0])
		return true
	}
	return false
}

// StoreErrorResponse stores response in the applicaton context.
func StoreErrorResponse(appCtx ApplicationContext, errorResponse *interop.ErrorResponse) {
	appCtx.Store(AppCtxInvokeErrorResponseKey, errorResponse)
}

// LoadErrorResponse retrieves response from the application context.
func LoadErrorResponse(appCtx ApplicationContext) *interop.ErrorResponse {
	v, ok := appCtx.Load(AppCtxInvokeErrorResponseKey)
	if ok {
		return v.(*interop.ErrorResponse)
	}
	return nil
}

// StoreInteropServer stores a reference to the interop server.
func StoreInteropServer(appCtx ApplicationContext, server interop.Server) {
	appCtx.Store(AppCtxInteropServerKey, server)
}

// LoadInteropServer retrieves the interop server.
func LoadInteropServer(appCtx ApplicationContext) interop.Server {
	v, ok := appCtx.Load(AppCtxInteropServerKey)
	if ok {
		return v.(interop.Server)
	}
	return nil
}

// StoreFirstFatalError stores unrecoverable error code in appctx once. This error is considered to be the rootcause of failure
func StoreFirstFatalError(appCtx ApplicationContext, err fatalerror.ErrorType) {
	if existing := appCtx.StoreIfNotExists(AppCtxFirstFatalErrorKey, err); existing != nil {
		log.Warnf("Omitting fatal error %s: %s already stored", err, existing.(fatalerror.ErrorType))
		return
	}

	log.Warnf("First fatal error stored in appctx: %s", err)
}

// LoadFirstFatalError returns stored error if found
func LoadFirstFatalError(appCtx ApplicationContext) (errorType fatalerror.ErrorType, found bool) {
	v, found := appCtx.Load(AppCtxFirstFatalErrorKey)
	if !found {
		return "", false
	}
	return v.(fatalerror.ErrorType), true
}
