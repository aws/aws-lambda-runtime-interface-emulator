// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package model

// StatusResponse is a response returned by the API server,
// providing status information.
type StatusResponse struct {
	Status string `json:"status"`
}
