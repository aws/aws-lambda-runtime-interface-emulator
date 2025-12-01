// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/appctx"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/testdata"
)

const InitCachingToken = "sampleInitCachingToken"
const InitCachingAwsKey = "sampleAwsKey"
const InitCachingAwsSecret = "sampleAwsSecret"
const InitCachingAwsSessionToken = "sampleAwsSessionToken"

func getRequestContext() (http.Handler, *http.Request, *httptest.ResponseRecorder) {
	flowTest := testdata.NewFlowTest()

	flowTest.ConfigureForInitCaching(InitCachingToken, InitCachingAwsKey, InitCachingAwsSecret, InitCachingAwsSessionToken)

	handler := NewCredentialsHandler(flowTest.CredentialsService)
	responseRecorder := httptest.NewRecorder()
	appCtx := flowTest.AppCtx

	request := appctx.RequestWithAppCtx(httptest.NewRequest("", "/", nil), appCtx)

	return handler, request, responseRecorder
}

func TestEmptyAuthorizationHeader(t *testing.T) {
	handler, request, responseRecorder := getRequestContext()

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusNotFound, responseRecorder.Code)
}

func TestArbitraryAuthorizationHeader(t *testing.T) {
	handler, request, responseRecorder := getRequestContext()
	request.Header.Set("Authorization", "randomAuthToken")

	handler.ServeHTTP(responseRecorder, request)
	assert.Equal(t, http.StatusNotFound, responseRecorder.Code)
}

func TestSuccessfulGet(t *testing.T) {
	handler, request, responseRecorder := getRequestContext()
	request.Header.Set("Authorization", InitCachingToken)

	handler.ServeHTTP(responseRecorder, request)

	var responseMap map[string]string
	json.Unmarshal(responseRecorder.Body.Bytes(), &responseMap)
	assert.Equal(t, InitCachingAwsKey, responseMap["AccessKeyId"])
	assert.Equal(t, InitCachingAwsSecret, responseMap["SecretAccessKey"])
	assert.Equal(t, InitCachingAwsSessionToken, responseMap["Token"])

	expirationTime, err := time.Parse(time.RFC3339, responseMap["Expiration"])
	assert.NoError(t, err)
	durationUntilExpiration := time.Until(expirationTime)
	assert.True(t, durationUntilExpiration.Minutes() <= 30 && durationUntilExpiration.Minutes() > 29 && durationUntilExpiration.Hours() < 1)
	log.Println(responseRecorder.Body.String())
}
