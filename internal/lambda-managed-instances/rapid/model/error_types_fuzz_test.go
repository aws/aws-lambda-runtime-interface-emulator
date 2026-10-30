// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"strings"
	"testing"
)

func isStrictErrorType(prefix, s string) bool {
	rest, ok := strings.CutPrefix(s, prefix+".")
	if !ok || len(rest) < 2 {
		return false
	}
	if rest[0] < 'A' || rest[0] > 'Z' {
		return false
	}
	for i := 1; i < len(rest); i++ {
		c := rest[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return true
}

func FuzzGetValidRuntimeOrFunctionErrorType(f *testing.F) {
	seeds := []string{
		"",
		"Runtime.MyError",
		"Function.MyError",
		"Sandbox.Failure Runtime.Ab",
		"Runtime.MyError\n",
		"Runtime.My🚀Error",
		" Function.MyError ",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		got := GetValidRuntimeOrFunctionErrorType(input)
		switch got {
		case ErrorRuntimeUnknown, ErrorFunctionUnknown:

		case ErrorType(input):
			if !isStrictErrorType("Runtime", input) && !isStrictErrorType("Function", input) {
				t.Errorf("accepted malformed input %q", input)
			}
		default:
			t.Errorf("returned %q which is neither a fallback nor the input %q", got, input)
		}
	})
}

func FuzzGetValidExtensionErrorType(f *testing.F) {
	seeds := []string{
		"",
		"Extension.AA",
		"Sandbox.Failure Extension.AA",
		"Extension.AA\n",
		"Extension.A🚀",
		" Extension.AA ",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		got := GetValidExtensionErrorType(input, ErrorAgentExit)
		switch got {
		case ErrorAgentExit:

		case ErrorType(input):
			if !isStrictErrorType("Extension", input) {
				t.Errorf("accepted malformed input %q", input)
			}
		default:
			t.Errorf("returned %q which is neither the default nor the input %q", got, input)
		}
	})
}
