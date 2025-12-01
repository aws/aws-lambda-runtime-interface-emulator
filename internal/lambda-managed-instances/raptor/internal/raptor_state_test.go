// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStateGuard(t *testing.T) {
	sg := NewStateGuard()
	assert.Equal(t, Idle, sg.current)
}

func TestGetState(t *testing.T) {
	sg := NewStateGuard()
	assert.Equal(t, Idle, sg.GetState())

	sg.current = Initialized
	assert.Equal(t, Initialized, sg.GetState())
}

func TestSetState(t *testing.T) {
	tests := []struct {
		name          string
		initialState  Status
		targetState   Status
		expectError   bool
		expectedState Status
	}{
		{
			name:          "Valid: Idle to Initializing",
			initialState:  Idle,
			targetState:   Initializing,
			expectError:   false,
			expectedState: Initializing,
		},
		{
			name:          "Valid: Idle to ShuttingDown",
			initialState:  Idle,
			targetState:   ShuttingDown,
			expectError:   false,
			expectedState: ShuttingDown,
		},
		{
			name:          "Valid: Initializing to Initialized",
			initialState:  Initializing,
			targetState:   Initialized,
			expectError:   false,
			expectedState: Initialized,
		},
		{
			name:          "Valid: Initializing to ShuttingDown",
			initialState:  Initializing,
			targetState:   ShuttingDown,
			expectError:   false,
			expectedState: ShuttingDown,
		},
		{
			name:          "Valid: Initialized to ShuttingDown",
			initialState:  Initialized,
			targetState:   ShuttingDown,
			expectError:   false,
			expectedState: ShuttingDown,
		},
		{
			name:          "Valid: ShuttingDown to Shutdown",
			initialState:  ShuttingDown,
			targetState:   Shutdown,
			expectError:   false,
			expectedState: Shutdown,
		},
		{
			name:          "Invalid: Idle to Initialized",
			initialState:  Idle,
			targetState:   Initialized,
			expectError:   true,
			expectedState: Idle,
		},
		{
			name:          "Invalid: Idle to Shutdown",
			initialState:  Idle,
			targetState:   Shutdown,
			expectError:   true,
			expectedState: Idle,
		},
		{
			name:          "Invalid: Initializing to Idle",
			initialState:  Initializing,
			targetState:   Idle,
			expectError:   false,
			expectedState: Idle,
		},
		{
			name:          "Invalid: Initialized to Idle",
			initialState:  Initialized,
			targetState:   Idle,
			expectError:   false,
			expectedState: Idle,
		},
		{
			name:          "Invalid: Initialized to Initializing",
			initialState:  Initialized,
			targetState:   Initializing,
			expectError:   true,
			expectedState: Initialized,
		},
		{
			name:          "Invalid: ShuttingDown to Idle",
			initialState:  ShuttingDown,
			targetState:   Idle,
			expectError:   false,
			expectedState: Idle,
		},
		{
			name:          "Valid: Shutdown to Idle",
			initialState:  Shutdown,
			targetState:   Idle,
			expectError:   false,
			expectedState: Idle,
		},
		{
			name:          "Invalid: Shutdown to other states",
			initialState:  Shutdown,
			targetState:   Initializing,
			expectError:   true,
			expectedState: Shutdown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sg := &StateGuard{current: tt.initialState}
			err := sg.SetState(tt.targetState)

			if tt.expectError {
				assert.ErrorIs(t, err, ErrInvalidTransition)
				assert.Equal(t, tt.expectedState, sg.current)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedState, sg.current)
			}
		})
	}
}

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     Status
		to       Status
		expected bool
	}{

		{"Idle to Initializing", Idle, Initializing, true},
		{"Idle to ShuttingDown", Idle, ShuttingDown, true},
		{"Initializing to Initialized", Initializing, Initialized, true},
		{"Initializing to ShuttingDown", Initializing, ShuttingDown, true},
		{"Initialized to ShuttingDown", Initialized, ShuttingDown, true},
		{"ShuttingDown to Shutdown", ShuttingDown, Shutdown, true},

		{"Idle to Initialized", Idle, Initialized, false},
		{"Idle to Shutdown", Idle, Shutdown, false},
		{"Initializing to Idle", Initializing, Idle, true},
		{"Initialized to Idle", Initialized, Idle, true},
		{"Initialized to Initializing", Initialized, Initializing, false},
		{"ShuttingDown to Idle", ShuttingDown, Idle, true},
		{"ShuttingDown to Initializing", ShuttingDown, Initializing, false},
		{"ShuttingDown to Initialized", ShuttingDown, Initialized, false},
		{"Shutdown to Idle", Shutdown, Idle, true},
		{"Shutdown to Initializing", Shutdown, Initializing, false},
		{"Shutdown to Initialized", Shutdown, Initialized, false},
		{"Shutdown to ShuttingDown", Shutdown, ShuttingDown, false},

		{"Idle to Idle", Idle, Idle, true},
		{"Initializing to Initializing", Initializing, Initializing, false},
		{"Initialized to Initialized", Initialized, Initialized, false},
		{"ShuttingDown to ShuttingDown", ShuttingDown, ShuttingDown, false},
		{"Shutdown to Shutdown", Shutdown, Shutdown, false},

		{"Invalid state to Idle", Status(99), Idle, true},
		{"Idle to invalid state", Idle, Status(99), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidTransition(tt.from, tt.to)
			assert.Equal(t, tt.expected, result, "isValidTransition(%v, %v) should return %v", tt.from, tt.to, tt.expected)
		})
	}
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected string
	}{
		{
			name:     "Idle",
			status:   Idle,
			expected: "Idle",
		},
		{
			name:     "initializing_status",
			status:   Initializing,
			expected: "Initializing",
		},
		{
			name:     "initialized_status",
			status:   Initialized,
			expected: "Initialized",
		},
		{
			name:     "shutting_down_status",
			status:   ShuttingDown,
			expected: "ShuttingDown",
		},
		{
			name:     "shutdown_status",
			status:   Shutdown,
			expected: "Shutdown",
		},
		{
			name:     "unknown_status",
			status:   Status(99),
			expected: "Status(99)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.String()
			assert.Equal(t, tt.expected, result, "Status.String() should return correct string representation")
		})
	}
}
