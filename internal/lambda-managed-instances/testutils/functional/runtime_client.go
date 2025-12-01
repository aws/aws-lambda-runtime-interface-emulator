// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build test

package functional

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"strings"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
)

const (
	protocol   = "http"
	apiversion = "2018-06-01"
)

const (
	ContentTypeHeader              = "Content-Type"
	LambdaInvocationIDHeader       = "Lambda-Runtime-Aws-Request-Id"
	LambdaInvocationDeadlineHeader = "Lambda-Runtime-Deadline-Ms"
	LambdaErrorTypeHeader          = "Lambda-Runtime-Function-Error-Type"
	LambdaErrorBodyHeader          = "Lambda-Runtime-Function-Error-Body"
	LambdaResponseModeHeader       = "Lambda-Runtime-Function-Response-Mode"
	LambdaXRayErrorCauseHeader     = "Lambda-Runtime-Function-XRay-Error-Cause"
)

type Client struct {
	baseurl string
	client  http.Client
}

func NewClient(endpoint netip.AddrPort) *Client {
	return &Client{
		baseurl: fmt.Sprintf("%s://%s/%s", protocol, endpoint, apiversion),
		client:  http.Client{},
	}
}

func (client *Client) Next(body io.Reader) *http.Response {
	slog.Debug("Runtime Client calling Next", "baseurl", client.baseurl)

	url := fmt.Sprintf("%s/runtime/invocation/next", client.baseurl)
	req, err := http.NewRequest(http.MethodGet, url, body)
	if err != nil {
		slog.Error("could not create request", "url", url, "error", err)
		panic(fmt.Sprintf("could not create request for %s: %s", url, err))
	}

	headers := make(map[string]string)
	headersJSON := os.Getenv("INVOCATION_NEXT_REQUEST_HEADERS")
	if headersJSON != "" {
		if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
			slog.Error("failed to unmarshal headers", "error", err)
			panic(err)
		}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.client.Do(req)
	if err != nil {
		slog.Error("Unable to call URL", "url", url, "error", err)

		return nil

	}

	return resp
}

func (client *Client) Response(invokeID interop.InvokeID, payload io.Reader, contentType string, responseModeHeader string, trailers map[string]string) (*http.Response, error) {
	url := fmt.Sprintf("%s/runtime/invocation/%s/response", client.baseurl, invokeID)
	headers := map[string]string{ContentTypeHeader: contentType}

	if responseModeHeader != "" {
		headers[LambdaResponseModeHeader] = responseModeHeader
	}
	return client.postBufferedResponse(url, payload, headers, trailers)
}

func (client *Client) ResponseError(invokeID interop.InvokeID, payload string, contentType string, errorType string, errorCause string) (*http.Response, error) {
	headers := map[string]string{ContentTypeHeader: contentType}
	if len(errorType) > 0 {
		headers[LambdaErrorTypeHeader] = errorType
	}
	if len(errorCause) > 0 {
		headers[LambdaXRayErrorCauseHeader] = errorCause
	}
	return client.ResponseErrorWithHeaders(invokeID, payload, headers)
}

func (client *Client) ResponseErrorWithHeaders(invokeID interop.InvokeID, payload string, headers map[string]string) (*http.Response, error) {
	url := fmt.Sprintf("%s/runtime/invocation/%s/error", client.baseurl, invokeID)
	return client.postBufferedResponse(url, strings.NewReader(payload), headers, nil)
}

func (client *Client) InitError(payload string, contentType string, errorType string) (*http.Response, error) {
	url := fmt.Sprintf("%s/runtime/init/error", client.baseurl)
	headers := map[string]string{ContentTypeHeader: contentType}
	if len(errorType) > 0 {
		headers[LambdaErrorTypeHeader] = errorType
	}
	return client.postBufferedResponse(url, strings.NewReader(payload), headers, nil)
}

func (client *Client) postBufferedResponse(url string, payload io.Reader, headers, trailers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		slog.Error("Unable to create request", "url", url, "error", err)
		panic(err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	if len(trailers) > 0 {
		req.ContentLength = -1
		req.Trailer = make(http.Header)
		req.TransferEncoding = []string{"chunked"}
		for key, value := range trailers {
			req.Header.Add("Trailer", key)
			req.Trailer.Set(key, value)
		}
	}

	return client.client.Do(req)
}

func (client *Client) postResponse(req *http.Request) *http.Response {
	resp, err := client.client.Do(req)

	if err != nil {

		slog.Error("Got postResponse error", "err", err)

	}

	return resp
}
