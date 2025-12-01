// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

type CognitoIdentity struct {
	CognitoIdentityID     string `json:"cognitoIdentityId"`
	CognitoIdentityPoolID string `json:"cognitoIdentityPoolId"`
}
