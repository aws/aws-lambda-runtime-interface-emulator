// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"io"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/logging"
)

type TimedReader struct {
	Reader        io.Reader
	Name          string
	Ctx           context.Context
	TotalDuration time.Duration
}

func (tr *TimedReader) Read(p []byte) (n int, err error) {
	start := time.Now()
	n, err = tr.Reader.Read(p)
	duration := time.Since(start)
	tr.TotalDuration += duration

	logging.Debug(tr.Ctx, "Read operation completed",
		"name", tr.Name,
		"bytes", n,
		"duration", duration,
		"totalReadTime", tr.TotalDuration,
		"error", err)

	return n, err
}

type TimedWriter struct {
	Writer        io.Writer
	Name          string
	Ctx           context.Context
	TotalDuration time.Duration
}

func (tw *TimedWriter) Write(p []byte) (n int, err error) {
	start := time.Now()
	n, err = tw.Writer.Write(p)
	duration := time.Since(start)
	tw.TotalDuration += duration

	logging.Debug(tw.Ctx, "Write operation completed",
		"name", tw.Name,
		"bytes", n,
		"duration", duration,
		"totalWriteDuration", tw.TotalDuration,
		"error", err)

	return n, err
}
