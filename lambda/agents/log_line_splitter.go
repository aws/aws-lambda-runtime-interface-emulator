// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"bytes"
	"io"
)

// NewlineSplitWriter wraps an io.Writer and calls the underlying writer for each newline separated line
type NewlineSplitWriter struct {
	writer io.Writer
}

// NewNewlineSplitWriter returns an instance of NewlineSplitWriter
func NewNewlineSplitWriter(w io.Writer) *NewlineSplitWriter {
	return &NewlineSplitWriter{
		writer: w,
	}
}

// Write splits the byte buffer by newline and calls the underlying writer for each line
func (nsw *NewlineSplitWriter) Write(buf []byte) (int, error) {
	newBuf := make([]byte, len(buf))
	copy(newBuf, buf)
	lines := bytes.SplitAfter(newBuf, []byte("\n"))
	var bytesWritten int
	for _, line := range lines {
		if len(line) > 0 {
			n, err := nsw.writer.Write(line)
			bytesWritten += n
			if err != nil {
				return bytesWritten, err
			}
		}
	}

	return bytesWritten, nil
}
