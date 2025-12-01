// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"io"
	"os"
)

type StdLogsEgressAPI interface {
	GetExtensionSockets() (io.Writer, io.Writer, error)
	GetRuntimeSockets() (io.Writer, io.Writer, error)
}

type NoOpLogsEgressAPI struct{}

func (s *NoOpLogsEgressAPI) GetExtensionSockets() (io.Writer, io.Writer, error) {

	return os.Stdout, os.Stdout, nil
}

func (s *NoOpLogsEgressAPI) GetRuntimeSockets() (io.Writer, io.Writer, error) {

	return os.Stdout, os.Stdout, nil
}

var _ StdLogsEgressAPI = (*NoOpLogsEgressAPI)(nil)
