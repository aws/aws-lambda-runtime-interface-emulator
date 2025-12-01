// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	intmodel "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/model"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapidcore/env"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils"
)

func Test_getInitRequestMessage(t *testing.T) {

	for k := range env.Defined {
		require.NoError(t, os.Unsetenv(k))
	}

	tests := []struct {
		name string
		args []string
		env  map[string]string
		want intmodel.InitRequestMessage
	}{
		{
			name: "default_values",
			args: []string{"aws-lambda-rie", "/path/to/bootstrap"},
			env:  map[string]string{},
			want: intmodel.InitRequestMessage{
				AccountID:            "123456789012",
				AwsKey:               "",
				AwsSecret:            "",
				AwsSession:           "",
				AwsRegion:            "us-east-1",
				EnvVars:              map[string]string{},
				MemorySizeBytes:      3008 * 1024 * 1024,
				FunctionARN:          "arn:aws:lambda:us-east-1:123456789012:function:test_function",
				FunctionVersion:      "$LATEST",
				FunctionVersionID:    "",
				ArtefactType:         intmodel.ArtefactTypeZIP,
				TaskName:             "test_function",
				Handler:              "",
				InvokeTimeout:        intmodel.DurationMS(300 * time.Second),
				InitTimeout:          intmodel.DurationMS(300 * time.Second),
				RuntimeVersion:       "",
				RuntimeArn:           "",
				RuntimeWorkerCount:   1,
				LogFormat:            "json",
				LogLevel:             "",
				LogGroupName:         "/aws/lambda/Functions",
				LogStreamName:        "$LATEST",
				TelemetryAPIAddress:  intmodel.TelemetryAddr(netip.MustParseAddrPort("127.0.0.1:0")),
				TelemetryPassphrase:  "",
				XRayDaemonAddress:    "",
				XrayTracingMode:      intmodel.XRayTracingModePassThrough,
				CurrentWorkingDir:    "REPLACE",
				RuntimeBinaryCommand: []string{"/path/to/bootstrap"},
				AvailabilityZoneId:   "",
				AmiId:                "",
			},
		},
		{
			name: "all_env_vars_and_args",
			args: []string{"app", "/custom/bootstrap", "custom_handler"},
			env: map[string]string{
				"AWS_ACCOUNT_ID":                  "987654321098",
				"AWS_ACCESS_KEY_ID":               "test_key",
				"AWS_SECRET_ACCESS_KEY":           "test_secret",
				"AWS_SESSION_TOKEN":               "test_session",
				"AWS_REGION":                      "eu-west-1",
				"AWS_LAMBDA_FUNCTION_MEMORY_SIZE": "1024",
				"AWS_LAMBDA_FUNCTION_NAME":        "custom_function",
				"AWS_LAMBDA_FUNCTION_VERSION":     "2",
				"AWS_LAMBDA_FUNCTION_TIMEOUT":     "60",
				"AWS_LAMBDA_MAX_CONCURRENCY":      "5",
				"AWS_LAMBDA_LOG_FORMAT":           "JSON",
				"AWS_LAMBDA_LOG_LEVEL":            "DEBUG",
				"AWS_LAMBDA_LOG_GROUP_NAME":       "/aws/lambda/custom",
				"AWS_LAMBDA_LOG_STREAM_NAME":      "custom-stream",
				"AWS_LAMBDA_FUNCTION_HANDLER":     "custom_handler_from_env",
				"_HANDLER":                        "lower_priority_custom_handler_from_env",
			},
			want: intmodel.InitRequestMessage{
				AccountID:            "987654321098",
				AwsKey:               "test_key",
				AwsSecret:            "test_secret",
				AwsSession:           "test_session",
				AwsRegion:            "eu-west-1",
				EnvVars:              map[string]string{},
				MemorySizeBytes:      1024 * 1024 * 1024,
				FunctionARN:          "arn:aws:lambda:eu-west-1:987654321098:function:custom_function",
				FunctionVersion:      "2",
				FunctionVersionID:    "",
				ArtefactType:         intmodel.ArtefactTypeZIP,
				TaskName:             "custom_function",
				Handler:              "custom_handler_from_env",
				InvokeTimeout:        intmodel.DurationMS(60 * time.Second),
				InitTimeout:          intmodel.DurationMS(60 * time.Second),
				RuntimeVersion:       "",
				RuntimeArn:           "",
				RuntimeWorkerCount:   5,
				LogFormat:            "JSON",
				LogLevel:             "DEBUG",
				LogGroupName:         "/aws/lambda/custom",
				LogStreamName:        "custom-stream",
				TelemetryAPIAddress:  intmodel.TelemetryAddr(netip.MustParseAddrPort("127.0.0.1:0")),
				TelemetryPassphrase:  "",
				XRayDaemonAddress:    "",
				XrayTracingMode:      intmodel.XRayTracingModePassThrough,
				CurrentWorkingDir:    "/var/task",
				RuntimeBinaryCommand: []string{"/custom/bootstrap", "custom_handler"},
				AvailabilityZoneId:   "",
				AmiId:                "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			tt.want.EnvVars = env.KVPairStringsToMap(os.Environ())
			cwd, err := os.Getwd()
			require.NoError(t, err)
			tt.want.CurrentWorkingDir = cwd

			got, err := GetInitRequestMessage(&utils.MockFileUtil{}, tt.args)

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_getBootstrap(t *testing.T) {
	type args struct {
		fileUtil utils.FileUtil
		args     []string
		cwd      string
	}
	tests := []struct {
		name    string
		args    args
		wantCmd []string
		wantErr bool
	}{
		{
			name: "with_bootstrap_arg",
			args: args{
				fileUtil: &utils.MockFileUtil{},
				args:     []string{"aws-lambda-rie", "/path/to/bootstrap", "handler"},
				cwd:      "/test/cwd",
			},
			wantCmd: []string{"/path/to/bootstrap", "handler"},
			wantErr: false,
		},
		{
			name: "find_bootstrap_in_cwd",
			args: args{
				fileUtil: func() *utils.MockFileUtil {
					m := &utils.MockFileUtil{}
					mockInfo := utils.NewMockFileInfo()
					mockInfo.On("IsDir").Return(false)
					m.On("Stat", "/test/cwd/bootstrap").Return(mockInfo, nil)
					m.On("IsNotExist", nil).Return(false)
					return m
				}(),
				args: []string{"aws-lambda-rie"},
				cwd:  "/test/cwd",
			},
			wantCmd: []string{"/test/cwd/bootstrap"},
			wantErr: false,
		},
		{
			name: "find_bootstrap_in_var_runtime",
			args: args{
				fileUtil: func() *utils.MockFileUtil {
					m := &utils.MockFileUtil{}

					m.On("Stat", "/test/cwd/bootstrap").Return(nil, os.ErrNotExist)
					m.On("IsNotExist", os.ErrNotExist).Return(true)

					mockInfo := utils.NewMockFileInfo()
					mockInfo.On("IsDir").Return(false)
					m.On("Stat", "/var/runtime/bootstrap").Return(mockInfo, nil)
					m.On("IsNotExist", nil).Return(false)
					return m
				}(),
				args: []string{"aws-lambda-rie"},
				cwd:  "/test/cwd",
			},
			wantCmd: []string{"/var/runtime/bootstrap"},
			wantErr: false,
		},
		{
			name: "bootstrap_is_directory",
			args: args{
				fileUtil: func() *utils.MockFileUtil {
					m := &utils.MockFileUtil{}
					mockInfo := utils.NewMockFileInfo()
					mockInfo.On("IsDir").Return(true)
					m.On("Stat", "/test/cwd/bootstrap").Return(mockInfo, nil)
					m.On("IsNotExist", nil).Return(false)

					m.On("Stat", "/var/runtime/bootstrap").Return(nil, os.ErrNotExist)
					m.On("Stat", "/var/task/bootstrap").Return(nil, os.ErrNotExist)
					m.On("Stat", "/opt/bootstrap").Return(nil, os.ErrNotExist)
					m.On("IsNotExist", os.ErrNotExist).Return(true)
					return m
				}(),
				args: []string{"aws-lambda-rie"},
				cwd:  "/test/cwd",
			},
			wantCmd: nil,
			wantErr: true,
		},
		{
			name: "no_bootstrap_found",
			args: args{
				fileUtil: func() *utils.MockFileUtil {
					m := &utils.MockFileUtil{}
					m.On("Stat", "/test/cwd/bootstrap").Return(nil, os.ErrNotExist)
					m.On("Stat", "/var/runtime/bootstrap").Return(nil, os.ErrNotExist)
					m.On("Stat", "/var/task/bootstrap").Return(nil, os.ErrNotExist)
					m.On("Stat", "/opt/bootstrap").Return(nil, os.ErrNotExist)
					m.On("IsNotExist", os.ErrNotExist).Return(true)
					return m
				}(),
				args: []string{"aws-lambda-rie"},
				cwd:  "/test/cwd",
			},
			wantCmd: nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCmd, gotErr := getBootstrap(tt.args.fileUtil, tt.args.args, tt.args.cwd)
			if tt.wantErr {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)
			}
			assert.Equal(t, tt.wantCmd, gotCmd)
		})
	}
}

func Test_getHandler(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		envVars map[string]string
		want    string
	}{
		{
			name: "AWS_LAMBDA_FUNCTION_HANDLER_takes_precedence",
			args: []string{"aws-lambda-rie", "/path/to/bootstrap", "handler_from_args"},
			envVars: map[string]string{
				"AWS_LAMBDA_FUNCTION_HANDLER": "handler_from_aws_lambda_function_handler",
				"_HANDLER":                    "handler_from__handler",
			},
			want: "handler_from_aws_lambda_function_handler",
		},
		{
			name: "_HANDLER_takes_precedence",
			args: []string{"aws-lambda-rie", "/path/to/bootstrap", "handler_from_args"},
			envVars: map[string]string{
				"_HANDLER": "handler_from__handler",
			},
			want: "handler_from__handler",
		},
		{
			name:    "handler_from_args",
			args:    []string{"aws-lambda-rie", "/path/to/bootstrap", "handler_from_args"},
			envVars: map[string]string{},
			want:    "handler_from_args",
		},

		{
			name:    "no_handler_specified",
			args:    []string{"aws-lambda-rie", "/path/to/bootstrap"},
			envVars: map[string]string{},
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			assert.Equalf(t, tt.want, getHandler(tt.args), "getHandler(%v)", tt.args)
		})
	}
}
