// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package testutils

import "io"

type BlockingReadCloser struct {
	done chan struct{}
}

func NewBlockingReadCloser() *BlockingReadCloser {
	return &BlockingReadCloser{done: make(chan struct{})}
}

func (r *BlockingReadCloser) Read(p []byte) (int, error) {
	<-r.done
	return 0, io.EOF
}

func (r *BlockingReadCloser) Close() error {
	select {
	case <-r.done:
	default:
		close(r.done)
	}
	return nil
}
