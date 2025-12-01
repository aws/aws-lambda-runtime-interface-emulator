// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package appctx

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func runTestRequestWithUserAgent(t *testing.T, userAgent string, expectedRuntimeRelease string) {

	req := httptest.NewRequest("", "/", nil)
	req.Header.Set("User-Agent", userAgent)
	request := RequestWithAppCtx(req, NewApplicationContext())
	appCtx := request.Context().Value(ReqCtxApplicationContextKey).(ApplicationContext)

	ok := UpdateAppCtxWithRuntimeRelease(request, appCtx)

	assert.True(t, ok)
	ctxRuntimeRelease, ok := appCtx.Load(AppCtxRuntimeReleaseKey)
	assert.True(t, ok)
	assert.Equal(t, expectedRuntimeRelease, ctxRuntimeRelease, "failed to extract runtime_release token")
}

func TestCreateRuntimeReleaseFromRequest(t *testing.T) {
	tests := map[string]struct {
		userAgentHeader             string
		lambdaRuntimeFeaturesHeader string
		expectedRuntimeRelease      string
	}{
		"No User-Agent header": {
			userAgentHeader:             "",
			lambdaRuntimeFeaturesHeader: "httpcl/2.0 execwr",
			expectedRuntimeRelease:      "Unknown (httpcl/2.0 execwr)",
		},
		"No Lambda-Runtime-Features header": {
			userAgentHeader:             "Node.js/14.16.0",
			lambdaRuntimeFeaturesHeader: "",
			expectedRuntimeRelease:      "Node.js/14.16.0",
		},
		"Lambda-Runtime-Features header with additional spaces": {
			userAgentHeader:             "Node.js/14.16.0",
			lambdaRuntimeFeaturesHeader: "httpcl/2.0    execwr",
			expectedRuntimeRelease:      "Node.js/14.16.0 (httpcl/2.0 execwr)",
		},
		"Lambda-Runtime-Features header with special characters": {
			userAgentHeader:             "Node.js/14.16.0",
			lambdaRuntimeFeaturesHeader: "httpcl/2.0@execwr-1 abcd?efg nodewr/(4.33)) nodewr/4.3",
			expectedRuntimeRelease:      "Node.js/14.16.0 (httpcl/2.0@execwr-1 abcd?efg nodewr/4.33 nodewr/4.3)",
		},
		"Lambda-Runtime-Features header with long Lambda-Runtime-Features header": {
			userAgentHeader:             "Node.js/14.16.0",
			lambdaRuntimeFeaturesHeader: strings.Repeat("abcdef ", MaxRuntimeReleaseLength/7),
			expectedRuntimeRelease:      "Node.js/14.16.0 (" + strings.Repeat("abcdef ", (MaxRuntimeReleaseLength-18-6)/7) + "abcdef)",
		},
		"Lambda-Runtime-Features header with long Lambda-Runtime-Features header with UTF-8 characters": {
			userAgentHeader:             "Node.js/14.16.0",
			lambdaRuntimeFeaturesHeader: strings.Repeat("我爱亚马逊 ", MaxRuntimeReleaseLength/16),
			expectedRuntimeRelease:      "Node.js/14.16.0 (" + strings.Repeat("我爱亚马逊 ", (MaxRuntimeReleaseLength-18-15)/16) + "我爱亚马逊)",
		},
	}

	for _, tc := range tests {
		req := httptest.NewRequest("", "/", nil)
		if tc.userAgentHeader != "" {
			req.Header.Set("User-Agent", tc.userAgentHeader)
		}
		if tc.lambdaRuntimeFeaturesHeader != "" {
			req.Header.Set("Lambda-Runtime-Features", tc.lambdaRuntimeFeaturesHeader)
		}
		appCtx := NewApplicationContext()
		request := RequestWithAppCtx(req, appCtx)

		UpdateAppCtxWithRuntimeRelease(request, appCtx)
		runtimeRelease := GetRuntimeRelease(appCtx)

		assert.LessOrEqual(t, len(runtimeRelease), MaxRuntimeReleaseLength)
		assert.Equal(t, tc.expectedRuntimeRelease, runtimeRelease)
	}
}

func TestUpdateAppCtxWithRuntimeRelease(t *testing.T) {
	type pair struct {
		in, wanted string
	}
	pairs := []pair{
		{"Mozilla/5.0", "Mozilla/5.0"},
		{"Mozilla/6.0 (Windows NT 6.1; Win64; x64; rv:47.0) Gecko/20100101 Firefox/47.0", "Mozilla/6.0"},
	}
	for _, p := range pairs {
		runTestRequestWithUserAgent(t, p.in, p.wanted)
	}
}

func TestUpdateAppCtxWithRuntimeReleaseWithoutUserAgent(t *testing.T) {

	request := RequestWithAppCtx(httptest.NewRequest("", "/", nil), NewApplicationContext())
	appCtx := request.Context().Value(ReqCtxApplicationContextKey).(ApplicationContext)

	ok := UpdateAppCtxWithRuntimeRelease(request, appCtx)

	assert.False(t, ok)
	_, ok = appCtx.Load(AppCtxRuntimeReleaseKey)
	assert.False(t, ok)
}

func TestUpdateAppCtxWithRuntimeReleaseWithBlankUserAgent(t *testing.T) {

	req := httptest.NewRequest("", "/", nil)
	req.Header.Set("User-Agent", "        ")
	request := RequestWithAppCtx(req, NewApplicationContext())
	appCtx := request.Context().Value(ReqCtxApplicationContextKey).(ApplicationContext)

	ok := UpdateAppCtxWithRuntimeRelease(request, appCtx)

	assert.False(t, ok)
	_, ok = appCtx.Load(AppCtxRuntimeReleaseKey)
	assert.False(t, ok)
}

func TestUpdateAppCtxWithRuntimeReleaseWithLambdaRuntimeFeatures(t *testing.T) {

	req := httptest.NewRequest("", "/", nil)
	req.Header.Set("User-Agent", "Node.js/14.16.0")
	req.Header.Set("Lambda-Runtime-Features", "httpcl/2.0 execwr nodewr/4.3")
	request := RequestWithAppCtx(req, NewApplicationContext())
	appCtx := request.Context().Value(ReqCtxApplicationContextKey).(ApplicationContext)

	ok := UpdateAppCtxWithRuntimeRelease(request, appCtx)

	assert.True(t, ok, "runtime_release updated based only on User-Agent and valid features")
	ctxRuntimeRelease, ok := appCtx.Load(AppCtxRuntimeReleaseKey)
	assert.True(t, ok)
	assert.Equal(t, "Node.js/14.16.0 (httpcl/2.0 execwr nodewr/4.3)", ctxRuntimeRelease)
}

func TestUpdateAppCtxWithRuntimeReleaseMultipleTimes(t *testing.T) {

	firstValue := "Value1"
	secondValue := "Value2"

	req := httptest.NewRequest("", "/", nil)
	req.Header.Set("User-Agent", firstValue)
	request := RequestWithAppCtx(req, NewApplicationContext())
	appCtx := request.Context().Value(ReqCtxApplicationContextKey).(ApplicationContext)

	ok := UpdateAppCtxWithRuntimeRelease(request, appCtx)

	assert.True(t, ok)
	ctxRuntimeRelease, ok := appCtx.Load(AppCtxRuntimeReleaseKey)
	assert.True(t, ok)
	assert.Equal(t, firstValue, ctxRuntimeRelease)

	req.Header.Set("User-Agent", secondValue)

	ok = UpdateAppCtxWithRuntimeRelease(request, appCtx)

	assert.False(t, ok, "failed to prevent second update of runtime_release")
	ctxRuntimeRelease, ok = appCtx.Load(AppCtxRuntimeReleaseKey)
	assert.True(t, ok)
	assert.Equal(t, firstValue, ctxRuntimeRelease, "failed to prevent second update of runtime_release")
}

func TestFirstFatalError(t *testing.T) {
	appCtx := NewApplicationContext()

	_, found := LoadFirstFatalError(appCtx)
	require.False(t, found)

	StoreFirstFatalError(appCtx, model.WrapErrorIntoCustomerFatalError(nil, model.ErrorAgentCrash))
	v, found := LoadFirstFatalError(appCtx)
	require.True(t, found)
	require.Equal(t, model.ErrorAgentCrash, v.ErrorType())

	StoreFirstFatalError(appCtx, model.WrapErrorIntoCustomerFatalError(nil, model.ErrorAgentExit))
	v, found = LoadFirstFatalError(appCtx)
	require.True(t, found)
	require.Equal(t, model.ErrorAgentCrash, v.ErrorType())
}
