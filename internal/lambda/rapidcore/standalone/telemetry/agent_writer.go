// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bufio"
	"bytes"
)

type SandboxAgentWriter struct {
	eventType string // 'runtime' or 'extension'
	eventsAPI *StandaloneEventsAPI
}

func NewSandboxAgentWriter(api *StandaloneEventsAPI, source string) *SandboxAgentWriter {
	return &SandboxAgentWriter{
		eventType: source,
		eventsAPI: api,
	}
}

func (w *SandboxAgentWriter) Write(logline []byte) (int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(logline))
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		w.eventsAPI.sendLogEvent(w.eventType, scanner.Text())
	}
	return len(logline), nil
}
