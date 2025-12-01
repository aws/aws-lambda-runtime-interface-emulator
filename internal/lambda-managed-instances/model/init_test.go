// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDurationMS_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{`1000`, 1 * time.Second, false},
		{`1500`, 1500 * time.Millisecond, false},
		{`0`, 0, false},
		{`-500`, -500 * time.Millisecond, false},
		{`"invalid"`, 0, true},
		{`null`, 0, false},
		{`123.456`, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var d DurationMS
			err := json.Unmarshal([]byte(tt.input), &d)
			if (err != nil) != tt.hasError {
				t.Errorf("input %s: unexpected error status: %v", tt.input, err)
				return
			}
			if !tt.hasError && time.Duration(d) != tt.expected {
				t.Errorf("input %s: expected %v, got %v", tt.input, tt.expected, d)
			}
		})
	}
}

func TestDurationMS_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		d       DurationMS
		want    []byte
		wantErr bool
	}{
		{
			name:    "1_second",
			d:       DurationMS(1 * time.Second),
			want:    []byte("1000"),
			wantErr: false,
		},
		{
			name:    "1_5_seconds",
			d:       DurationMS(1500 * time.Millisecond),
			want:    []byte("1500"),
			wantErr: false,
		},
		{
			name:    "zero_duration",
			d:       DurationMS(0),
			want:    []byte("0"),
			wantErr: false,
		},
		{
			name:    "negative_duration",
			d:       DurationMS(-500 * time.Millisecond),
			want:    []byte("-500"),
			wantErr: false,
		},
		{
			name:    "large_duration",
			d:       DurationMS(10 * time.Minute),
			want:    []byte("600000"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.d)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("json.Marshal() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInitRequestMessage_String(t *testing.T) {

	awsKey := "AKIAIOSFODNN7EXAMPLE"
	awsSecret := "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	awsSession := "FwoGZXIvYXdzEMj//////////wEaDM1Qz0oN8BNwV9GqyyLVAebxhwq9ZGqojXZe1UTJkzK6F9V+VZHhT5JSWYzJUKEwOqOkQyQXJpfJsYHfkJEXtR6Kh9mXnEbqKi"
	telemetryPassphrase := "hello"
	envVarKey := "CUSTOMER_ENV_VAR_1"
	envVarValue := "customer_env_value_1"

	sensitiveValues := []string{
		awsKey,
		awsSecret,
		awsSession,
		telemetryPassphrase,
		envVarKey,
		envVarValue,
	}

	tests := []struct {
		name     string
		msg      InitRequestMessage
		expected string
	}{
		{
			name: "Test Init Message",
			msg: InitRequestMessage{
				AccountID:  "123456789012",
				AwsKey:     awsKey,
				AwsSecret:  awsSecret,
				AwsSession: awsSession,
				AwsRegion:  "us-west-2",
				EnvVars: map[string]string{
					envVarKey: envVarValue,
				},
				ArtefactType:         ArtefactTypeZIP,
				MemorySizeBytes:      3008 * 1024 * 1024,
				FunctionARN:          "arn:aws:lambda:us-east-1:123456789012:function:test_function",
				FunctionVersion:      "$LATEST",
				FunctionVersionID:    "test-function-version-id",
				TaskName:             "test_function",
				InvokeTimeout:        DurationMS(3 * time.Second),
				InitTimeout:          DurationMS(10 * time.Second),
				RuntimeWorkerCount:   1,
				LogFormat:            "json",
				LogLevel:             "info",
				LogGroupName:         "/aws/lambda/test_function",
				LogStreamName:        "$LATEST",
				TelemetryAPIAddress:  TelemetryAddr(netip.MustParseAddrPort("1.1.1.1:1234")),
				TelemetryPassphrase:  telemetryPassphrase,
				XRayDaemonAddress:    "2.2.2.2:2345",
				XrayTracingMode:      XRayTracingModeActive,
				RuntimeBinaryCommand: []string{"cmd", "arg1", "arg2"},
				CurrentWorkingDir:    "/",
				AmiId:                "ami-12345",
				AvailabilityZoneId:   "az-1",
				Handler:              "lambda_function.lambda_handler",
			},
			expected: "InitRequestMessage{AccountID=123456789012, AwsRegion=us-west-2, FunctionARN=arn:aws:lambda:us-east-1:123456789012:function:test_function, FunctionVersion=$LATEST, FunctionVersionID=test-function-version-id, ArtefactType=zip, TaskName=test_function, Handler=lambda_function.lambda_handler, InvokeTimeout=3000ms, InitTimeout=10000ms, RuntimeVersion=, RuntimeArn=, RuntimeWorkerCount=1, LogFormat=json, LogLevel=info, LogGroupName=/aws/lambda/test_function, LogStreamName=$LATEST, TelemetryAPIAddress=1.1.1.1:1234, XRayDaemonAddress=2.2.2.2:2345, XrayTracingMode=Active, CurrentWorkingDir=/, AvailabilityZoneId=az-1, AmiId=ami-12345, MemorySizeBytes=3154116608, RuntimeBinaryCommand=[cmd arg1 arg2]}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.msg.String()
			if result != tt.expected {
				assert.Equal(t, tt.expected, result, "Incorrect InitRequestMessage string representation")
			}

			for _, sensitiveValue := range sensitiveValues {
				assert.NotContains(t, result, sensitiveValue, "String() output should not contain sensitive value: %s", sensitiveValue)
			}
		})
	}
}
