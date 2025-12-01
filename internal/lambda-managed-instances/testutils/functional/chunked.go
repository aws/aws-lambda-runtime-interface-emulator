// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package functional

import (
	"bytes"
	"io"
	"time"
)

type ChunkedReader struct {
	buffers    []*bytes.Buffer
	currentIdx int
	delay      time.Duration
}

func NewChunkedReader(chunks []string, delay time.Duration) *ChunkedReader {
	buffers := make([]*bytes.Buffer, len(chunks))
	for i, chunk := range chunks {
		buffers[i] = bytes.NewBuffer([]byte(chunk))
	}

	return &ChunkedReader{
		buffers:    buffers,
		currentIdx: 0,
		delay:      delay,
	}
}

func (r *ChunkedReader) Read(p []byte) (n int, err error) {
	if r.currentIdx >= len(r.buffers) {
		return 0, io.EOF
	}

	n, err = r.buffers[r.currentIdx].Read(p)

	if err == io.EOF && r.currentIdx < len(r.buffers)-1 {
		r.currentIdx++
		time.Sleep(r.delay)
		return r.Read(p)
	}

	return n, err
}
