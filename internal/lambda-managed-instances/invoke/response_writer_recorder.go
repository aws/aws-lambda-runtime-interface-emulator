// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"log/slog"
	"net/http"
	"sync/atomic"
)

type ResponseWriterRecorder struct {
	statusCode  int
	header      http.Header
	trailer     http.Header
	body        []byte
	headersSent atomic.Bool
}

func (r *ResponseWriterRecorder) Header() http.Header {
	if r.headersSent.Load() {
		if r.trailer == nil {
			r.trailer = make(http.Header)
		}
		return r.trailer
	}
	if r.header == nil {
		r.header = make(http.Header)
	}
	return r.header
}

func (r *ResponseWriterRecorder) Write(bytes []byte) (int, error) {
	r.headersSent.Store(true)
	r.body = append(r.body, bytes...)
	return len(bytes), nil
}

func (r *ResponseWriterRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.headersSent.Store(true)
}

func (r *ResponseWriterRecorder) Flush() {}

func (r *ResponseWriterRecorder) BodySize() int { return len(r.body) }

func (r *ResponseWriterRecorder) WriteTo(w http.ResponseWriter) error {
	for k, vs := range r.header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	if r.statusCode != 0 {
		w.WriteHeader(r.statusCode)
	}

	if _, err := w.Write(r.body); err != nil {
		slog.Error("ResponseWriterRecorder: failed to write body", "err", err)
		return err
	}
	for k, vs := range r.trailer {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	return nil
}
