// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build test

package functional

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"time"
)

type (
	Event     string
	ConfigKey string
)

const (
	Shutdown Event = "SHUTDOWN"
)

const (
	lambdaAgentIdentifierHeaderKey string = "Lambda-Extension-Identifier"
	lambdaAgentNameHeaderKey       string = "Lambda-Extension-Name"
	lambdaAgentErrorTypeHeaderKey  string = "Lambda-Extension-Function-Error-Type"
	lambdaAcceptFeatureHeaderKey   string = "Lambda-Extension-Accept-Feature"
)

type errorExtensions interface {
	Error() string
}

func (client *Client) ExtensionsSleep(d time.Duration) {
	if d == 0 {
		d = 100 * time.Millisecond
	}
	time.Sleep(d)
}

type RegisterRequest struct {
	Events []Event
}

type rapidRegisterResponse struct {
	AccountID       *string           `json:"accountId"`
	FunctionName    string            `json:"functionName"`
	FunctionVersion string            `json:"functionVersion"`
	Handler         string            `json:"handler"`
	Configuration   map[string]string `json:"configuration"`
}

type StatusResponse struct {
	Status string `json:"status"`
}

type NextResponse struct {
	EventType Event `json:"eventType"`
}

type ShutdownResponse struct {
	*NextResponse
	ShutdownReason string `json:"shutdownReason"`
	DeadlineMs     int64  `json:"deadlineMs"`
}

type RapidHTTPError struct {
	StatusCode int
	Status     string
}

func (s *RapidHTTPError) Error() string {
	return fmt.Sprintf("/event/next failed: %d[%s]", s.StatusCode, s.Status)
}

func NewExtensionsClient(endpoint netip.AddrPort) *Client {
	return &Client{
		baseurl: fmt.Sprintf("http://%s/2020-01-01/extension", endpoint),
		client:  http.Client{},
	}
}

func (client *Client) ExtensionsRegister(agentUniqueName string, events []Event) (*http.Response, errorExtensions) {
	data, err := json.Marshal(
		&RegisterRequest{
			Events: events,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal RegisterRequest: %w", err)
	}

	return client.extensionsRegisterWithMarshalledRequest(agentUniqueName, data, nil)
}

func (client *Client) extensionsRegisterWithMarshalledRequest(
	agentUniqueName string, data []byte, headersOverrides map[string]string,
) (*http.Response, errorExtensions) {
	headers := map[string]string{lambdaAgentNameHeaderKey: agentUniqueName}
	for name, val := range headersOverrides {
		headers[name] = val
	}

	resp, err := HttpPostWithHeaders(&client.client, fmt.Sprintf("%s/register", client.baseurl), data, &headers)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var registerResponse rapidRegisterResponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if err := json.Unmarshal(body, &registerResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return resp, nil
}

func (client *Client) ExtensionsInitError(agentIdentifier, functionErrorType, payload string) (*StatusResponse, errorExtensions) {
	headers := make(map[string]string)
	headers[lambdaAgentIdentifierHeaderKey] = agentIdentifier
	headers[lambdaAgentErrorTypeHeaderKey] = functionErrorType

	resp, err := HttpPostWithHeaders(&client.client, fmt.Sprintf("%s/init/error", client.baseurl), []byte(payload), &headers)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var response StatusResponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

func (client *Client) ExtensionsExitError(agentIdentifier, functionErrorType, payload string) (*StatusResponse, errorExtensions) {
	headers := make(map[string]string)
	headers[lambdaAgentIdentifierHeaderKey] = agentIdentifier
	headers[lambdaAgentErrorTypeHeaderKey] = functionErrorType

	resp, err := HttpPostWithHeaders(&client.client, fmt.Sprintf("%s/exit/error", client.baseurl), []byte(payload), &headers)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var response StatusResponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

func (client *Client) ExtensionsNext(agentIdentifier string) (interface{}, errorExtensions) {
	invokeNextHeaders := map[string]string{lambdaAgentIdentifierHeaderKey: agentIdentifier}
	return client.ExtensionsNextWithHeaders(invokeNextHeaders)
}

func (client *Client) ExtensionsNextWithHeaders(headers map[string]string) (interface{}, errorExtensions) {
	resp, err := HttpGetWithHeaders(&client.client, fmt.Sprintf("%s/event/next", client.baseurl), &headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &RapidHTTPError{resp.StatusCode, resp.Status}
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			panic(err)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var nextResponse NextResponse
	if err := json.Unmarshal(body, &nextResponse); err != nil {
		return nil, fmt.Errorf("could not unmarshal /next response: %s", err)
	}

	switch nextResponse.EventType {
	case Shutdown:
		var shutdownResponse ShutdownResponse
		err := json.Unmarshal(body, &shutdownResponse)
		return shutdownResponse, err
	default:
		return nil, fmt.Errorf("unrecognisable eventType: %s", nextResponse.EventType)
	}
}

func (client *Client) ExtensionsTelemetrySubscribe(agentIdentifier string, agentName string, body io.Reader, headers map[string][]string, remoteAddr string) (*http.Response, errorExtensions) {

	baseURL, err := url.Parse(client.baseurl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	telemetryURL := fmt.Sprintf("http://%s/2022-07-01/telemetry", baseURL.Host)

	req, err := http.NewRequest(http.MethodPut, telemetryURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry subscription request: %w", err)
	}

	req.Header.Set(lambdaAgentIdentifierHeaderKey, agentIdentifier)

	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send telemetry subscription request: %w", err)
	}

	return resp, nil
}
