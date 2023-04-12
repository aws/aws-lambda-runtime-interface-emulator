// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package handler

const (
	// LambdaAgentIdentifier is the header key for passing agent's id
	LambdaAgentIdentifier        string = "Lambda-Extension-Identifier"
	LambdaAgentFunctionErrorType string = "Lambda-Extension-Function-Error-Type"
	// LambdaAgentName is agent name, provided by user (internal agents) or equal to executable basename (external agents)
	LambdaAgentName string = "Lambda-Extension-Name"

	ErrAgentIdentifierMissing string = "Extension.MissingExtensionIdentifier"
	ErrAgentIdentifierInvalid string = "Extension.InvalidExtensionIdentifier"

	errAgentNameInvalid        string = "Extension.InvalidExtensionName"
	errAgentRegistrationClosed string = "Extension.RegistrationClosed"
	errAgentIdentifierUnknown  string = "Extension.UnknownExtensionIdentifier"
	errAgentInvalidState       string = "Extension.InvalidExtensionState"
	errAgentMissingHeader      string = "Extension.MissingHeader"
	errTooManyExtensions       string = "Extension.TooManyExtensions"
	errInvalidEventType        string = "Extension.InvalidEventType"
	errInvalidRequestFormat    string = "InvalidRequestFormat"

	StateTransitionFailedForExtensionMessageFormat string = "State transition from %s to %s failed for extension %s. Error: %s"
	StateTransitionFailedForRuntimeMessageFormat   string = "State transition from %s to %s failed for runtime. Error: %s"
)
