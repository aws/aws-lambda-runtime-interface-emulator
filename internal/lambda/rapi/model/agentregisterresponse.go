// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

// ExtensionRegisterResponse is a response returned by the API server on extension/register post request
type ExtensionRegisterResponse struct {
	AccountID       string `json:"accountId,omitempty"`
	FunctionName    string `json:"functionName"`
	FunctionVersion string `json:"functionVersion"`
	Handler         string `json:"handler"`
}
