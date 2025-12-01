// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

const (
	LambdaAgentIdentifier     string = "Lambda-Extension-Identifier"
	ErrAgentIdentifierMissing string = "Extension.MissingExtensionIdentifier"
	ErrAgentIdentifierInvalid string = "Extension.InvalidExtensionIdentifier"
)

type CtxKey int

const (
	AgentIDCtxKey CtxKey = iota
)
