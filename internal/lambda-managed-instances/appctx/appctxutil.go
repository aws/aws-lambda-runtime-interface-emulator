// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package appctx

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

type ReqCtxKey int

const ReqCtxApplicationContextKey ReqCtxKey = iota

const MaxRuntimeReleaseLength = 128

func FromRequest(request *http.Request) ApplicationContext {
	return request.Context().Value(ReqCtxApplicationContextKey).(ApplicationContext)
}

func RequestWithAppCtx(request *http.Request, appCtx ApplicationContext) *http.Request {
	return request.WithContext(context.WithValue(request.Context(), ReqCtxApplicationContextKey, appCtx))
}

func GetRuntimeRelease(appCtx ApplicationContext) string {
	return appCtx.GetOrDefault(AppCtxRuntimeReleaseKey, "").(string)
}

func GetUserAgentFromRequest(request *http.Request) string {
	runtimeRelease := ""
	userAgent := request.Header.Get("User-Agent")

	if fields := strings.Fields(userAgent); len(fields) > 0 && len(fields[0]) > 0 {
		runtimeRelease = fields[0]
	}
	return runtimeRelease
}

func CreateRuntimeReleaseFromRequest(request *http.Request, runtimeRelease string) string {
	lambdaRuntimeFeaturesHeader := request.Header.Get("Lambda-Runtime-Features")

	lambdaRuntimeFeaturesHeader = strings.ReplaceAll(lambdaRuntimeFeaturesHeader, "(", "")
	lambdaRuntimeFeaturesHeader = strings.ReplaceAll(lambdaRuntimeFeaturesHeader, ")", "")

	numberOfAppendedFeatures := 0

	runtimeReleaseLength := len(runtimeRelease)
	if runtimeReleaseLength == 0 {
		runtimeReleaseLength = len("Unknown")
	}
	availableLength := MaxRuntimeReleaseLength - runtimeReleaseLength - 3
	var lambdaRuntimeFeatures []string

	for _, feature := range strings.Fields(lambdaRuntimeFeaturesHeader) {
		featureLength := len(feature)

		if featureLength <= availableLength-numberOfAppendedFeatures {
			availableLength -= featureLength
			lambdaRuntimeFeatures = append(lambdaRuntimeFeatures, feature)
			numberOfAppendedFeatures++
		}
	}

	if len(lambdaRuntimeFeatures) > 0 {
		if runtimeRelease == "" {
			runtimeRelease = "Unknown"
		}
		runtimeRelease += " (" + strings.Join(lambdaRuntimeFeatures, " ") + ")"
	}

	return runtimeRelease
}

func UpdateAppCtxWithRuntimeRelease(request *http.Request, appCtx ApplicationContext) bool {

	if appCtxRuntimeRelease := GetRuntimeRelease(appCtx); len(appCtxRuntimeRelease) > 0 {

		if runtimeReleaseWithFeatures := CreateRuntimeReleaseFromRequest(request, appCtxRuntimeRelease); len(runtimeReleaseWithFeatures) > len(appCtxRuntimeRelease) &&
			appCtxRuntimeRelease[len(appCtxRuntimeRelease)-1] != ')' {
			appCtx.Store(AppCtxRuntimeReleaseKey, runtimeReleaseWithFeatures)
			return true
		}
		return false
	}

	if runtimeReleaseWithFeatures := CreateRuntimeReleaseFromRequest(request,
		GetUserAgentFromRequest(request)); runtimeReleaseWithFeatures != "" {
		appCtx.Store(AppCtxRuntimeReleaseKey, runtimeReleaseWithFeatures)
		return true
	}
	return false
}

func StoreInvokeErrorTraceData(appCtx ApplicationContext, invokeError *interop.InvokeErrorTraceData) {
	appCtx.Store(AppCtxInvokeErrorTraceDataKey, invokeError)
}

func LoadInvokeErrorTraceData(appCtx ApplicationContext) *interop.InvokeErrorTraceData {
	v, ok := appCtx.Load(AppCtxInvokeErrorTraceDataKey)
	if ok {
		return v.(*interop.InvokeErrorTraceData)
	}
	return nil
}

func StoreInteropServer(appCtx ApplicationContext, server interop.Server) {
	appCtx.Store(AppCtxInteropServerKey, server)
}

func LoadInteropServer(appCtx ApplicationContext) interop.Server {
	v, ok := appCtx.Load(AppCtxInteropServerKey)
	if ok {
		return v.(interop.Server)
	}
	return nil
}

func StoreResponseSender(appCtx ApplicationContext, server interop.InvokeResponseSender) {
	appCtx.Store(AppCtxResponseSenderKey, server)
}

func LoadResponseSender(appCtx ApplicationContext) interop.InvokeResponseSender {
	v, ok := appCtx.Load(AppCtxResponseSenderKey)
	if ok {
		return v.(interop.InvokeResponseSender)
	}
	return nil
}

func StoreFirstFatalError(appCtx ApplicationContext, err model.CustomerError) {
	if existing := appCtx.StoreIfNotExists(AppCtxFirstFatalErrorKey, err); existing != nil {
		slog.Warn("Omitting fatal error: already stored", "err", err, "existing", existing.(model.CustomerError))
		return
	}

	slog.Warn("First fatal error stored in appctx", "errorType", err.ErrorType())
}

func LoadFirstFatalError(appCtx ApplicationContext) (customerError model.CustomerError, found bool) {
	v, found := appCtx.Load(AppCtxFirstFatalErrorKey)
	if !found {
		return model.CustomerError{}, false
	}
	return v.(model.CustomerError), true
}
