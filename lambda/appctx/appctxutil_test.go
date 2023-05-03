// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package appctx

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.amzn.com/lambda/fatalerror"

	"go.amzn.com/lambda/interop"
)

func runTestRequestWithUserAgent(t *testing.T, userAgent string, expectedRuntimeRelease string) {
	// Simple User_Agent passed.
	// GIVEN
	req := httptest.NewRequest("", "/", nil)
	req.Header.Set("User-Agent", userAgent)
	request := RequestWithAppCtx(req, NewApplicationContext())
	appCtx := request.Context().Value(ReqCtxApplicationContextKey).(ApplicationContext)

	// DO
	ok := UpdateAppCtxWithRuntimeRelease(request, appCtx)

	//ASSERT
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
	// GIVEN
	// No User_Agent passed.
	request := RequestWithAppCtx(httptest.NewRequest("", "/", nil), NewApplicationContext())
	appCtx := request.Context().Value(ReqCtxApplicationContextKey).(ApplicationContext)

	// DO
	ok := UpdateAppCtxWithRuntimeRelease(request, appCtx)

	// ASSERT
	assert.False(t, ok)
	_, ok = appCtx.Load(AppCtxRuntimeReleaseKey)
	assert.False(t, ok)
}

func TestUpdateAppCtxWithRuntimeReleaseWithBlankUserAgent(t *testing.T) {
	// GIVEN
	req := httptest.NewRequest("", "/", nil)
	req.Header.Set("User-Agent", "        ")
	request := RequestWithAppCtx(req, NewApplicationContext())
	appCtx := request.Context().Value(ReqCtxApplicationContextKey).(ApplicationContext)

	// DO
	ok := UpdateAppCtxWithRuntimeRelease(request, appCtx)

	// ASSERT
	assert.False(t, ok)
	_, ok = appCtx.Load(AppCtxRuntimeReleaseKey)
	assert.False(t, ok)
}

func TestUpdateAppCtxWithRuntimeReleaseWithLambdaRuntimeFeatures(t *testing.T) {
	// GIVEN
	// Simple LambdaRuntimeFeatures passed.
	req := httptest.NewRequest("", "/", nil)
	req.Header.Set("User-Agent", "Node.js/14.16.0")
	req.Header.Set("Lambda-Runtime-Features", "httpcl/2.0 execwr nodewr/4.3")
	request := RequestWithAppCtx(req, NewApplicationContext())
	appCtx := request.Context().Value(ReqCtxApplicationContextKey).(ApplicationContext)

	// DO
	ok := UpdateAppCtxWithRuntimeRelease(request, appCtx)

	//ASSERT
	assert.True(t, ok, "runtime_release updated based only on User-Agent and valid features")
	ctxRuntimeRelease, ok := appCtx.Load(AppCtxRuntimeReleaseKey)
	assert.True(t, ok)
	assert.Equal(t, "Node.js/14.16.0 (httpcl/2.0 execwr nodewr/4.3)", ctxRuntimeRelease)
}

// Test that RAPID allows updating runtime_release only once
func TestUpdateAppCtxWithRuntimeReleaseMultipleTimes(t *testing.T) {
	// GIVEN
	firstValue := "Value1"
	secondValue := "Value2"

	req := httptest.NewRequest("", "/", nil)
	req.Header.Set("User-Agent", firstValue)
	request := RequestWithAppCtx(req, NewApplicationContext())
	appCtx := request.Context().Value(ReqCtxApplicationContextKey).(ApplicationContext)

	// DO
	ok := UpdateAppCtxWithRuntimeRelease(request, appCtx)

	// ASSERT
	assert.True(t, ok)
	ctxRuntimeRelease, ok := appCtx.Load(AppCtxRuntimeReleaseKey)
	assert.True(t, ok)
	assert.Equal(t, firstValue, ctxRuntimeRelease)

	// GIVEN
	req.Header.Set("User-Agent", secondValue)

	// DO
	ok = UpdateAppCtxWithRuntimeRelease(request, appCtx)

	// ASSERT
	assert.False(t, ok, "failed to prevent second update of runtime_release")
	ctxRuntimeRelease, ok = appCtx.Load(AppCtxRuntimeReleaseKey)
	assert.True(t, ok)
	assert.Equal(t, firstValue, ctxRuntimeRelease, "failed to prevent second update of runtime_release")
}

func TestFirstFatalError(t *testing.T) {
	appCtx := NewApplicationContext()

	_, found := LoadFirstFatalError(appCtx)
	require.False(t, found)

	StoreFirstFatalError(appCtx, fatalerror.AgentCrash)
	v, found := LoadFirstFatalError(appCtx)
	require.True(t, found)
	require.Equal(t, fatalerror.AgentCrash, v)

	StoreFirstFatalError(appCtx, fatalerror.AgentExitError)
	v, found = LoadFirstFatalError(appCtx)
	require.True(t, found)
	require.Equal(t, fatalerror.AgentCrash, v)
}

func TestStoreLoadInitType(t *testing.T) {
	appCtx := NewApplicationContext()

	initType := LoadInitType(appCtx)
	assert.Equal(t, Init, initType)

	StoreInitType(appCtx, true)
	initType = LoadInitType(appCtx)
	assert.Equal(t, InitCaching, initType)
}

func TestStoreLoadSandboxType(t *testing.T) {
	appCtx := NewApplicationContext()

	sandboxType := LoadSandboxType(appCtx)
	assert.Equal(t, interop.SandboxClassic, sandboxType)

	StoreSandboxType(appCtx, interop.SandboxPreWarmed)

	sandboxType = LoadSandboxType(appCtx)
	assert.Equal(t, interop.SandboxPreWarmed, sandboxType)
}
