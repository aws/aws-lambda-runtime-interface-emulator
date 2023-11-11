// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "io"

type StandaloneLogsEgressAPI struct {
	api *StandaloneEventsAPI
}

func NewStandaloneLogsEgressAPI(api *StandaloneEventsAPI) *StandaloneLogsEgressAPI {
	return &StandaloneLogsEgressAPI{
		api: api,
	}
}

func (s *StandaloneLogsEgressAPI) GetExtensionSockets() (io.Writer, io.Writer, error) {
	w := NewSandboxAgentWriter(s.api, "extension")
	return w, w, nil
}

func (s *StandaloneLogsEgressAPI) GetRuntimeSockets() (io.Writer, io.Writer, error) {
	w := NewSandboxAgentWriter(s.api, "function")
	return w, w, nil
}
