// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

// CognitoIdentity is returned by the API server in a response headers,
// providing information about client's Cognito identity.
type CognitoIdentity struct {
	CognitoIdentityID     string `json:"cognitoIdentityId"`
	CognitoIdentityPoolID string `json:"cognitoIdentityPoolId"`
}
