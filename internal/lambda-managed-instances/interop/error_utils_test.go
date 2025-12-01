// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func TestBuildStatusFromError(t *testing.T) {
	testCases := []struct {
		name     string
		err      model.AppError
		expected ResponseStatus
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: Success,
		},
		{
			name:     "sandbox timeout error",
			err:      model.NewCustomerError(model.ErrorSandboxTimedout),
			expected: Timeout,
		},
		{
			name:     "customer error",
			err:      model.NewCustomerError(model.ErrorFunctionUnknown),
			expected: Error,
		},
		{
			name:     "runtime error",
			err:      model.NewPlatformError(nil, model.ErrorReasonUnknownError),
			expected: Failure,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := BuildStatusFromError(tc.err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
