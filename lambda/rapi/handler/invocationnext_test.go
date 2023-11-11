// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/metering"
	"go.amzn.com/lambda/telemetry"
	"go.amzn.com/lambda/testdata"
)

// TestRenderInvokeEmptyHeaders tests that headers
// are not rendered when not set.
func TestRenderInvokeEmptyHeaders(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	handler := NewInvocationNextHandler(flowTest.RegistrationService, flowTest.RenderingService)
	responseRecorder := httptest.NewRecorder()
	appCtx := flowTest.AppCtx

	flowTest.ConfigureForInvoke(context.Background(), &interop.Invoke{})
	request := appctx.RequestWithAppCtx(httptest.NewRequest("", "/", nil), appCtx)
	handler.ServeHTTP(responseRecorder, request)

	headers := responseRecorder.Header()
	assert.Equal(t, "application/json", headers.Get("Content-Type"))
	assert.Len(t, headers, 1)
	assert.Equal(t, http.StatusOK, responseRecorder.Code)
}

func TestRenderInvokeHappy(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	appCtx := flowTest.AppCtx

	deadlineNs := 12345
	invoke := &interop.Invoke{
		TraceID:               "Root=RootID;Parent=LambdaFrontend;Sampled=1",
		ID:                    "", // updated in loop
		InvokedFunctionArn:    "InvokedFunctionArn",
		CognitoIdentityID:     "CognitoIdentityId1",
		CognitoIdentityPoolID: "CognitoIdentityPoolId1",
		ClientContext:         "ClientContext",
		DeadlineNs:            strconv.Itoa(deadlineNs),
		ContentType:           "image/png",
		Payload:               strings.NewReader(""), // updated in loop
	}

	ctx := telemetry.NewTraceContext(context.Background(), "RootID", "InvocationSubegmentID")
	var requestBuffer bytes.Buffer
	for i := 0; i < 6; i++ {
		handler := NewInvocationNextHandler(flowTest.RegistrationService, flowTest.RenderingService)
		responseRecorder := httptest.NewRecorder()
		invoke.ID = fmt.Sprintf("ID-%d", i)
		invokePayload := string(bytes.Repeat([]byte("a"), (i%3)*128*1024)) // vary payload size up and down across invokes
		invoke.Payload = strings.NewReader(invokePayload)

		flowTest.ConfigureForInvoke(ctx, invoke)
		flowTest.ConfigureInvokeRenderer(ctx, invoke, &requestBuffer) // reuse request buffer on each invoke
		request := appctx.RequestWithAppCtx(httptest.NewRequest("", "/", nil), appCtx)
		handler.ServeHTTP(responseRecorder, request)

		headers := responseRecorder.Header()
		assert.Equal(t, invoke.InvokedFunctionArn, headers.Get("Lambda-Runtime-Invoked-Function-Arn"))
		assert.Equal(t, invoke.ID, headers.Get("Lambda-Runtime-Aws-Request-Id"))
		assert.Equal(t, invoke.ClientContext, headers.Get("Lambda-Runtime-Client-Context"))
		expectedCognitoIdentityHeader := fmt.Sprintf("{\"cognitoIdentityId\":\"%s\",\"cognitoIdentityPoolId\":\"%s\"}", invoke.CognitoIdentityID, invoke.CognitoIdentityPoolID)
		assert.JSONEq(t, expectedCognitoIdentityHeader, headers.Get("Lambda-Runtime-Cognito-Identity"))
		assert.Equal(t, "Root=RootID;Parent=InvocationSubegmentID;Sampled=1", headers.Get("Lambda-Runtime-Trace-Id"))

		// Assert deadline precision. E.g. 1999 ns and 2001 ns having diff of 2 ns
		// would result in 1ms and 2ms deadline correspondingly.
		expectedDeadline := metering.MonoToEpoch(int64(deadlineNs)) / int64(time.Millisecond)
		receivedDeadline, _ := strconv.ParseInt(headers.Get("Lambda-Runtime-Deadline-Ms"), 10, 64)
		assert.True(t, math.Abs(float64(expectedDeadline-receivedDeadline)) <= float64(1),
			fmt.Sprintf("Expected: %v, received: %v", expectedDeadline, receivedDeadline))

		assert.Equal(t, "image/png", headers.Get("Content-Type"))
		assert.Len(t, headers, 7)
		responsePayload := responseRecorder.Body.String()
		require.Equalf(t, len(invokePayload), len(responsePayload), "Unexpected payload for request %d", i)
		assert.Equal(t, invokePayload, responsePayload)
	}
}

// Cgo calls removed due to crashes while spawning threads under memory pressure.
func TestRenderInvokeDoesNotCallCgo(t *testing.T) {
	cgoCallsBefore := runtime.NumCgoCall()
	TestRenderInvokeHappy(t)
	cgoCallsAfter := runtime.NumCgoCall()
	assert.Equal(t, cgoCallsBefore, cgoCallsAfter)
}

func BenchmarkRenderInvoke(b *testing.B) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	handler := NewInvocationNextHandler(flowTest.RegistrationService, flowTest.RenderingService)
	responseRecorder := httptest.NewRecorder()
	appCtx := flowTest.AppCtx

	deadlineNs := 12345
	invoke := &interop.Invoke{
		TraceID:               "Root=RootID;Parent=LambdaFrontend;Sampled=1",
		ID:                    "ID",
		InvokedFunctionArn:    "InvokedFunctionArn",
		CognitoIdentityID:     "CognitoIdentityId1",
		CognitoIdentityPoolID: "CognitoIdentityPoolId1",
		ClientContext:         "ClientContext",
		DeadlineNs:            strconv.Itoa(deadlineNs),
		ContentType:           "image/png",
		Payload:               strings.NewReader("Payload"),
	}

	ctx := telemetry.NewTraceContext(context.Background(), "RootID", "InvocationSubegmentID")
	flowTest.ConfigureForInvoke(ctx, invoke)

	request := appctx.RequestWithAppCtx(httptest.NewRequest("", "/", nil), appCtx)

	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(responseRecorder, request)
	}
}

type mockBrokenRenderer struct{}

// RenderAgentEvent renders shutdown event for agent.
func (s *mockBrokenRenderer) RenderAgentEvent(w http.ResponseWriter, r *http.Request) error {
	return errors.New("Broken")
}

// RenderRuntimeEvent renders shutdown event for runtime.
func (s *mockBrokenRenderer) RenderRuntimeEvent(w http.ResponseWriter, r *http.Request) error {
	return errors.New("Broken")
}

func TestRender500AndExitOnInteropFailureDuringFirstInvoke(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	handler := NewInvocationNextHandler(flowTest.RegistrationService, flowTest.RenderingService)
	responseRecorder := httptest.NewRecorder()
	appCtx := flowTest.AppCtx

	flowTest.InvokeFlow.InitializeBarriers()
	flowTest.RenderingService.SetRenderer(&mockBrokenRenderer{})

	request := appctx.RequestWithAppCtx(httptest.NewRequest("", "/", nil), appCtx)
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	assert.JSONEq(t, `{"errorMessage":"Internal Server Error","errorType":"InternalServerError"}`, responseRecorder.Body.String())
}

func TestMain(m *testing.M) {
	if err := runtime.StartTrace(); err != nil {
		log.Fatalf("Failed to start Golang tracer: %s", err)
		os.Exit(1)
	}
	defer runtime.StopTrace()

	os.Exit(m.Run())
}
