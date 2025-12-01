// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package mocks

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

type ReaderMock struct {
	PayloadSize    int
	ReadTotal      int
	WaitBeforeRead time.Duration
}

func (r *ReaderMock) Read(p []byte) (int, error) {
	if r.WaitBeforeRead != 0 {
		time.Sleep(r.WaitBeforeRead)
	}

	if r.ReadTotal >= r.PayloadSize {
		return 0, io.EOF
	}

	haveRead := min(len(p), r.PayloadSize-r.ReadTotal)
	r.ReadTotal += haveRead

	return haveRead, nil
}

type ReaderFailureMock struct{}

func (r *ReaderFailureMock) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("can't read")
}

type WriterFailureMock struct{}

func (w *WriterFailureMock) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("can't write")
}

type ResponseWriterMock struct {
	io.Writer
}

func (w *ResponseWriterMock) Header() http.Header {
	return map[string][]string{}
}

func (w *ResponseWriterMock) WriteHeader(int) {}

func (w *ResponseWriterMock) Flush() {}
