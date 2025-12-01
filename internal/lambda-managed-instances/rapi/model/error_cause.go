// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"fmt"
)

const MaxErrorCauseSizeBytes = 64 << 10

type exceptionStackFrame struct {
	Path  string `json:"path,omitempty"`
	Line  int    `json:"line,omitempty"`
	Label string `json:"label,omitempty"`
}

type exception struct {
	Message string                `json:"message,omitempty"`
	Type    string                `json:"type,omitempty"`
	Stack   []exceptionStackFrame `json:"stack,omitempty"`
}

type ErrorCause struct {
	Exceptions []exception `json:"exceptions"`
	WorkingDir string      `json:"working_directory"`
	Paths      []string    `json:"paths"`
	Message    string      `json:"message,omitempty"`
}

func newErrorCause(errorCauseJSON []byte) (*ErrorCause, error) {
	var ec ErrorCause

	if err := json.Unmarshal(errorCauseJSON, &ec); err != nil {
		return nil, fmt.Errorf("failed to parse error cause JSON: %s", err)
	}

	return &ec, nil
}

func (ec *ErrorCause) isValid() bool {
	if len(ec.WorkingDir) == 0 && len(ec.Paths) == 0 && len(ec.Exceptions) == 0 && len(ec.Message) == 0 {

		return false
	}

	return true
}

func (ec *ErrorCause) croppedJSON() []byte {

	truncationFactors := []float64{0.8, 0.6, 0.4, 0.2}
	for _, factor := range truncationFactors {
		compactor := newErrorCauseCompactor(*ec)
		compactor.crop(factor)
		validErrorCauseJSON, err := json.Marshal(compactor.cause())
		if err != nil {
			break
		}

		if len(validErrorCauseJSON) <= MaxErrorCauseSizeBytes {
			return validErrorCauseJSON
		}
	}

	compactor := newErrorCauseCompactor(*ec)
	compactor.crop(0)

	validErrorCauseJSON, err := json.Marshal(compactor.cause())
	if err != nil {
		return nil
	}

	return validErrorCauseJSON
}

func ValidatedErrorCauseJSON(errorCauseJSON []byte) ([]byte, error) {
	errorCause, err := newErrorCause(errorCauseJSON)
	if err != nil {
		return nil, err
	}

	if !errorCause.isValid() {
		return nil, fmt.Errorf("error cause body has invalid format: %s", errorCauseJSON)
	}

	validErrorCauseJSON, err := json.Marshal(errorCause)
	if err != nil {
		return nil, err
	}

	if len(validErrorCauseJSON) > MaxErrorCauseSizeBytes {
		return errorCause.croppedJSON(), nil
	}

	return validErrorCauseJSON, nil
}
