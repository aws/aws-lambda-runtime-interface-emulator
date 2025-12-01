// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlatformError(t *testing.T) {
	testCases := []struct {
		name          string
		platformError PlatformError
		expectedError string
	}{
		{
			name:          "WrapGoErrorIntoPlatformError",
			platformError: WrapErrorIntoPlatformFatalError(errors.New("connection has timed out"), ErrorSandboxTimedout),
			expectedError: "Sandbox.Timedout: connection has timed out",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedError, tc.platformError.Error())
		})
	}
}
