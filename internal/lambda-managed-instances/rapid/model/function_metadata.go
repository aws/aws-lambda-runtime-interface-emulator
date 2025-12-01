// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

type FunctionMetadata struct {
	AccountID       string
	FunctionName    string
	FunctionVersion string
	MemorySizeBytes uint64
	Handler         string
	RuntimeInfo     RuntimeInfo
}

type RuntimeInfo struct {
	Arn     string
	Version string
}

type Credentials struct {
	AwsKey     string
	AwsSecret  string
	AwsSession string
}
