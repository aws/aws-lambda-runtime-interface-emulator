// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/aws-lambda-rie/internal/telemetry/internal"
)

type LogsEgress struct {
	eventRelay relay
	w          io.Writer
}

func NewLogsEgress(eventRelay relay, w io.Writer) *LogsEgress {
	return &LogsEgress{
		eventRelay: eventRelay,
		w:          w,
	}
}

func (e *LogsEgress) startWriter(category internal.EventCategory, typ internal.EventType) io.Writer {
	pipeReader, pipeWriter := io.Pipe()
	scanner := bufio.NewScanner(pipeReader)

	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			_, _ = fmt.Fprintln(e.w, line)
			e.eventRelay.broadcast(line, category, typ)
		}
		if err := scanner.Err(); err != nil {
			slog.Error("scanner failed", "err", err)
		} else {
			slog.Debug("log scanner reached EOF", "category", category, "type", typ)
		}
	}()

	return pipeWriter
}

func (e *LogsEgress) GetExtensionSockets() (io.Writer, io.Writer, error) {
	stdout := e.startWriter(internal.CategoryExtension, internal.TypeExtension)
	stderr := e.startWriter(internal.CategoryExtension, internal.TypeExtension)
	return stdout, stderr, nil
}

func (e *LogsEgress) GetRuntimeSockets() (io.Writer, io.Writer, error) {
	stdout := e.startWriter(internal.CategoryFunction, internal.TypeFunction)
	stderr := e.startWriter(internal.CategoryFunction, internal.TypeFunction)
	return stdout, stderr, nil
}

type relay interface {
	broadcast(record any, category internal.EventCategory, typ internal.EventType)
	flush(ctx context.Context)
}
