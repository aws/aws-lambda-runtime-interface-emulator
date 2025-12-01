// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "errors"

const TimeFormat = "2006-01-02T15:04:05.000Z"

const (
	SubscribeSuccess   = "logs_api_subscribe_success"
	SubscribeClientErr = "logs_api_subscribe_client_err"
	SubscribeServerErr = "logs_api_subscribe_server_err"
	NumSubscribers     = "logs_api_num_subscribers"
)

var ErrTelemetryServiceOff = errors.New("ErrTelemetryServiceOff")
