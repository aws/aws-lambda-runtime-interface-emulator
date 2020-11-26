// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package appctx

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.amzn.com/lambda/fatalerror"
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
