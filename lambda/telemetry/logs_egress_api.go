// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"io"
	"os"
)

type LogsEgressAPI interface {
	GetExtensionSockets() (io.Writer, io.Writer, error)
	GetRuntimeSockets() (io.Writer, io.Writer, error)
}

type NoOpLogsEgressAPI struct{}

func (s *NoOpLogsEgressAPI) GetExtensionSockets() (io.Writer, io.Writer, error) {
	// os.Stderr can not be used for the stderrWriter because stderr is for internal logging (not customer visible).
	return os.Stdout, os.Stdout, nil
}

func (s *NoOpLogsEgressAPI) GetRuntimeSockets() (io.Writer, io.Writer, error) {
	// os.Stderr can not be used for the stderrWriter because stderr is for internal logging (not customer visible).
	return os.Stdout, os.Stdout, nil
}
