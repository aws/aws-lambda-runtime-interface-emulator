// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package directinvoke

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.amzn.com/lambda/interop"
)

func NewResponseWriterWithoutFlushMethod() *ResponseWriterWithoutFlushMethod {
	return &ResponseWriterWithoutFlushMethod{}
}

type ResponseWriterWithoutFlushMethod struct{}

func (*ResponseWriterWithoutFlushMethod) Header() http.Header             { return http.Header{} }
func (*ResponseWriterWithoutFlushMethod) Write([]byte) (n int, err error) { return }
func (*ResponseWriterWithoutFlushMethod) WriteHeader(_ int)               {}

func NewSimpleResponseWriter() *SimpleResponseWriter {
	return &SimpleResponseWriter{
		buffer:   bytes.NewBuffer(nil),
		trailers: make(http.Header),
	}
}

type SimpleResponseWriter struct {
	buffer   *bytes.Buffer
	trailers http.Header
}

func (w *SimpleResponseWriter) Header() http.Header               { return w.trailers }
func (w *SimpleResponseWriter) Write(p []byte) (n int, err error) { return w.buffer.Write(p) }
func (*SimpleResponseWriter) WriteHeader(_ int)                   {}
func (*SimpleResponseWriter) Flush()                              {}

func NewInterruptableResponseWriter(interruptAfter int) (*InterruptableResponseWriter, chan struct{}) {
	interruptedTestWriterChan := make(chan struct{})
	return &InterruptableResponseWriter{
		buffer:                    bytes.NewBuffer(nil),
		trailers:                  make(http.Header),
		interruptAfter:            interruptAfter,
		interruptedTestWriterChan: interruptedTestWriterChan,
	}, interruptedTestWriterChan
}

type InterruptableResponseWriter struct {
	buffer                    *bytes.Buffer
	trailers                  http.Header
	interruptAfter            int // expect Writer to be interrupted after 'interruptAfter' number of writes
	interruptedTestWriterChan chan struct{}
}

func (w *InterruptableResponseWriter) Header() http.Header { return w.trailers }
func (w *InterruptableResponseWriter) Write(p []byte) (n int, err error) {
	if w.interruptAfter >= 1 {
		w.interruptAfter--
	} else if w.interruptAfter == 0 {
		w.interruptedTestWriterChan <- struct{}{} // ready to be interrupted
		<-w.interruptedTestWriterChan             // wait until interrupted
	}
	n, err = w.buffer.Write(p)
	return
}
func (*InterruptableResponseWriter) WriteHeader(_ int) {}
func (*InterruptableResponseWriter) Flush()            {}

// This is a simple reader implementing io.Reader interface. It's based on strings.Reader, but it doesn't have extra
// methods that allow faster copying such as .WriteTo() method.
func NewReader(s string) *Reader { return &Reader{s, 0, -1} }

type Reader struct {
	s        string
	i        int64 // current reading index
	prevRune int   // index of previous rune; or < 0
}

func (r *Reader) Read(b []byte) (n int, err error) {
	if r.i >= int64(len(r.s)) {
		return 0, io.EOF
	}
	r.prevRune = -1
	n = copy(b, r.s[r.i:])
	r.i += int64(n)
	return
}

func TestSendDirectInvokeWithIncompatibleResponseWriter(t *testing.T) {
	MaxDirectResponseSize = -1
	err := SendDirectInvokeResponse(nil, nil, nil, NewResponseWriterWithoutFlushMethod(), nil, nil, nil, false)
	require.Error(t, err)
	require.Equal(t, "ErrInternalPlatformError", err.Error())
}

func TestAsyncPayloadCopySuccess(t *testing.T) {
	payloadString := strings.Repeat("a", 10*1024*1024)
	writer := NewSimpleResponseWriter()

	expectedPayloadString := payloadString

	copyDone, _, err := asyncPayloadCopy(writer, NewReader(payloadString))
	require.Nil(t, err)

	<-copyDone
	require.Equal(t, expectedPayloadString, writer.buffer.String())
}

// We use an interruptable response writer which informs on a channel that it's ready to be interrupted after
// 'interruptAfter' number of writes, then it waits for interruption completion to resume the current write operation.
// For this test, after initiating copying, we wait for one chunk of 32 KiB to be copied. Then, we use cancel() to
// interrupt copying. At this point, only ongoing .Write() operations can be performed. We inform the writer about
// interruption completion, and the writer resumes the current .Write() operation, which gives us another 32 KiB chunk
// that is copied. After that, copying returns, and we receive a signal on <-copyDone channel.
func TestAsyncPayloadCopySuccessAfterCancel(t *testing.T) {
	payloadString := strings.Repeat("a", 10*1024*1024) // 10 MiB
	writer, interruptedTestWriterChan := NewInterruptableResponseWriter(1)

	expectedPayloadString := strings.Repeat("a", 64*1024) // 64 KiB (2 chunks)

	copyDone, cancel, err := asyncPayloadCopy(writer, NewReader(payloadString))
	require.Nil(t, err)

	<-interruptedTestWriterChan             // wait for writing 'interruptAfter' number of chunks
	cancel()                                // interrupt copying
	interruptedTestWriterChan <- struct{}{} // inform test writer about interruption

	<-copyDone
	require.Equal(t, expectedPayloadString, writer.buffer.String())
}

func TestAsyncPayloadCopyWithIncompatibleResponseWriter(t *testing.T) {
	copyDone, cancel, err := asyncPayloadCopy(&ResponseWriterWithoutFlushMethod{}, nil)
	require.Nil(t, copyDone)
	require.Nil(t, cancel)
	require.Error(t, err)
	require.Equal(t, "ErrInternalPlatformError", err.Error())
}

func TestSendStreamingInvokeResponseSuccess(t *testing.T) {
	payloadString := strings.Repeat("a", 128*1024) // 128 KiB
	payload := NewReader(payloadString)
	trailers := http.Header{}
	writer := NewSimpleResponseWriter()
	interruptedResponseChan := make(chan *interop.Reset)
	sendResponseChan := make(chan *interop.InvokeResponseMetrics)
	testFinished := make(chan struct{})

	expectedPayloadString := payloadString

	go func() {
		err := sendStreamingInvokeResponse(payload, trailers, writer, interruptedResponseChan, sendResponseChan, nil, false)
		require.Nil(t, err)
		testFinished <- struct{}{}
	}()

	<-sendResponseChan
	require.Equal(t, expectedPayloadString, writer.buffer.String())
	require.Equal(t, "", writer.Header().Get("Lambda-Runtime-Function-Error-Type"))
	require.Equal(t, "", writer.Header().Get("Lambda-Runtime-Function-Error-Body"))
	require.Equal(t, "Complete", writer.Header().Get("End-Of-Response"))
	<-testFinished
}

func TestSendPayloadLimitedResponseWithinThresholdWithStreamingFunction(t *testing.T) {
	payloadSize := 1
	payloadString := strings.Repeat("a", payloadSize)
	payload := NewReader(payloadString)
	trailers := http.Header{}
	writer := NewSimpleResponseWriter()
	writer.Header().Set("Lambda-Runtime-Function-Response-Mode", "streaming")
	sendResponseChan := make(chan *interop.InvokeResponseMetrics)
	testFinished := make(chan struct{})

	MaxDirectResponseSize = int64(payloadSize + 1)

	go func() {
		err := sendPayloadLimitedResponse(payload, trailers, writer, sendResponseChan, true)
		require.Nil(t, err)
		testFinished <- struct{}{}
	}()

	metrics := <-sendResponseChan
	require.Equal(t, interop.FunctionResponseModeBuffered, metrics.FunctionResponseMode)
	require.Equal(t, len(payloadString), len(writer.buffer.String()))
	require.Equal(t, "", writer.Header().Get("Lambda-Runtime-Function-Error-Type"))
	require.Equal(t, "", writer.Header().Get("Lambda-Runtime-Function-Error-Body"))
	require.Equal(t, "Complete", writer.Header().Get("End-Of-Response"))
	<-testFinished

	// Reset to its default value, just in case other tests use them
	MaxDirectResponseSize = interop.MaxPayloadSize
}

func TestSendPayloadLimitedResponseAboveThresholdWithStreamingFunction(t *testing.T) {
	payloadSize := 2
	payloadString := strings.Repeat("a", payloadSize)
	payload := NewReader(payloadString)
	trailers := http.Header{}
	writer := NewSimpleResponseWriter()
	writer.Header().Set("Lambda-Runtime-Function-Response-Mode", "streaming")
	sendResponseChan := make(chan *interop.InvokeResponseMetrics)
	testFinished := make(chan struct{})
	MaxDirectResponseSize = int64(payloadSize - 1)
	expectedError := &interop.ErrorResponseTooLargeDI{
		ErrorResponseTooLarge: interop.ErrorResponseTooLarge{
			MaxResponseSize: int(MaxDirectResponseSize),
			ResponseSize:    payloadSize,
		},
	}

	go func() {
		err := sendPayloadLimitedResponse(payload, trailers, writer, sendResponseChan, true)
		require.Equal(t, expectedError, err)
		testFinished <- struct{}{}
	}()

	metrics := <-sendResponseChan
	require.Equal(t, interop.FunctionResponseModeBuffered, metrics.FunctionResponseMode)
	require.Equal(t, len(payloadString), len(writer.buffer.String()))
	require.Equal(t, "", writer.Header().Get("Lambda-Runtime-Function-Error-Type"))
	require.Equal(t, "", writer.Header().Get("Lambda-Runtime-Function-Error-Body"))
	require.Equal(t, "Oversized", writer.Header().Get("End-Of-Response"))
	<-testFinished

	// Reset to its default value, just in case other tests use them
	MaxDirectResponseSize = interop.MaxPayloadSize
}

func TestSendStreamingInvokeResponseSuccessWithTrailers(t *testing.T) {
	payloadString := strings.Repeat("a", 128*1024) // 128 KiB
	payload := NewReader(payloadString)
	trailers := http.Header{
		"Lambda-Runtime-Function-Error-Type": []string{"ErrorType"},
		"Lambda-Runtime-Function-Error-Body": []string{"ErrorBody"},
	}
	writer := NewSimpleResponseWriter()
	interruptedResponseChan := make(chan *interop.Reset)
	sendResponseChan := make(chan *interop.InvokeResponseMetrics)
	testFinished := make(chan struct{})

	expectedPayloadString := payloadString

	go func() {
		err := sendStreamingInvokeResponse(payload, trailers, writer, interruptedResponseChan, sendResponseChan, nil, false)
		require.Nil(t, err)
		testFinished <- struct{}{}
	}()

	<-sendResponseChan
	require.Equal(t, expectedPayloadString, writer.buffer.String())
	require.Equal(t, "ErrorType", writer.Header().Get("Lambda-Runtime-Function-Error-Type"))
	require.Equal(t, "ErrorBody", writer.Header().Get("Lambda-Runtime-Function-Error-Body"))
	require.Equal(t, "Complete", writer.Header().Get("End-Of-Response"))
	<-testFinished
}

func TestSendStreamingInvokeResponseReset(t *testing.T) { // Reset initiated after writing two chunks of 32 KiB
	payloadString := strings.Repeat("a", 128*1024) // 128 KiB
	payload := NewReader(payloadString)
	trailers := http.Header{}
	writer, interruptedTestWriterChan := NewInterruptableResponseWriter(1)
	interruptedResponseChan := make(chan *interop.Reset)
	sendResponseChan := make(chan *interop.InvokeResponseMetrics)
	testFinished := make(chan struct{})

	expectedPayloadString := strings.Repeat("a", 64*1024) // 64 KiB

	go func() {
		err := sendStreamingInvokeResponse(payload, trailers, writer, interruptedResponseChan, sendResponseChan, nil, true)
		require.Error(t, err)
		require.Equal(t, "ErrTruncatedResponse", err.Error())
		testFinished <- struct{}{}
	}()

	reset := &interop.Reset{Reason: "timeout"}
	require.Nil(t, reset.InvokeResponseMetrics)

	<-interruptedTestWriterChan             // wait for writing 'interruptAfter' number of chunks
	interruptedResponseChan <- reset        // send reset
	time.Sleep(10 * time.Millisecond)       // wait for cancel() being called (first instruction after getting reset)
	interruptedTestWriterChan <- struct{}{} // inform test writer about interruption
	<-interruptedResponseChan               // wait for copy done after interruption
	require.NotNil(t, reset.InvokeResponseMetrics)

	<-sendResponseChan
	require.Equal(t, expectedPayloadString, writer.buffer.String())
	require.Equal(t, "Sandbox.Timeout", writer.Header().Get("Lambda-Runtime-Function-Error-Type"))
	require.Equal(t, "", writer.Header().Get("Lambda-Runtime-Function-Error-Body"))
	require.Equal(t, "Truncated", writer.Header().Get("End-Of-Response"))
	<-testFinished
}

func TestSendStreamingInvokeErrorResponseSuccess(t *testing.T) {
	payloadString := strings.Repeat("a", 128*1024) // 128 KiB
	payload := NewReader(payloadString)
	writer := NewSimpleResponseWriter()
	interruptedResponseChan := make(chan *interop.Reset)
	sendResponseChan := make(chan *interop.InvokeResponseMetrics)
	testFinished := make(chan struct{})

	expectedPayloadString := payloadString

	go func() {
		err := sendStreamingInvokeErrorResponse(payload, writer, interruptedResponseChan, sendResponseChan, false)
		require.Nil(t, err)
		testFinished <- struct{}{}
	}()

	<-sendResponseChan
	require.Equal(t, expectedPayloadString, writer.buffer.String())
	require.Equal(t, "", writer.Header().Get("Lambda-Runtime-Function-Error-Type"))
	require.Equal(t, "", writer.Header().Get("Lambda-Runtime-Function-Error-Body"))
	require.Equal(t, "Complete", writer.Header().Get("End-Of-Response"))
	<-testFinished
}

func TestSendStreamingInvokeErrorResponseReset(t *testing.T) { // Reset initiated after writing two chunks of 32 KiB
	payloadString := strings.Repeat("a", 128*1024) // 128 KiB
	payload := NewReader(payloadString)
	writer, interruptedTestWriterChan := NewInterruptableResponseWriter(1)
	interruptedResponseChan := make(chan *interop.Reset)
	sendResponseChan := make(chan *interop.InvokeResponseMetrics)
	testFinished := make(chan struct{})

	expectedPayloadString := strings.Repeat("a", 64*1024) // 64 KiB

	go func() {
		err := sendStreamingInvokeErrorResponse(payload, writer, interruptedResponseChan, sendResponseChan, true)
		require.Error(t, err)
		require.Equal(t, "ErrTruncatedResponse", err.Error())
		testFinished <- struct{}{}
	}()

	reset := &interop.Reset{Reason: "timeout"}
	require.Nil(t, reset.InvokeResponseMetrics)

	<-interruptedTestWriterChan             // wait for writing 'interruptAfter' number of chunks
	interruptedResponseChan <- reset        // send reset
	time.Sleep(10 * time.Millisecond)       // wait for cancel() being called (first instruction after getting reset)
	interruptedTestWriterChan <- struct{}{} // inform test writer about interruption
	<-interruptedResponseChan               // wait for copy done after interruption
	require.NotNil(t, reset.InvokeResponseMetrics)

	<-sendResponseChan
	require.Equal(t, expectedPayloadString, writer.buffer.String())
	require.Equal(t, "", writer.Header().Get("Lambda-Runtime-Function-Error-Type"))
	require.Equal(t, "", writer.Header().Get("Lambda-Runtime-Function-Error-Body"))
	require.Equal(t, "Truncated", writer.Header().Get("End-Of-Response"))
	<-testFinished
}
