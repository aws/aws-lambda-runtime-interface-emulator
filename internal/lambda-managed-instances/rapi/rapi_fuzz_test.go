// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/testdata"
)

func makeRapiServer(flowTest *testdata.FlowTest) *Server {
	s, err := NewServer(
		netip.MustParseAddrPort("127.0.0.1:0"),
		flowTest.AppCtx,
		flowTest.RegistrationService,
		flowTest.RenderingService,
		flowTest.TelemetrySubscription,
		nil,
	)
	if err != nil {
		panic(err)
	}
	return s
}

func makeTargetURL(path string, apiVersion string) string {
	protocol := "http"
	endpoint := os.Getenv("AWS_LAMBDA_RUNTIME_API")
	baseurl := fmt.Sprintf("%s://%s%s", protocol, endpoint, apiVersion)

	url := fmt.Sprintf("%s%s", baseurl, path)

	return strings.TrimRight(url, "#")
}

func serveTestRequest(rapiServer *Server, request *http.Request) *httptest.ResponseRecorder {
	responseRecorder := httptest.NewRecorder()
	rapiServer.server.Handler.ServeHTTP(responseRecorder, request)
	slog.Debug("test request", "url", request.URL, "status_code", responseRecorder.Code)

	return responseRecorder
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

func assertExpectedPathResponseCode(t *testing.T, code int, target string) {
	if code != http.StatusOK &&
		code != http.StatusAccepted &&
		code != http.StatusForbidden {
		t.Errorf("Unexpected status code (%v) for target (%v)", code, target)
	}
}

func assertUnexpectedPathResponseCode(t *testing.T, code int, target string) {
	if code != http.StatusNotFound &&
		code != http.StatusMethodNotAllowed &&
		code != http.StatusBadRequest {
		t.Errorf("Unexpected status code (%v) for target (%v)", code, target)
	}
}
