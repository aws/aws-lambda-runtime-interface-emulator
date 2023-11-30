// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"go.amzn.com/lambda/appctx"
	"go.amzn.com/lambda/extensions"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/telemetry"
	"go.amzn.com/lambda/testdata"
)

type runtimeFunctionErrStruct struct {
	ErrorMessage string
	ErrorType    string
	StackTrace   []string
}

func FuzzRuntimeAPIRouter(f *testing.F) {
	extensions.Enable()
	defer extensions.Disable()

	addSeedCorpusURLTargets(f)

	f.Fuzz(func(t *testing.T, rawPath string, payload []byte, isGetMethod bool) {
		u, err := parseToURLStruct(rawPath)
		if err != nil {
			t.Skipf("error parsing url: %v. Skipping test.", err)
		}

		flowTest := testdata.NewFlowTest()
		flowTest.ConfigureForInit()

		invoke := createDummyInvoke()
		flowTest.ConfigureForInvoke(context.Background(), invoke)

		appctx.StoreInitType(flowTest.AppCtx, true)

		rapiServer := makeRapiServer(flowTest)

		method := "GET"
		if !isGetMethod {
			method = "POST"
		}

		request := httptest.NewRequest(method, rawPath, bytes.NewReader(payload))
		responseRecorder := serveTestRequest(rapiServer, request)

		if isExpectedPath(u.Path, invoke.ID, isGetMethod) {
			assertExpectedPathResponseCode(t, responseRecorder.Code, rawPath)
		} else {
			assertUnexpectedPathResponseCode(t, responseRecorder.Code, rawPath)
		}
	})
}

func FuzzInitErrorHandler(f *testing.F) {
	addRuntimeFunctionErrorJSONCorpus(f)

	f.Fuzz(func(t *testing.T, errorBody []byte, errTypeHeader []byte) {
		flowTest := testdata.NewFlowTest()
		flowTest.ConfigureForInit()

		rapiServer := makeRapiServer(flowTest)

		target := makeTargetURL("/runtime/init/error", version20180601)
		request := httptest.NewRequest("POST", target, bytes.NewReader(errorBody))
		request = appctx.RequestWithAppCtx(request, flowTest.AppCtx)
		request.Header.Set("Lambda-Runtime-Function-Error-Type", string(errTypeHeader))

		responseRecorder := serveTestRequest(rapiServer, request)

		assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
		assert.JSONEq(t, "{\"status\":\"OK\"}\n", responseRecorder.Body.String())
		assert.Equal(t, "application/json", responseRecorder.Header().Get("Content-Type"))

		assertErrorResponsePersists(t, errorBody, errTypeHeader, flowTest)
	})
}

func FuzzInvocationResponseHandler(f *testing.F) {
	f.Add([]byte("SUCCESS"), []byte("application/json"), []byte("streaming"))
	f.Add([]byte(strings.Repeat("a", interop.MaxPayloadSize+1)), []byte("application/json"), []byte("streaming"))

	f.Fuzz(func(t *testing.T, responseBody []byte, contentType []byte, responseMode []byte) {
		flowTest := testdata.NewFlowTest()
		flowTest.ConfigureForInit()
		flowTest.Runtime.Ready()

		invoke := createDummyInvoke()
		flowTest.ConfigureForInvoke(context.Background(), invoke)

		rapiServer := makeRapiServer(flowTest)

		target := makeTargetURL(fmt.Sprintf("/runtime/invocation/%s/response", invoke.ID), version20180601)
		request := httptest.NewRequest("POST", target, bytes.NewReader(responseBody))
		request.Header.Set("Content-Type", string(contentType))
		request.Header.Set("Lambda-Runtime-Function-Response-Mode", string(responseMode))

		request = appctx.RequestWithAppCtx(request, flowTest.AppCtx)

		responseRecorder := serveTestRequest(rapiServer, request)

		if !isValidResponseMode(responseMode) {
			assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
			return
		}

		if len(responseBody) > interop.MaxPayloadSize {
			assertInvocationResponseTooLarge(t, responseRecorder, flowTest, responseBody)
		} else {
			assertInvocationResponseAccepted(t, responseRecorder, flowTest, responseBody, contentType)
		}
	})
}

func FuzzInvocationErrorHandler(f *testing.F) {
	addRuntimeFunctionErrorJSONCorpus(f)

	f.Fuzz(func(t *testing.T, errorBody []byte, errTypeHeader []byte) {
		flowTest := testdata.NewFlowTest()
		flowTest.ConfigureForInit()
		flowTest.Runtime.Ready()
		appCtx := flowTest.AppCtx

		invoke := createDummyInvoke()
		flowTest.ConfigureForInvoke(context.Background(), invoke)

		rapiServer := makeRapiServer(flowTest)

		target := makeTargetURL(fmt.Sprintf("/runtime/invocation/%s/error", invoke.ID), version20180601)
		request := httptest.NewRequest("POST", target, bytes.NewReader(errorBody))
		request = appctx.RequestWithAppCtx(request, appCtx)

		request.Header.Set("Lambda-Runtime-Function-Error-Type", string(errTypeHeader))

		responseRecorder := serveTestRequest(rapiServer, request)

		assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
		assert.JSONEq(t, "{\"status\":\"OK\"}\n", responseRecorder.Body.String())
		assert.Equal(t, "application/json", responseRecorder.Header().Get("Content-Type"))

		assertErrorResponsePersists(t, errorBody, errTypeHeader, flowTest)
	})
}

func FuzzRestoreErrorHandler(f *testing.F) {
	f.Fuzz(func(t *testing.T, errorBody []byte, errTypeHeader []byte) {
		flowTest := testdata.NewFlowTest()
		flowTest.ConfigureForRestoring()

		appctx.StoreInitType(flowTest.AppCtx, true)

		rapiServer := makeRapiServer(flowTest)

		target := makeTargetURL("/runtime/restore/error", version20180601)
		request := httptest.NewRequest("POST", target, bytes.NewReader(errorBody))
		request = appctx.RequestWithAppCtx(request, flowTest.AppCtx)

		request.Header.Set("Lambda-Runtime-Function-Error-Type", string(errTypeHeader))

		responseRecorder := serveTestRequest(rapiServer, request)

		assert.Equal(t, http.StatusAccepted, responseRecorder.Code)
		assert.JSONEq(t, "{\"status\":\"OK\"}\n", responseRecorder.Body.String())
		assert.Equal(t, "application/json", responseRecorder.Header().Get("Content-Type"))
	})
}

func makeRapiServer(flowTest *testdata.FlowTest) *Server {
	return NewServer(
		"127.0.0.1",
		0,
		flowTest.AppCtx,
		flowTest.RegistrationService,
		flowTest.RenderingService,
		true,
		&telemetry.NoOpSubscriptionAPI{},
		flowTest.TelemetrySubscription,
		flowTest.CredentialsService,
	)
}

func createDummyInvoke() *interop.Invoke {
	return &interop.Invoke{
		ID:      "InvocationID1",
		Payload: strings.NewReader("Payload1"),
	}
}

func makeTargetURL(path string, apiVersion string) string {
	protocol := "http"
	endpoint := os.Getenv("AWS_LAMBDA_RUNTIME_API")
	baseurl := fmt.Sprintf("%s://%s%s", protocol, endpoint, apiVersion)

	return fmt.Sprintf("%s%s", baseurl, path)
}

func serveTestRequest(rapiServer *Server, request *http.Request) *httptest.ResponseRecorder {
	responseRecorder := httptest.NewRecorder()
	rapiServer.server.Handler.ServeHTTP(responseRecorder, request)
	log.Printf("test(%v) = %v", request.URL, responseRecorder.Code)

	return responseRecorder
}

func addSeedCorpusURLTargets(f *testing.F) {
	invoke := createDummyInvoke()
	errStruct := runtimeFunctionErrStruct{
		ErrorMessage: "error occurred",
		ErrorType:    "Runtime.UnknownReason",
		StackTrace:   []string{},
	}
	errJSON, _ := json.Marshal(errStruct)
	f.Add(makeTargetURL("/runtime/init/error", version20180601), errJSON, false)
	f.Add(makeTargetURL("/runtime/invocation/next", version20180601), []byte{}, true)
	f.Add(makeTargetURL(fmt.Sprintf("/runtime/invocation/%s/response", invoke.ID), version20180601), []byte("SUCCESS"), false)
	f.Add(makeTargetURL(fmt.Sprintf("/runtime/invocation/%s/error", invoke.ID), version20180601), errJSON, false)
	f.Add(makeTargetURL("/runtime/restore/next", version20180601), []byte{}, true)
	f.Add(makeTargetURL("/runtime/restore/error", version20180601), errJSON, false)

	f.Add(makeTargetURL("/extension/register", version20200101), []byte("register"), false)
	f.Add(makeTargetURL("/extension/event/next", version20200101), []byte("next"), true)
	f.Add(makeTargetURL("/extension/init/error", version20200101), []byte("init error"), false)
	f.Add(makeTargetURL("/extension/exit/error", version20200101), []byte("exit error"), false)
}

func addRuntimeFunctionErrorJSONCorpus(f *testing.F) {
	runtimeFuncErr := runtimeFunctionErrStruct{
		ErrorMessage: "error",
		ErrorType:    "Runtime.Unknown",
		StackTrace:   []string{},
	}
	data, _ := json.Marshal(runtimeFuncErr)

	f.Add(data, []byte("Runtime.Unknown"))
}

func isExpectedPath(path string, invokeID string, isGetMethod bool) bool {
	expectedPaths := make(map[string]bool)

	expectedPaths[fmt.Sprintf("%s/runtime/init/error", version20180601)] = false
	expectedPaths[fmt.Sprintf("%s/runtime/invocation/next", version20180601)] = true
	expectedPaths[fmt.Sprintf("%s/runtime/invocation/%s/response", version20180601, invokeID)] = false
	expectedPaths[fmt.Sprintf("%s/runtime/invocation/%s/error", version20180601, invokeID)] = false
	expectedPaths[fmt.Sprintf("%s/runtime/restore/next", version20180601)] = true
	expectedPaths[fmt.Sprintf("%s/runtime/restore/error", version20180601)] = false

	expectedPaths[fmt.Sprintf("%s/extension/register", version20200101)] = false
	expectedPaths[fmt.Sprintf("%s/extension/event/next", version20200101)] = true
	expectedPaths[fmt.Sprintf("%s/extension/init/error", version20200101)] = false
	expectedPaths[fmt.Sprintf("%s/extension/exit/error", version20200101)] = false

	val, found := expectedPaths[path]
	return found && (val == isGetMethod)
}

func parseToURLStruct(rawPath string) (*url.URL, error) {
	invalidChars := regexp.MustCompile(`[ %]+`)
	if invalidChars.MatchString(rawPath) {
		return nil, errors.New("url must not contain spaces or %")
	}

	for _, r := range rawPath {
		if !unicode.IsGraphic(r) {
			return nil, errors.New("url contains non-graphic runes")
		}
	}

	if _, err := url.ParseRequestURI(rawPath); err != nil {
		return nil, err
	}

	u, err := url.Parse(rawPath)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "" {
		return nil, errors.New("blank url scheme")
	}

	return u, nil
}

func assertInvocationResponseAccepted(t *testing.T, responseRecorder *httptest.ResponseRecorder,
	flowTest *testdata.FlowTest, responseBody []byte, contentType []byte) {
	assert.Equal(t, http.StatusAccepted, responseRecorder.Code,
		"Handler returned wrong status code: got %v expected %v",
		responseRecorder.Code, http.StatusAccepted)

	expectedAPIResponse := "{\"status\":\"OK\"}\n"
	body, err := io.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)
	assert.JSONEq(t, expectedAPIResponse, string(body))

	response := flowTest.InteropServer.Response
	assert.NotNil(t, response)
	assert.Nil(t, flowTest.InteropServer.ErrorResponse)

	assert.Equal(t, string(contentType), flowTest.InteropServer.ResponseContentType)

	assert.Equal(t, responseBody, response,
		"Persisted response data in app context must match the submitted.")
}

func assertInvocationResponseTooLarge(t *testing.T, responseRecorder *httptest.ResponseRecorder, flowTest *testdata.FlowTest, responseBody []byte) {
	assert.Equal(t, http.StatusRequestEntityTooLarge, responseRecorder.Code, "Handler returned wrong status code: got %v expected %v",
		responseRecorder.Code, http.StatusRequestEntityTooLarge)

	expectedAPIResponse := fmt.Sprintf("{\"errorMessage\":\"Exceeded maximum allowed payload size (%d bytes).\",\"errorType\":\"RequestEntityTooLarge\"}\n", interop.MaxPayloadSize)
	body, err := io.ReadAll(responseRecorder.Body)
	assert.NoError(t, err)
	assert.JSONEq(t, expectedAPIResponse, string(body))

	errorResponse := flowTest.InteropServer.ErrorResponse
	assert.NotNil(t, errorResponse)
	assert.Nil(t, flowTest.InteropServer.Response)
	assert.Equal(t, fatalerror.FunctionOversizedResponse, errorResponse.FunctionError.Type)
	assert.Equal(t, fmt.Sprintf("Response payload size (%v bytes) exceeded maximum allowed payload size (6291556 bytes).", len(responseBody)), errorResponse.FunctionError.Message)

	var errorPayload map[string]interface{}
	assert.NoError(t, json.Unmarshal(errorResponse.Payload, &errorPayload))
	assert.Equal(t, string(errorResponse.FunctionError.Type), errorPayload["errorType"])
	assert.Equal(t, errorResponse.FunctionError.Message, errorPayload["errorMessage"])
}

func assertErrorResponsePersists(t *testing.T, errorBody []byte, errTypeHeader []byte, flowTest *testdata.FlowTest) {
	errorResponse := flowTest.InteropServer.ErrorResponse
	assert.NotNil(t, errorResponse)
	assert.Nil(t, flowTest.InteropServer.Response)

	var runtimeFunctionErr runtimeFunctionErrStruct
	var expectedErrMsg string

	// If input payload is a valid function error json object,
	// assert that the error message persisted in the response
	err := json.Unmarshal(errorBody, &runtimeFunctionErr)
	if err != nil {
		expectedErrMsg = runtimeFunctionErr.ErrorMessage
	}
	assert.Equal(t, expectedErrMsg, errorResponse.FunctionError.Message)

	// If input error type is valid (within the allow-listed value,
	// assert that the error type persisted in the response
	expectedErrType := fatalerror.GetValidRuntimeOrFunctionErrorType(string(errTypeHeader))
	assert.Equal(t, expectedErrType, errorResponse.FunctionError.Type)

	assert.Equal(t, errorBody, errorResponse.Payload)
}

func isValidResponseMode(responseMode []byte) bool {
	responseModeStr := string(responseMode)
	return responseModeStr == "streaming" ||
		responseModeStr == ""
}

func assertExpectedPathResponseCode(t *testing.T, code int, target string) {
	if !(code == http.StatusOK ||
		code == http.StatusAccepted ||
		code == http.StatusForbidden) {
		t.Errorf("Unexpected status code (%v) for target (%v)", code, target)
	}
}

func assertUnexpectedPathResponseCode(t *testing.T, code int, target string) {
	if !(code == http.StatusNotFound ||
		code == http.StatusMethodNotAllowed ||
		code == http.StatusBadRequest) {
		t.Errorf("Unexpected status code (%v) for target (%v)", code, target)
	}
}
