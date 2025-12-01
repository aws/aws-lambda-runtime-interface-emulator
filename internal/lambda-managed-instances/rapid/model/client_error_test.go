// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientError(t *testing.T) {
	testCases := []struct {
		name          string
		clientError   ClientError
		expectedError string
	}{
		{
			name:          "New Client Error",
			clientError:   NewClientError(errors.New("Invalid Request"), ErrorSeverityFatal, ErrorInvalidRequest),
			expectedError: "InvalidRequest: Invalid Request",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedError, tc.clientError.Error())
			assert.Equal(t, tc.clientError.Severity(), ErrorSeverityFatal)
			assert.Equal(t, tc.clientError.Source(), ErrorSourceClient)
		})
	}
}
