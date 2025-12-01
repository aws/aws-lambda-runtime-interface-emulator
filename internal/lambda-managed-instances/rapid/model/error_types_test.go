// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetValidRuntimeOrFunctionErrorType(t *testing.T) {
	type test struct {
		input    string
		expected ErrorType
	}

	tests := []test{
		{"", ErrorRuntimeUnknown},
		{"MyCustomError", ErrorRuntimeUnknown},
		{"MyCustomError.Error", ErrorRuntimeUnknown},
		{"Runtime.MyCustomErrorTypeHere", ErrorType("Runtime.MyCustomErrorTypeHere")},
		{"Function.MyCustomErrorTypeHere", ErrorType("Function.MyCustomErrorTypeHere")},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("TestGetValidRuntimeOrFunctionErrorType with %s", tt.input)
		t.Run(testname, func(t *testing.T) {
			assert.Equal(t, GetValidRuntimeOrFunctionErrorType(tt.input), tt.expected)
		})
	}
}

func TestGetValidExtensionErrorType(t *testing.T) {
	type test struct {
		input    string
		expected ErrorType
	}

	defaultErrorType := ErrorAgentExit

	tests := []test{
		{"", defaultErrorType},
		{"MyCustomError", defaultErrorType},
		{"MyCustomError.Error", defaultErrorType},
		{"Runtime.MyCustomErrorTypeHere", defaultErrorType},
		{"Function.MyCustomErrorTypeHere", defaultErrorType},
		{"Extension.", defaultErrorType},
		{"Extension.A", defaultErrorType},
		{"Extension.az", defaultErrorType},
		{"Extension.AA", ErrorType("Extension.AA")},
		{"Extension.Az", ErrorType("Extension.Az")},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("TestGetValidExtensionErrorType with %s", tt.input)
		t.Run(testname, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetValidExtensionErrorType(tt.input, defaultErrorType))
		})
	}
}
