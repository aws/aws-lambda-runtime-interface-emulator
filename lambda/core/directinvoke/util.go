// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package directinvoke

import (
	"context"
	"errors"
	"go.amzn.com/lambda/core/bandwidthlimiter"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

const DefaultRefillIntervalMs = 125 // default refill interval in milliseconds

func NewStreamedResponseWriter(w http.ResponseWriter) (*bandwidthlimiter.BandwidthLimitingWriter, context.CancelFunc, error) {
	flushingWriter, err := NewFlushingWriter(w) // after writing a chunk we have to flush it to avoid additional buffering by ResponseWriter
	if err != nil {
		return nil, nil, err
	}
	cancellableWriter, cancel := NewCancellableWriter(flushingWriter) // cancelling prevents next calls to Write() from happening

	refillNumber := ResponseBandwidthRate * DefaultRefillIntervalMs / 1000 // refillNumber is calculated based on 'ResponseBandwidthRate' and bucket refill interval
	refillInterval := DefaultRefillIntervalMs * time.Millisecond

	// Initial bucket for token bucket algorithm allows for a burst of up to 6 MiB, and an average transmission rate of 2 MiB/s
	bucket, err := bandwidthlimiter.NewBucket(ResponseBandwidthBurstSize, ResponseBandwidthBurstSize, refillNumber, refillInterval)
	if err != nil {
		cancel() // free resources
		return nil, nil, err
	}

	bandwidthLimitingWriter, err := bandwidthlimiter.NewBandwidthLimitingWriter(cancellableWriter, bucket)
	if err != nil {
		cancel() // free resources
		return nil, nil, err
	}

	return bandwidthLimitingWriter, cancel, nil
}

func NewFlushingWriter(w io.Writer) (*FlushingWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		errorMsg := "expected http.ResponseWriter to be an http.Flusher"
		log.Error(errorMsg)
		return nil, errors.New(errorMsg)
	}
	return &FlushingWriter{
		w:       w,
		flusher: flusher,
	}, nil
}

type FlushingWriter struct {
	w       io.Writer
	flusher http.Flusher
}

func (w *FlushingWriter) Write(p []byte) (n int, err error) {
	n, err = w.w.Write(p)
	w.flusher.Flush()
	return
}

func NewCancellableWriter(w io.Writer) (*CancellableWriter, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	return &CancellableWriter{w: w, ctx: ctx}, cancel
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
