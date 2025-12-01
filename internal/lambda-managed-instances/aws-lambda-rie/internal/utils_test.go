// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"log/slog"
	"net/netip"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCLIArgs(t *testing.T) {

	originalArgs := os.Args
	originalLogLevel := os.Getenv("LOG_LEVEL")
	defer func() {
		os.Args = originalArgs
		require.NoError(t, os.Setenv("LOG_LEVEL", originalLogLevel))
	}()

	tests := []struct {
		name         string
		args         []string
		envLogLevel  string
		expectedOpts Options
		expectError  bool
	}{
		{
			name:        "Default values",
			args:        []string{"cmd"},
			envLogLevel: "",
			expectedOpts: Options{
				LogLevel:       "info",
				RuntimeAddress: "",
				RIEAddress:     "0.0.0.0:8080",
			},
			expectError: false,
		},
		{
			name:        "Command line arguments",
			args:        []string{"cmd", "--log-level=debug", "--runtime-api-address=127.0.0.1:8000", "--runtime-interface-emulator-address=0.0.0.0:9000"},
			envLogLevel: "",
			expectedOpts: Options{
				LogLevel:       "debug",
				RuntimeAddress: "127.0.0.1:8000",
				RIEAddress:     "0.0.0.0:9000",
			},
			expectError: false,
		},
		{
			name:        "Environment variable override",
			args:        []string{"cmd"},
			envLogLevel: "warn",
			expectedOpts: Options{
				LogLevel:       "warn",
				RuntimeAddress: "",
				RIEAddress:     "0.0.0.0:8080",
			},
			expectError: false,
		},
		{
			name:        "Command line takes precedence over env",
			args:        []string{"cmd", "--log-level=debug"},
			envLogLevel: "warn",
			expectedOpts: Options{
				LogLevel:       "debug",
				RuntimeAddress: "",
				RIEAddress:     "0.0.0.0:8080",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			os.Args = tt.args
			if tt.envLogLevel != "" {
				require.NoError(t, os.Setenv("LOG_LEVEL", tt.envLogLevel))
			} else {
				require.NoError(t, os.Unsetenv("LOG_LEVEL"))
			}

			opts, _, err := ParseCLIArgs(os.Args)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedOpts.LogLevel, opts.LogLevel)
			assert.Equal(t, tt.expectedOpts.RuntimeAddress, opts.RuntimeAddress)
			assert.Equal(t, tt.expectedOpts.RIEAddress, opts.RIEAddress)
		})
	}
}

func TestConfigureLogging(t *testing.T) {

	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	tests := []struct {
		name      string
		logLevel  string
		wantLevel slog.Level
	}{
		{
			name:      "Debug level",
			logLevel:  "debug",
			wantLevel: slog.LevelDebug,
		},
		{
			name:      "Info level",
			logLevel:  "info",
			wantLevel: slog.LevelInfo,
		},
		{
			name:      "Warn level",
			logLevel:  "warn",
			wantLevel: slog.LevelWarn,
		},
		{
			name:      "Error level",
			logLevel:  "error",
			wantLevel: slog.LevelError,
		},
		{
			name:      "Invalid level defaults to Info",
			logLevel:  "invalid",
			wantLevel: slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ConfigureLogging(tt.logLevel)

			logger := slog.Default()
			ctx := context.TODO()
			assert.True(t, logger.Enabled(ctx, tt.wantLevel),
				"Logger should have %v level enabled", tt.wantLevel)
		})
	}
}

func TestParseAddr(t *testing.T) {
	tests := []struct {
		name        string
		addrStr     string
		defaultAddr string
		want        netip.AddrPort
		wantErr     bool
	}{
		{
			name:        "Valid IPv6 address",
			addrStr:     "[::1]:8080",
			defaultAddr: "0.0.0.0:9000",
			want:        netip.AddrPortFrom(netip.MustParseAddr("::1"), 8080),
			wantErr:     false,
		},
		{
			name:        "Use default address",
			addrStr:     "",
			defaultAddr: "0.0.0.0:9000",
			want:        netip.AddrPortFrom(netip.MustParseAddr("0.0.0.0"), 9000),
			wantErr:     false,
		},
		{
			name:        "Invalid address format",
			addrStr:     "127.0.0.1",
			defaultAddr: "0.0.0.0:9000",
			want:        netip.AddrPort{},
			wantErr:     true,
		},
		{
			name:        "Invalid port",
			addrStr:     "127.0.0.1:invalid",
			defaultAddr: "0.0.0.0:9000",
			want:        netip.AddrPort{},
			wantErr:     true,
		},
		{
			name:        "Invalid IP",
			addrStr:     "invalid:8080",
			defaultAddr: "0.0.0.0:9000",
			want:        netip.AddrPort{},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAddr(tt.addrStr, tt.defaultAddr)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
