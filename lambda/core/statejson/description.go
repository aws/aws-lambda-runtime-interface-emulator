// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package statejson

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
)

// StateDescription ...
type StateDescription struct {
	Name         string `json:"name"`
	LastModified int64  `json:"lastModified"`
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

func (s *InternalStateDescription) AsJSON() []byte {
	bytes, err := json.Marshal(s)
	if err != nil {
		log.Panicf("Failed to marshall internal states: %s", err)
	}
	return bytes
}
