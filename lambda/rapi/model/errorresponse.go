// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"go.amzn.com/lambda/interop"
)

// ErrorResponse is a standard invoke error response,
// providing information about the error.
type ErrorResponse struct {
	ErrorMessage string   `json:"errorMessage"`
	ErrorType    string   `json:"errorType"`
	StackTrace   []string `json:"stackTrace,omitempty"`
}

func (s *ErrorResponse) AsInteropError() *interop.ErrorResponse {
	respJSON, err := json.Marshal(s)
	if err != nil {
		log.Panicf("Failed to marshal %#v: %s", *s, err)
	}

	return &interop.ErrorResponse{
		ErrorType:    s.ErrorType,
		ErrorMessage: s.ErrorMessage,
		Payload:      respJSON,
	}
}
