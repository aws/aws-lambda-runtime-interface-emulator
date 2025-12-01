// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package interop

type ResponseStatus = string

const (
	Success ResponseStatus = "success"
	Timeout ResponseStatus = "timeout"
	Error   ResponseStatus = "error"
	Failure ResponseStatus = "failure"
)
