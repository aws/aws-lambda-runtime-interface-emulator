// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package directinvoke

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/core/bandwidthlimiter"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils/invariant"
)

const DefaultRefillIntervalMs = 125

func NewStreamedResponseWriter(w io.Writer, responseBandwidthRate int64, responseBandwidthBurstSize int64) (*bandwidthlimiter.BandwidthLimitingWriter, context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cancellableWriter := NewCancellableWriter(ctx, w)

	refillNumber := responseBandwidthRate * DefaultRefillIntervalMs / 1000
	refillInterval := DefaultRefillIntervalMs * time.Millisecond

	bucket, err := bandwidthlimiter.NewBucket(responseBandwidthBurstSize, responseBandwidthBurstSize, refillNumber, refillInterval)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	bandwidthLimitingWriter, err := bandwidthlimiter.NewBandwidthLimitingWriter(cancellableWriter, bucket)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	return bandwidthLimitingWriter, cancel, nil
}

func NewFlushingWriter(w io.Writer) *FlushingWriter {
	flusher, ok := w.(http.Flusher)
	invariant.Checkf(ok, "writer must implement http.Flusher interface")

	return &FlushingWriter{
		w:       w,
		flusher: flusher,
	}
}

type FlushingWriter struct {
	w       io.Writer
	flusher http.Flusher
}

func (w *FlushingWriter) Write(p []byte) (n int, err error) {
	n, err = w.w.Write(p)
	w.Flush()
	return n, err
}

func (w *FlushingWriter) Flush() {
	w.flusher.Flush()
}

func NewCancellableWriter(ctx context.Context, w io.Writer) io.Writer {
	return &CancellableWriter{w: w, ctx: ctx}
}

type CancellableWriter struct {
	w   io.Writer
	ctx context.Context
}

func (w *CancellableWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.w.Write(p)
}
