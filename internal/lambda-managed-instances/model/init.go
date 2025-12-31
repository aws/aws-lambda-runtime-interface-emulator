// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"time"
)

type InitRequestMessage struct {
	AccountID            string          `json:"account_id"`
	AwsKey               string          `json:"aws_key"`
	AwsSecret            string          `json:"aws_secret"`
	AwsSession           string          `json:"aws_session"`
	AwsRegion            string          `json:"aws_region"`
	EnvVars              KVMap           `json:"env_vars"`
	MemorySizeBytes      int             `json:"ram_limit"`
	FunctionARN          string          `json:"function_arn"`
	FunctionVersion      string          `json:"function_version"`
	FunctionVersionID    string          `json:"version_id"`
	ArtefactType         ArtefactType    `json:"artefact_type"`
	TaskName             string          `json:"task_name"`
	Handler              string          `json:"handler,omitempty"`
	InvokeTimeout        DurationMS      `json:"invoke_timeout_ms"`
	InitTimeout          DurationMS      `json:"init_timeout_ms"`
	RuntimeVersion       string          `json:"runtime_version,omitempty"`
	RuntimeArn           string          `json:"runtime_arn,omitempty"`
	RuntimeWorkerCount   int             `json:"runtime_worker_count"`
	LogFormat            string          `json:"log_format"`
	LogLevel             string          `json:"log_level"`
	LogGroupName         string          `json:"log_group_name"`
	LogStreamName        string          `json:"log_stream_name"`
	TelemetryAPIAddress  TelemetryAddr   `json:"telemetry_api_address"`
	TelemetryPassphrase  string          `json:"telemetry_passphrase"`
	XRayDaemonAddress    string          `json:"xray_daemon_address"`
	XrayTracingMode      XrayTracingMode `json:"xray_tracing_mode"`
	CurrentWorkingDir    string          `json:"cwd"`
	RuntimeBinaryCommand []string        `json:"cmd"`

	AvailabilityZoneId string `json:"aws_availability_zone_id"`

	AmiId string `json:"ami_id"`
}

type DurationMS time.Duration

func (d *DurationMS) UnmarshalJSON(b []byte) error {
	var ms int
	if err := json.Unmarshal(b, &ms); err != nil {
		return err
	}
	*d = DurationMS(ms * int(time.Millisecond))
	return nil
}

func (d DurationMS) MarshalJSON() ([]byte, error) {
	ms := time.Duration(d).Milliseconds()
	return json.Marshal(ms)
}

type KVSlice []string

type KVMap map[string]string

type TelemetryAddr netip.AddrPort

func (t *TelemetryAddr) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	addrPort, err := netip.ParseAddrPort(s)
	if err != nil {
		return err
	}
	*t = TelemetryAddr(addrPort)
	return nil
}

func (t TelemetryAddr) MarshalJSON() ([]byte, error) {
	return json.Marshal(netip.AddrPort(t).String())
}

func (i InitRequestMessage) String() string {
	return fmt.Sprintf("InitRequestMessage{"+
		"AccountID=%s, "+
		"AwsRegion=%s, "+
		"FunctionARN=%s, "+
		"FunctionVersion=%s, "+
		"FunctionVersionID=%s, "+
		"ArtefactType=%s, "+
		"TaskName=%s, "+
		"Handler=%s, "+
		"InvokeTimeout=%dms, "+
		"InitTimeout=%dms, "+
		"RuntimeVersion=%s, "+
		"RuntimeArn=%s, "+
		"RuntimeWorkerCount=%d, "+
		"LogFormat=%s, "+
		"LogLevel=%s, "+
		"LogGroupName=%s, "+
		"LogStreamName=%s, "+
		"TelemetryAPIAddress=%s, "+
		"XRayDaemonAddress=%s, "+
		"XrayTracingMode=%s, "+
		"CurrentWorkingDir=%s, "+
		"AvailabilityZoneId=%s, "+
		"AmiId=%s, "+
		"MemorySizeBytes=%d, "+
		"RuntimeBinaryCommand=%v"+
		"}",
		i.AccountID,
		i.AwsRegion,
		i.FunctionARN,
		i.FunctionVersion,
		i.FunctionVersionID,
		i.ArtefactType,
		i.TaskName,
		i.Handler,
		time.Duration(i.InvokeTimeout).Milliseconds(),
		time.Duration(i.InitTimeout).Milliseconds(),
		i.RuntimeVersion,
		i.RuntimeArn,
		i.RuntimeWorkerCount,
		i.LogFormat,
		i.LogLevel,
		i.LogGroupName,
		i.LogStreamName,
		netip.AddrPort(i.TelemetryAPIAddress).String(),
		i.XRayDaemonAddress,
		i.XrayTracingMode,
		i.CurrentWorkingDir,
		i.AvailabilityZoneId,
		i.AmiId,
		i.MemorySizeBytes,
		i.RuntimeBinaryCommand,
	)
}
