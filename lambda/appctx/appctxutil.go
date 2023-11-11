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

// MaxRuntimeReleaseLength Max length for user agent string.
const MaxRuntimeReleaseLength = 128

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

// GetUserAgentFromRequest Returns the first token -seperated by a space-
// from request header 'User-Agent'.
func GetUserAgentFromRequest(request *http.Request) string {
	runtimeRelease := ""
	userAgent := request.Header.Get("User-Agent")
	// Split around spaces and use only the first token.
	if fields := strings.Fields(userAgent); len(fields) > 0 && len(fields[0]) > 0 {
		runtimeRelease = fields[0]
	}
	return runtimeRelease
}

// CreateRuntimeReleaseFromRequest Gets runtime features from request header
// 'Lambda-Runtime-Features', and append it to the given runtime release.
func CreateRuntimeReleaseFromRequest(request *http.Request, runtimeRelease string) string {
	lambdaRuntimeFeaturesHeader := request.Header.Get("Lambda-Runtime-Features")

	// "(", ")" are not valid token characters, and potentially could invalidate runtime_release
	lambdaRuntimeFeaturesHeader = strings.ReplaceAll(lambdaRuntimeFeaturesHeader, "(", "")
	lambdaRuntimeFeaturesHeader = strings.ReplaceAll(lambdaRuntimeFeaturesHeader, ")", "")

	numberOfAppendedFeatures := 0
	// Available length is a maximum length available for runtime features (including delimiters). From maximal runtime
	// release length we subtract what we already have plus 3 additional bytes for a space and a pair of brackets for
	// list of runtime features that is added later.
	runtimeReleaseLength := len(runtimeRelease)
	if runtimeReleaseLength == 0 {
		runtimeReleaseLength = len("Unknown")
	}
	availableLength := MaxRuntimeReleaseLength - runtimeReleaseLength - 3
	var lambdaRuntimeFeatures []string

	for _, feature := range strings.Fields(lambdaRuntimeFeaturesHeader) {
		featureLength := len(feature)
		// If featureLength <= availableLength - numberOfAppendedFeatures
		// (where numberOfAppendedFeatures is equal to number of delimiters needed).
		if featureLength <= availableLength-numberOfAppendedFeatures {
			availableLength -= featureLength
			lambdaRuntimeFeatures = append(lambdaRuntimeFeatures, feature)
			numberOfAppendedFeatures++
		}
	}
	// Append valid features to runtime release.
	if len(lambdaRuntimeFeatures) > 0 {
		if runtimeRelease == "" {
			runtimeRelease = "Unknown"
		}
		runtimeRelease += " (" + strings.Join(lambdaRuntimeFeatures, " ") + ")"
	}

	return runtimeRelease
}

// UpdateAppCtxWithRuntimeRelease extracts runtime release info from user agent & lambda runtime features
// headers and update it into appCtx.
// Sample UA:
// Mozilla/5.0 (Windows NT 6.1; Win64; x64; rv:47.0) Gecko/20100101 Firefox/47.0
func UpdateAppCtxWithRuntimeRelease(request *http.Request, appCtx ApplicationContext) bool {
	// If appCtx has runtime release value already, just append the runtime features.
	if appCtxRuntimeRelease := GetRuntimeRelease(appCtx); len(appCtxRuntimeRelease) > 0 {
		// if the runtime features are not appended before append them, otherwise ignore
		if runtimeReleaseWithFeatures := CreateRuntimeReleaseFromRequest(request, appCtxRuntimeRelease); len(runtimeReleaseWithFeatures) > len(appCtxRuntimeRelease) &&
			appCtxRuntimeRelease[len(appCtxRuntimeRelease)-1] != ')' {
			appCtx.Store(AppCtxRuntimeReleaseKey, runtimeReleaseWithFeatures)
			return true
		}
		return false
	}
	// If appCtx doesn't have runtime release value, update it with user agent and runtime features.
	if runtimeReleaseWithFeatures := CreateRuntimeReleaseFromRequest(request,
		GetUserAgentFromRequest(request)); runtimeReleaseWithFeatures != "" {
		appCtx.Store(AppCtxRuntimeReleaseKey, runtimeReleaseWithFeatures)
		return true
	}
	return false
}

// StoreInvokeErrorTraceData stores invocation error x-ray cause header in the applicaton context.
func StoreInvokeErrorTraceData(appCtx ApplicationContext, invokeError *interop.InvokeErrorTraceData) {
	appCtx.Store(AppCtxInvokeErrorTraceDataKey, invokeError)
}

// LoadInvokeErrorTraceData retrieves invocation error x-ray cause header from the application context.
func LoadInvokeErrorTraceData(appCtx ApplicationContext) *interop.InvokeErrorTraceData {
	v, ok := appCtx.Load(AppCtxInvokeErrorTraceDataKey)
	if ok {
		return v.(*interop.InvokeErrorTraceData)
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

// StoreResponseSender stores a reference to the response sender
func StoreResponseSender(appCtx ApplicationContext, server interop.InvokeResponseSender) {
	appCtx.Store(AppCtxResponseSenderKey, server)
}

// LoadResponseSender retrieves the response sender
func LoadResponseSender(appCtx ApplicationContext) interop.InvokeResponseSender {
	v, ok := appCtx.Load(AppCtxResponseSenderKey)
	if ok {
		return v.(interop.InvokeResponseSender)
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

func StoreInitType(appCtx ApplicationContext, initCachingEnabled bool) {
	if initCachingEnabled {
		appCtx.Store(AppCtxInitType, InitCaching)
	} else {
		appCtx.Store(AppCtxInitType, Init)
	}
}

// Default Init Type is Init unless it's explicitly stored in ApplicationContext
func LoadInitType(appCtx ApplicationContext) InitType {
	return appCtx.GetOrDefault(AppCtxInitType, Init).(InitType)
}

func StoreSandboxType(appCtx ApplicationContext, sandboxType interop.SandboxType) {
	appCtx.Store(AppCtxSandboxType, sandboxType)
}

func LoadSandboxType(appCtx ApplicationContext) interop.SandboxType {
	return appCtx.GetOrDefault(AppCtxSandboxType, interop.SandboxClassic).(interop.SandboxType)
}
