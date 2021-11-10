// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

type EventsAPI interface {
	SetCurrentRequestID(requestID string)
	SendRuntimeDone(status string) error
}

type NoOpEventsAPI struct{}

func (s *NoOpEventsAPI) SetCurrentRequestID(requestID string) {}
func (s *NoOpEventsAPI) SendRuntimeDone(status string) error  { return nil }
