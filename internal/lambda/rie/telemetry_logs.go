// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rie

import (
	"io"
	"os"

	standalonetelemetry "github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapidcore/standalone/telemetry"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/telemetry"
)

// teeLogsEgressAPI sends the runtime's and extensions' output to the console, as
// the emulator has always done, and additionally reports each line as a telemetry
// event so that subscribers can receive it.
//
// The two sources are reported separately, as the Telemetry API defines them:
// runtime output is "function" telemetry and an extension's own output is
// "extension" telemetry. Keeping them distinct is what lets an extension
// subscribe to function logs without being sent its own.
type teeLogsEgressAPI struct {
	eventsAPI *standalonetelemetry.StandaloneEventsAPI
}

func newTeeLogsEgressAPI(eventsAPI *standalonetelemetry.StandaloneEventsAPI) *teeLogsEgressAPI {
	return &teeLogsEgressAPI{eventsAPI: eventsAPI}
}

func (t *teeLogsEgressAPI) GetExtensionSockets() (io.Writer, io.Writer, error) {
	writer := t.writerFor("extension")
	return writer, writer, nil
}

func (t *teeLogsEgressAPI) GetRuntimeSockets() (io.Writer, io.Writer, error) {
	writer := t.writerFor("function")
	return writer, writer, nil
}

// writerFor tees to stdout so console output is unchanged. os.Stderr is not used
// for either source, because stderr carries the emulator's own logging.
func (t *teeLogsEgressAPI) writerFor(source string) io.Writer {
	return io.MultiWriter(os.Stdout, standalonetelemetry.NewSandboxAgentWriter(t.eventsAPI, source))
}

var _ telemetry.StdLogsEgressAPI = (*teeLogsEgressAPI)(nil)
