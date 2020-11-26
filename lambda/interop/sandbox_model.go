// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

// Init represents an init message and is currently only used in standalone
type Init struct {
	InvokeID          string
	Handler           string
	AwsKey            string
	AwsSecret         string
	AwsSession        string
	SuppressInit      bool
	XRayDaemonAddress string // only in standalone
	FunctionName      string // only in standalone
	FunctionVersion   string // only in standalone
	CorrelationID     string // internal use only
	// TODO: define new Init type that has the Start fields as well as env vars below.
	// In standalone mode, these env vars come from test/init but from environment otherwise.
	CustomerEnvironmentVariables map[string]string
	SandboxType
}
