// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package statejson

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"
)

// ResponseMode are top-level constants used in combination with the various types of
// modes we have for responses, such as invoke's response mode and function's response mode.
// In the future we might have invoke's request mode or similar, so these help set the ground
// for consistency.
type ResponseMode string

const ResponseModeBuffered = "Buffered"
const ResponseModeStreaming = "Streaming"

type InvokeResponseMode string

const InvokeResponseModeBuffered InvokeResponseMode = ResponseModeBuffered
const InvokeResponseModeStreaming InvokeResponseMode = ResponseModeStreaming

// StateDescription ...
type StateDescription struct {
	Name           string `json:"name"`
	LastModified   int64  `json:"lastModified"`
	ResponseTimeNs int64  `json:"responseTimeNs"`
}

// RuntimeDescription ...
type RuntimeDescription struct {
	State StateDescription `json:"state"`
}

// ExtensionDescription ...
type ExtensionDescription struct {
	Name      string `json:"name"`
	ID        string
	State     StateDescription `json:"state"`
	ErrorType string           `json:"errorType"`
}

// InternalStateDescription describes internal state of runtime and extensions for debugging purposes
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

type ReleaseResponse struct {
	*InternalStateDescription
	ResponseMetrics ResponseMetrics `json:"responseMetrics"`
}

// ResetDescription describes fields of the response to an INVOKE API request
type ResetDescription struct {
	ExtensionsResetMs int64           `json:"extensionsResetMs"`
	ResponseMetrics   ResponseMetrics `json:"responseMetrics"`
}

func (s *InternalStateDescription) AsJSON() []byte {
	bytes, err := json.Marshal(s)
	if err != nil {
		log.Panicf("Failed to marshall internal states: %s", err)
	}
	return bytes
}

func (s *ResetDescription) AsJSON() []byte {
	bytes, err := json.Marshal(s)
	if err != nil {
		log.Panicf("Failed to marshall reset description: %s", err)
	}
	return bytes
}

func (s *ReleaseResponse) AsJSON() []byte {
	bytes, err := json.Marshal(s)
	if err != nil {
		log.Panicf("Failed to marshall release response: %s", err)
	}
	return bytes
}
