// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"io"
	"sync"
)

// TailLogWriter writes tail/debug log to provided io.Writer
type TailLogWriter struct {
	out     io.Writer
	enabled bool
	mutex   sync.Mutex
}

// Enable enables log writer.
func (lw *TailLogWriter) Enable() {
	lw.mutex.Lock()
	defer lw.mutex.Unlock()

	lw.enabled = true
}

// Disable disables log writer.
func (lw *TailLogWriter) Disable() {
	lw.mutex.Lock()
	defer lw.mutex.Unlock()

	lw.enabled = false
}

// Writer wraps the basic io.Write method
func (lw *TailLogWriter) Write(p []byte) (n int, err error) {
	lw.mutex.Lock()
	defer lw.mutex.Unlock()

	if lw.enabled {
		return lw.out.Write(p)
	}
	// Else returns a successful write so that MultiWriter won't stop
	return len(p), nil
}

// NewTailLogWriter returns a new invoke tail log writer, default output is discarded until output is configured.
func NewTailLogWriter(w io.Writer) *TailLogWriter {
	return &TailLogWriter{
		out:     w,
		enabled: false,
	}
}
