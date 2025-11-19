// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package bandwidthlimiter

import (
	"io"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/interop"
)

func BandwidthLimitingCopy(dst *BandwidthLimitingWriter, src io.Reader) (written int64, err error) {
	written, err = io.Copy(dst, src)
	_ = dst.Close()
	return
}

func NewBandwidthLimitingWriter(w io.Writer, bucket *Bucket) (*BandwidthLimitingWriter, error) {
	throttler, err := NewThrottler(bucket)
	if err != nil {
		return nil, err
	}
	return &BandwidthLimitingWriter{w: w, th: throttler}, nil
}

type BandwidthLimitingWriter struct {
	w  io.Writer
	th *Throttler
}

func (w *BandwidthLimitingWriter) ChunkedWrite(p []byte) (n int, err error) {
	i := NewChunkIterator(p, int(w.th.b.capacity))
	for {
		buf := i.Next()
		if buf == nil {
			return
		}
		written, writeErr := w.th.bandwidthLimitingWrite(w.w, buf)
		n += written
		if writeErr != nil {
			return n, writeErr
		}
	}
}

func (w *BandwidthLimitingWriter) Write(p []byte) (n int, err error) {
	w.th.start()
	if int64(len(p)) > w.th.b.capacity {
		return w.ChunkedWrite(p)
	}
	return w.th.bandwidthLimitingWrite(w.w, p)
}

func (w *BandwidthLimitingWriter) Close() (err error) {
	w.th.stop()
	return
}

func (w *BandwidthLimitingWriter) GetMetrics() (metrics *interop.InvokeResponseMetrics) {
	return w.th.metrics
}
