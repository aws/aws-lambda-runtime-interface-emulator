// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rie

import (
	"fmt"
	"net/http"
)

type ErrorType int

const (
	ClientInvalidRequest ErrorType = iota
)

func (t ErrorType) String() string {
	switch t {
	case ClientInvalidRequest:
		return "Client.InvalidRequest"
	}
	return fmt.Sprintf("Cannot stringify standalone.ErrorType.%d", int(t))
}

// ResponseWriterProxy is the http.ResponseWriter wrapper used between the
// rapidcore Server and the real caller-facing connection.
//
// Historically it acted as a tiny buffer: it captured a single status code and
// a single body slab so that, after the invoke returned, the rie.InvokeHandler
// could decide whether to forward the captured response or to overwrite it
// with an error of its own. That design forced the runtime's /response body to
// be fully read before any byte was written to the caller, which made
// Lambda response streaming (e.g. Server-Sent Events) impossible.
//
// The proxy can now optionally hold an Underlying http.ResponseWriter. When
// set, writes are forwarded straight to the underlying writer and immediately
// flushed if the writer implements http.Flusher, while still keeping a copy of
// the headers/status that we accumulate before the first write so we can
// commit them on demand. Errors paths that fire BEFORE any data has been
// streamed (Started == false) keep their existing behavior of being able to
// write a synthetic response. Once Started is true, the rie.InvokeHandler
// must not attempt to set headers or status again on the underlying writer.
type ResponseWriterProxy struct {
	// Body keeps a copy of any bytes written while we have not yet committed
	// to streaming (Underlying == nil or Started == false). It is preserved
	// for backward compatibility with error code paths in rie.InvokeHandler
	// that re-emit invokeResp.Body on failure. Once we start streaming we
	// stop accumulating to avoid unbounded memory growth.
	Body       []byte
	StatusCode int

	// Underlying, when non-nil, is the real http.ResponseWriter we forward
	// writes to. Setting this enables true response streaming: every Write
	// passes straight through and triggers a Flush.
	Underlying http.ResponseWriter

	// headers accumulates Header().Add(...) calls until the first Write.
	// On the first Write they are copied onto Underlying.Header().
	headers http.Header

	// Started is set to true after the first Write that has been committed
	// to Underlying. Once true, headers and status are locked.
	Started bool
}

// Header returns the staged headers map. When streaming has not yet started
// these are local to the proxy; once Started, they are merged onto the real
// writer's header map.
func (w *ResponseWriterProxy) Header() http.Header {
	if w.Started && w.Underlying != nil {
		return w.Underlying.Header()
	}
	if w.headers == nil {
		w.headers = make(http.Header)
	}
	return w.headers
}

// Write forwards bytes to the underlying writer when available, flushing
// after each chunk so that callers (browsers consuming SSE) see chunks as
// soon as the runtime emits them.
func (w *ResponseWriterProxy) Write(b []byte) (int, error) {
	if w.Underlying != nil {
		if !w.Started {
			// Promote staged headers + status onto the underlying writer.
			if w.headers != nil {
				underlyingHeaders := w.Underlying.Header()
				for k, vs := range w.headers {
					for _, v := range vs {
						underlyingHeaders.Add(k, v)
					}
				}
			}
			if w.StatusCode != 0 {
				w.Underlying.WriteHeader(w.StatusCode)
			}
			w.Started = true
		}
		n, err := w.Underlying.Write(b)
		if f, ok := w.Underlying.(http.Flusher); ok {
			f.Flush()
		}
		return n, err
	}

	// No streaming target: behave like the original buffer-everything proxy
	// (note: the original returned (0, nil) which is technically a violation
	// of io.Writer; we return len(b) to be correct, since callers like
	// io.Copy interpret a short write as an error).
	w.Body = append(w.Body, b...)
	return len(b), nil
}

func (w *ResponseWriterProxy) WriteHeader(statusCode int) {
	w.StatusCode = statusCode
}

func (w *ResponseWriterProxy) IsError() bool {
	return w.StatusCode != 0 && w.StatusCode/100 != 2
}

// Flush implements http.Flusher so that callers performing io.Copy on us
// (or wrapping us) can drive intermediate flushes too.
func (w *ResponseWriterProxy) Flush() {
	if w.Underlying == nil {
		return
	}
	if f, ok := w.Underlying.(http.Flusher); ok {
		f.Flush()
	}
}
