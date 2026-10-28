// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseWriterRecorder_HeadersBeforeWrite(t *testing.T) {
	t.Parallel()
	rec := &ResponseWriterRecorder{}

	rec.Header().Set("Content-Type", "text/plain")
	rec.WriteHeader(http.StatusOK)
	_, err := rec.Write([]byte("body"))
	require.NoError(t, err)

	assert.Equal(t, "text/plain", rec.header.Get("Content-Type"))
	assert.Equal(t, http.StatusOK, rec.statusCode)
	assert.Equal(t, []byte("body"), rec.body)
}

func TestResponseWriterRecorder_TrailersAfterWrite(t *testing.T) {
	t.Parallel()
	rec := &ResponseWriterRecorder{}

	rec.Header().Set("Content-Type", "text/plain")
	rec.WriteHeader(http.StatusOK)
	_, err := rec.Write([]byte("body"))
	require.NoError(t, err)

	rec.Header().Set("End-Of-Response", "Complete")

	assert.Equal(t, "text/plain", rec.header.Get("Content-Type"))
	assert.Equal(t, "", rec.header.Get("End-Of-Response"))
	assert.Equal(t, "Complete", rec.trailer.Get("End-Of-Response"))
}

func TestResponseWriterRecorder_WriteTo_ReplaysAll(t *testing.T) {
	t.Parallel()
	rec := &ResponseWriterRecorder{}

	rec.Header().Set("X-Header", "val")
	rec.WriteHeader(http.StatusOK)
	_, err := rec.Write([]byte("body"))
	require.NoError(t, err)
	rec.Header().Set("X-Trailer", "tval")

	w := httptest.NewRecorder()
	_ = rec.WriteTo(w)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "val", w.Header().Get("X-Header"))
	assert.Equal(t, "body", w.Body.String())
	assert.Equal(t, "tval", w.Header().Get("X-Trailer"))
}

func TestResponseWriterRecorder_WriteTo_EmptyRecorder(t *testing.T) {
	t.Parallel()
	rec := &ResponseWriterRecorder{}
	w := httptest.NewRecorder()

	_ = rec.WriteTo(w)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestResponseWriterRecorder_WriteTo_WriteError(t *testing.T) {
	t.Parallel()
	rec := &ResponseWriterRecorder{}
	rec.WriteHeader(http.StatusOK)
	_, _ = rec.Write([]byte("payload"))

	w := &failingWriter{header: make(http.Header)}
	err := rec.WriteTo(w)
	assert.Error(t, err)
}

func TestResponseWriterRecorder_Flush_NoOp(t *testing.T) {
	t.Parallel()
	rec := &ResponseWriterRecorder{}
	rec.Flush()
}

func TestResponseWriterRecorder_WriteHeaderSwitchesToTrailerMap(t *testing.T) {
	t.Parallel()
	rec := &ResponseWriterRecorder{}

	rec.Header().Set("Invoke-Id", "abc-123")
	rec.WriteHeader(http.StatusOK)
	rec.Header().Set("End-Of-Response", "Complete")
	rec.Header().Set("Error-Category", "RuntimeError")

	assert.Equal(t, "abc-123", rec.header.Get("Invoke-Id"))

	assert.Equal(t, "", rec.header.Get("End-Of-Response"))
	assert.Equal(t, "", rec.header.Get("Error-Category"))
	assert.Equal(t, "Complete", rec.trailer.Get("End-Of-Response"))
	assert.Equal(t, "RuntimeError", rec.trailer.Get("Error-Category"))
}

func TestResponseWriterRecorder_WriteTo_ErrorWithoutBody(t *testing.T) {
	t.Parallel()
	rec := &ResponseWriterRecorder{}

	rec.Header().Set("Content-Type", "text/plain")
	rec.Header().Set("Invoke-Id", "abc-123")
	rec.WriteHeader(http.StatusOK)

	rec.Header().Set("End-Of-Response", "Complete")
	rec.Header().Set("Error-Category", "RuntimeError")

	w := httptest.NewRecorder()
	_ = rec.WriteTo(w)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
	assert.Equal(t, "abc-123", w.Header().Get("Invoke-Id"))

	assert.Equal(t, "Complete", w.Header().Get("End-Of-Response"))
	assert.Equal(t, "RuntimeError", w.Header().Get("Error-Category"))
}

func TestResponseWriterRecorder_WriteTo_EmptyBodyTriggersWrite(t *testing.T) {
	t.Parallel()
	rec := &ResponseWriterRecorder{}

	rec.Header().Set("Invoke-Id", "abc-123")
	rec.WriteHeader(http.StatusOK)

	spy := &spyWriter{ResponseRecorder: httptest.NewRecorder()}
	_ = rec.WriteTo(spy)

	assert.True(t, spy.writeCalled, "Write should have been called even with empty body")
	assert.Equal(t, http.StatusOK, spy.Code)
}

type spyWriter struct {
	*httptest.ResponseRecorder
	writeCalled bool
}

func (s *spyWriter) Write(b []byte) (int, error) {
	s.writeCalled = true
	return s.ResponseRecorder.Write(b)
}

func TestResponseWriterRecorder_WriteTo_SuccessWithBody(t *testing.T) {
	t.Parallel()
	rec := &ResponseWriterRecorder{}

	rec.Header().Set("Content-Type", "application/json")
	rec.Header().Set("Invoke-Id", "abc-123")
	rec.WriteHeader(http.StatusOK)
	_, err := rec.Write([]byte(`{"result":"ok"}`))
	require.NoError(t, err)
	rec.Header().Set("End-Of-Response", "Complete")

	w := httptest.NewRecorder()
	_ = rec.WriteTo(w)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, `{"result":"ok"}`, w.Body.String())
	assert.Equal(t, "Complete", w.Header().Get("End-Of-Response"))
}

type failingWriter struct {
	header http.Header
}

func (f *failingWriter) Header() http.Header         { return f.header }
func (f *failingWriter) WriteHeader(_ int)           {}
func (f *failingWriter) Write(_ []byte) (int, error) { return 0, errors.New("connection reset") }
