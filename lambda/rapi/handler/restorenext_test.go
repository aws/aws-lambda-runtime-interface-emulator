// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/telemetry"
	"go.amzn.com/lambda/testdata"
)

func TestRenderRestoreNext(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	handler := NewRestoreNextHandler(flowTest.RegistrationService, flowTest.RenderingService)
	responseRecorder := httptest.NewRecorder()
	appCtx := flowTest.AppCtx

	flowTest.ConfigureForRestore()
	request := appctx.RequestWithAppCtx(httptest.NewRequest("", "/", nil), appCtx)
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)
}

func TestBrokenRenderer(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	handler := NewRestoreNextHandler(flowTest.RegistrationService, flowTest.RenderingService)
	responseRecorder := httptest.NewRecorder()
	appCtx := flowTest.AppCtx

	flowTest.ConfigureForRestore()
	flowTest.RenderingService.SetRenderer(&mockBrokenRenderer{})
	request := appctx.RequestWithAppCtx(httptest.NewRequest("", "/", nil), appCtx)
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)

	assert.JSONEq(t, `{"errorMessage":"Internal Server Error","errorType":"InternalServerError"}`, responseRecorder.Body.String())
}

func TestRenderRestoreAfterInvoke(t *testing.T) {
	flowTest := testdata.NewFlowTest()
	flowTest.ConfigureForInit()
	handler := NewInvocationNextHandler(flowTest.RegistrationService, flowTest.RenderingService)
	responseRecorder := httptest.NewRecorder()
	appCtx := flowTest.AppCtx

	deadlineNs := 12345
	invokePayload := "Payload"
	invoke := &interop.Invoke{
		TraceID:               "Root=RootID;Parent=LambdaFrontend;Sampled=1",
		ID:                    "ID",
		InvokedFunctionArn:    "InvokedFunctionArn",
		CognitoIdentityID:     "CognitoIdentityId1",
		CognitoIdentityPoolID: "CognitoIdentityPoolId1",
		ClientContext:         "ClientContext",
		DeadlineNs:            strconv.Itoa(deadlineNs),
		ContentType:           "image/png",
		Payload:               strings.NewReader(invokePayload),
	}

	ctx := telemetry.NewTraceContext(context.Background(), "RootID", "InvocationSubegmentID")
	flowTest.ConfigureForInvoke(ctx, invoke)

	request := appctx.RequestWithAppCtx(httptest.NewRequest("", "/", nil), appCtx)
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, http.StatusOK, responseRecorder.Code)

	restoreHandler := NewRestoreNextHandler(flowTest.RegistrationService, flowTest.RenderingService)
	restoreRequest := appctx.RequestWithAppCtx(httptest.NewRequest("", "/", nil), appCtx)
	responseRecorder = httptest.NewRecorder()
	restoreHandler.ServeHTTP(responseRecorder, restoreRequest)

	assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
}
