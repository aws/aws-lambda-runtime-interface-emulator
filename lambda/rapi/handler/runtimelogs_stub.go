// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/rapi/model"
	"go.amzn.com/lambda/rapi/rendering"
)

const (
	logsAPIDisabledErrorType      = "Logs.NotSupported"
	telemetryAPIDisabledErrorType = "Telemetry.NotSupported"
)

type runtimeTelemetryBuffering struct {
	MaxBytes  int64 `json:"maxBytes"`
	MaxItems  int   `json:"maxItems"`
	TimeoutMs int64 `json:"timeoutMs"`
}

type runtimeTelemetryDestination struct {
	URI      string `json:"URI"`
	Protocol string `json:"protocol"`
}

type runtimeTelemetryRequest struct {
	Buffering     runtimeTelemetryBuffering   `json:"buffering"`
	Destination   runtimeTelemetryDestination `json:"destination"`
	Types         []string                    `json:"types"`
	SchemaVersion string                      `json:"schemaVersion"`
}

type runtimeLogsStubAPIHandler struct{}

func (h *runtimeLogsStubAPIHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if err := rendering.RenderJSON(http.StatusAccepted, writer, request, &model.ErrorResponse{
		ErrorType:    logsAPIDisabledErrorType,
		ErrorMessage: "Logs API is not supported",
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// NewRuntimeLogsAPIStubHandler returns a new instance of http handler
// for serving /runtime/logs when a telemetry service implementation is absent
func NewRuntimeLogsAPIStubHandler() http.Handler {
	return &runtimeLogsStubAPIHandler{}
}

type runtimeTelemetryAPIStubHandler struct {
	destinations []string
	mu           sync.Mutex
}

func (h *runtimeTelemetryAPIStubHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var runtimeReq runtimeTelemetryRequest
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		log.WithError(err).Warn("Error while reading request body")
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
	err = json.Unmarshal(body, &runtimeReq)
	if err != nil {
		log.WithError(err).Warn("Error while unmarshaling request")
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
	if len(runtimeReq.Destination.URI) > 0 && runtimeReq.Destination.Protocol == "HTTP" {
		u, err := url.Parse(runtimeReq.Destination.URI)
		if err != nil {
			log.WithError(err).Warn("Error while parsing destination URL")
			http.Error(writer, err.Error(), http.StatusInternalServerError)
		}
		if sep := strings.IndexRune(u.Host, ':'); sep != -1 && u.Host[:sep] == "sandbox" {
			u.Host = "localhost" + u.Host[sep:]
		}
		h.mu.Lock()
		h.destinations = append(h.destinations, u.String())
		h.mu.Unlock()
	}
	if err := rendering.RenderJSON(http.StatusAccepted, writer, request, &model.ErrorResponse{
		ErrorType:    telemetryAPIDisabledErrorType,
		ErrorMessage: "Telemetry API is not supported",
	}); err != nil {
		log.WithError(err).Warn("Error while rendering response")
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

type logMessage struct {
	Time   string `json:"time"`
	Type   string `json:"type"`
	Record string `json:"record"`
}

// NewRuntimeTelemetryAPIStubHandler returns a new instance of http handler
// for serving /runtime/logs when a telemetry service implementation is absent
func NewRuntimeTelemetryAPIStubHandler() http.Handler {
	handler := runtimeTelemetryAPIStubHandler{}
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	go func() {
		for {
			if len(handler.destinations) > 0 {
				var msgs []logMessage
				for scanner.Scan() && len(msgs) < 10 {
					line := scanner.Text()
					originalStdout.WriteString(fmt.Sprintf("%s\n", line))
					msgs = append(msgs, logMessage{
						Time:   time.Now().Format("2006-01-02T15:04:05.999Z"),
						Type:   "function",
						Record: line,
					})
				}
				data, err := json.Marshal(msgs)
				if err != nil {
					originalStdout.WriteString(fmt.Sprintf("%s\n", err))
				}
				bodyReader := bytes.NewReader(data)
				handler.mu.Lock()
				destinations := handler.destinations
				handler.mu.Unlock()
				for _, dest := range destinations {
					resp, err := http.Post(dest, "application/json", bodyReader)
					if err != nil {
						originalStdout.WriteString(fmt.Sprintf("%s\n", err))
					}
					if resp.StatusCode > 300 {
						originalStdout.WriteString(fmt.Sprintf("failed to send logs to destination %q: status %d", dest, resp.StatusCode))
					}
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()
	return &handler
}
