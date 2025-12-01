// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package statejson

import (
	"encoding/json"
	"log/slog"
	"time"
)

type ResponseMode string

const (
	ResponseModeBuffered  = "Buffered"
	ResponseModeStreaming = "Streaming"
)

type InvokeResponseMode string

const (
	InvokeResponseModeBuffered  InvokeResponseMode = ResponseModeBuffered
	InvokeResponseModeStreaming InvokeResponseMode = ResponseModeStreaming
)

type StateDescription struct {
	Name         string    `json:"name"`
	LastModified time.Time `json:"lastModified"`
	ResponseTime time.Time `json:"responseTime"`
}

type RuntimeDescription struct {
	State StateDescription `json:"state"`
}

type ExtensionDescription struct {
	Name      string `json:"name"`
	ID        string
	State     StateDescription `json:"state"`
	ErrorType string           `json:"errorType"`
}

type InternalStateDescription struct {
	Runtime         *RuntimeDescription    `json:"runtime"`
	Extensions      []ExtensionDescription `json:"extensions"`
	FirstFatalError string                 `json:"firstFatalError"`
}

type ResponseMetricsDimensions struct {
	InvokeResponseMode InvokeResponseMode `json:"invokeResponseMode"`
}

type ResponseMetrics struct {
	RuntimeResponseLatencyMs float64                   `json:"runtimeResponseLatencyMs"`
	Dimensions               ResponseMetricsDimensions `json:"dimensions"`
}

func (s *InternalStateDescription) AsJSON() []byte {
	bytes, err := json.Marshal(s)
	if err != nil {
		slog.Error("Failed to marshall internal states", "err", err)
		panic(err)
	}
	return bytes
}
