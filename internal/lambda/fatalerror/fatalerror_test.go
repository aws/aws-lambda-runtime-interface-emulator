// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package fatalerror

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidRuntimeAndFunctionErrors(t *testing.T) {
	type test struct {
		input    string
		expected ErrorType
	}

	var tests = []test{}
	for validError := range validRuntimeAndFunctionErrors {
		tests = append(tests, test{input: string(validError), expected: validError})
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, GetValidRuntimeOrFunctionErrorType(tt.input), tt.expected)
		})
	}
}

func TestGetValidRuntimeOrFunctionErrorType(t *testing.T) {
	type test struct {
		input    string
		expected ErrorType
	}

	var tests = []test{
		{"", RuntimeUnknown},
		{"MyCustomError", RuntimeUnknown},
		{"MyCustomError.Error", RuntimeUnknown},
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
