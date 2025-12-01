// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
)

func getEmptyContext() context.Context {
	return context.Background()
}

func TestContextualLogger(t *testing.T) {
	tests := []struct {
		name         string
		setupContext func() context.Context
		logFunc      func(ctx context.Context, msg string, args ...any)
		message      string
		args         []any
	}{
		{
			name:         "Default_Info",
			setupContext: getEmptyContext,
			logFunc:      Info,
			message:      "info message",
			args:         []any{"key", "value"},
		},
		{
			name:         "Default_Debug",
			setupContext: getEmptyContext,
			logFunc:      Debug,
			message:      "debug message",
			args:         []any{"someData", "123"},
		},
		{
			name:         "Default_Warn",
			setupContext: getEmptyContext,
			logFunc:      Warn,
			message:      "warn message",
			args:         []any{"warning", "something"},
		},
		{
			name:         "Default_Error",
			setupContext: getEmptyContext,
			logFunc:      Error,
			message:      "error message",
			args:         []any{"error", "something failed"},
		},
		{
			name: "Context_InvokeId",
			setupContext: func() context.Context {
				return WithInvokeID(context.Background(), interop.InvokeID("12345"))
			},
			logFunc: Info,
			message: "processing request",
			args:    []any{"error", "something"},
		},
		{
			name: "Context_Multiple_Values",
			setupContext: func() context.Context {
				ctx := WithInvokeID(context.Background(), interop.InvokeID("11111"))
				return WithFields(ctx, "userID", "user-789")
			},
			logFunc: Info,
			message: "chained context logging",
			args:    []any{"error", "something"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx := tt.setupContext()
			logger := FromContext(testCtx)

			tt.logFunc(testCtx, tt.message, tt.args...)

			if testCtx == getEmptyContext() {
				assert.Equal(t, logger, slog.Default(), "Should return default logger for empty context")
			} else {
				assert.NotEqual(t, logger, slog.Default(), "Should return contextual logger, not default, when With functions are used")
			}
		})
	}
}
