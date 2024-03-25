// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "errors"

const (
	// Metrics
	SubscribeSuccess   = "logs_api_subscribe_success"
	SubscribeClientErr = "logs_api_subscribe_client_err"
	SubscribeServerErr = "logs_api_subscribe_server_err"
	NumSubscribers     = "logs_api_num_subscribers"
)

// ErrTelemetryServiceOff returned on attempt to subscribe after telemetry service has been turned off.
var ErrTelemetryServiceOff = errors.New("ErrTelemetryServiceOff")
