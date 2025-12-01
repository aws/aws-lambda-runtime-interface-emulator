// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCustomerError(t *testing.T) {
	testCases := []struct {
		name          string
		customerError CustomerError
		expectedError string
	}{
		{
			name:          "WrapErrorIntoCustomerInvalidError",
			customerError: WrapErrorIntoCustomerInvalidError(errors.New("permission denied"), ErrorAgentPermissionDenied),
			expectedError: "PermissionDenied: permission denied",
		},
		{
			name:          "WrapErrorIntoCustomerFatalError",
			customerError: WrapErrorIntoCustomerFatalError(errors.New("runtime initialization failed"), ErrorRuntimeInit),
			expectedError: "Runtime.InitError: runtime initialization failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedError, tc.customerError.Error())
		})
	}
}
