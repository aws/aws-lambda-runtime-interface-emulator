// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package directinvoke

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.amzn.com/lambda/fatalerror"
	"go.amzn.com/lambda/interop"
	"go.amzn.com/lambda/metering"
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

func TestAsyncPayloadCopyWhenPayloadSizeBelowMaxAllowed(t *testing.T) {
	MaxDirectResponseSize = 2
	payloadSize := int(MaxDirectResponseSize - 1)
	payloadString := strings.Repeat("a", payloadSize)
	writer := NewSimpleResponseWriter()

	copyDone, _, err := asyncPayloadCopy(writer, NewReader(payloadString))
	require.Nil(t, err)

	copyDoneResult := <-copyDone
	require.Nil(t, copyDoneResult.Error)

	require.Equal(t, payloadString, writer.buffer.String())
	require.Equal(t, EndOfResponseComplete, writer.Header().Get(EndOfResponseTrailer))

	// reset it to its original value
	MaxDirectResponseSize = interop.MaxPayloadSize
}

func TestAsyncPayloadCopyWhenPayloadSizeEqualMaxAllowed(t *testing.T) {
	MaxDirectResponseSize = 2
	payloadSize := int(MaxDirectResponseSize)
	payloadString := strings.Repeat("a", payloadSize)
	writer := NewSimpleResponseWriter()

	copyDone, _, err := asyncPayloadCopy(writer, NewReader(payloadString))
	require.Nil(t, err)

	copyDoneResult := <-copyDone
	require.Nil(t, copyDoneResult.Error)

	require.Equal(t, payloadString, writer.buffer.String())
	require.Equal(t, EndOfResponseComplete, writer.Header().Get(EndOfResponseTrailer))

	// reset it to its original value
	MaxDirectResponseSize = interop.MaxPayloadSize
}

func TestAsyncPayloadCopyWhenPayloadSizeAboveMaxAllowed(t *testing.T) {
	MaxDirectResponseSize = 2
	payloadSize := int(MaxDirectResponseSize) + 1
	payloadString := strings.Repeat("a", payloadSize)
	writer := NewSimpleResponseWriter()
	expectedCopyDoneResultError := &interop.ErrorResponseTooLargeDI{
		ErrorResponseTooLarge: interop.ErrorResponseTooLarge{
			ResponseSize:    payloadSize,
			MaxResponseSize: int(MaxDirectResponseSize),
		},
	}

	copyDone, _, err := asyncPayloadCopy(writer, NewReader(payloadString))
	require.Nil(t, err)

	copyDoneResult := <-copyDone
	require.Equal(t, expectedCopyDoneResultError, copyDoneResult.Error)

	require.Equal(t, payloadString, writer.buffer.String())
	require.Equal(t, EndOfResponseOversized, writer.Header().Get(EndOfResponseTrailer))

	// reset it to its original value
	MaxDirectResponseSize = interop.MaxPayloadSize
}

// This is only allowed in streaming mode, currently.
func TestAsyncPayloadCopyWhenUnlimitedPayloadSizeAllowed(t *testing.T) {
	MaxDirectResponseSize = -1
	payloadSize := int(interop.MaxPayloadSize + 1)
	payloadString := strings.Repeat("a", payloadSize)
	writer := NewSimpleResponseWriter()

	copyDone, _, err := asyncPayloadCopy(writer, NewReader(payloadString))
	require.Nil(t, err)

	copyDoneResult := <-copyDone
	require.Nil(t, copyDoneResult.Error)

	require.Equal(t, payloadString, writer.buffer.String())
	require.Equal(t, EndOfResponseComplete, writer.Header().Get(EndOfResponseTrailer))

	// reset it to its original value
	MaxDirectResponseSize = interop.MaxPayloadSize
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

// TODO: in order to implement this test we need bandwidthlimiter to be received by asyncPayloadCopy
// as an argument. Otherwise, this test will need to know how to force bandwidthlimiter to fail,
// which isn't a good practice.
func TestAsyncPayloadCopyWhenResponseIsTruncated(t *testing.T) {
	t.Skip("Pending injection of bandwidthlimiter as a dependency of asyncPayloadCopy.")
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
	require.Equal(t, interop.InvokeResponseMode("Buffered"), reset.InvokeResponseMode)

	<-sendResponseChan
	require.Equal(t, expectedPayloadString, writer.buffer.String())
	require.Equal(t, "Sandbox.Timeout", writer.Header().Get("Lambda-Runtime-Function-Error-Type"))
	require.Equal(t, "", writer.Header().Get("Lambda-Runtime-Function-Error-Body"))
	require.Equal(t, "Truncated", writer.Header().Get("End-Of-Response"))
	<-testFinished
}

// TODO: mock asyncPayloadCopy and force it to return Oversized in copyDone
func TestSendStreamingInvokeResponseOversizedRuntimesWithTrailers(t *testing.T) {
	oversizedPayloadString := strings.Repeat("a", int(MaxDirectResponseSize)+1)
	payload := NewReader(oversizedPayloadString)
	trailers := http.Header{
		FunctionErrorTypeTrailer: []string{"RuntimesErrorType"},
		FunctionErrorBodyTrailer: []string{"RuntimesBody"},
	}
	writer := NewSimpleResponseWriter()
	interruptedResponseChan := make(chan *interop.Reset)
	sendResponseChan := make(chan *interop.InvokeResponseMetrics)
	testFinished := make(chan struct{})

	go func() {
		err := sendStreamingInvokeResponse(payload, trailers, writer, interruptedResponseChan, sendResponseChan, nil, false)
		require.Error(t, err)
		require.IsType(t, &interop.ErrorResponseTooLargeDI{}, err)
		testFinished <- struct{}{}
	}()

	<-sendResponseChan
	require.Equal(t, trailers.Get(FunctionErrorTypeTrailer), writer.Header().Get(FunctionErrorTypeTrailer))
	require.Equal(t, trailers.Get(FunctionErrorBodyTrailer), writer.Header().Get(FunctionErrorBodyTrailer))
	require.Equal(t, EndOfResponseOversized, writer.Header().Get(EndOfResponseTrailer))
	<-testFinished
}

// TODO: mock asyncPayloadCopy and force it to return Oversized in copyDone
func TestSendStreamingInvokeResponseOversizedRuntimesWithoutErrorTypeTrailer(t *testing.T) {
	oversizedPayloadString := strings.Repeat("a", int(MaxDirectResponseSize)+1)
	payload := NewReader(oversizedPayloadString)
	trailers := http.Header{
		FunctionErrorTypeTrailer: []string{""},
		FunctionErrorBodyTrailer: []string{"RuntimesErrorBody"},
	}
	writer := NewSimpleResponseWriter()
	interruptedResponseChan := make(chan *interop.Reset)
	sendResponseChan := make(chan *interop.InvokeResponseMetrics)
	testFinished := make(chan struct{})

	go func() {
		err := sendStreamingInvokeResponse(payload, trailers, writer, interruptedResponseChan, sendResponseChan, nil, false)
		require.Error(t, err)
		require.IsType(t, &interop.ErrorResponseTooLargeDI{}, err)
		testFinished <- struct{}{}
	}()

	<-sendResponseChan
	require.Equal(t, "Function.ResponseSizeTooLarge", writer.Header().Get(FunctionErrorTypeTrailer))
	require.Equal(t, trailers.Get(FunctionErrorBodyTrailer), writer.Header().Get(FunctionErrorBodyTrailer))
	require.Equal(t, EndOfResponseOversized, writer.Header().Get(EndOfResponseTrailer))
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

func TestIsStreamingInvokeTrue(t *testing.T) {
	fallbackFlag := -1
	reponseForFallback := isStreamingInvoke(fallbackFlag, interop.InvokeResponseModeBuffered)

	require.True(t, reponseForFallback)

	nonFallbackFlag := 1
	reponseForResponseMode := isStreamingInvoke(nonFallbackFlag, interop.InvokeResponseModeStreaming)

	require.True(t, reponseForResponseMode)
}

func TestIsStreamingInvokeFalse(t *testing.T) {
	nonFallbackFlag := 1
	response := isStreamingInvoke(nonFallbackFlag, interop.InvokeResponseModeBuffered)

	require.False(t, response)
}

func TestMapCopyDoneResultErrorToErrorType(t *testing.T) {
	require.Equal(t, fatalerror.TruncatedResponse, mapCopyDoneResultErrorToErrorType(&interop.ErrTruncatedResponse{}))
	require.Equal(t, fatalerror.FunctionOversizedResponse, mapCopyDoneResultErrorToErrorType(&interop.ErrorResponseTooLargeDI{}))
	require.Equal(t, fatalerror.SandboxFailure, mapCopyDoneResultErrorToErrorType(errors.New("")))
}

func TestConvertToInvokeResponseMode(t *testing.T) {
	response, err := convertToInvokeResponseMode("buffered")
	require.Equal(t, interop.InvokeResponseModeBuffered, response)
	require.Nil(t, err)

	response, err = convertToInvokeResponseMode("streaming")
	require.Equal(t, interop.InvokeResponseModeStreaming, response)
	require.Nil(t, err)

	response, err = convertToInvokeResponseMode("foo-bar")
	require.Equal(t, interop.InvokeResponseMode(""), response)
	require.Equal(t, interop.ErrInvalidInvokeResponseMode, err)
}

func FuzzReceiveDirectInvoke(f *testing.F) {
	testCustHeaders := CustomerHeaders{
		CognitoIdentityID:     "id1",
		CognitoIdentityPoolID: "id2",
		ClientContext:         "clientcontext1",
	}
	custHeadersJSON := testCustHeaders.Dump()

	f.Add([]byte{'a'}, "res-token", "invokeid", "functionarn", "versionid", "contenttype",
		custHeadersJSON, "1000",
		"Streaming", fmt.Sprint(interop.MinResponseBandwidthRate), fmt.Sprint(interop.MinResponseBandwidthBurstSize))
	f.Add([]byte{'b'}, "res-token", "invokeid", "functionarn", "versionid", "contenttype",
		custHeadersJSON, "2000", "Buffered",
		"0", "0")
	f.Add([]byte{'0'}, "0", "0", "0", "0", "0",
		"", "", "0",
		"0", "0")

	f.Fuzz(func(
		t *testing.T,
		payload []byte,
		reservationToken string,
		invokeID string,
		invokedFunctionArn string,
		versionID string,
		contentType string,
		custHeadersStr string,
		maxPayloadSizeStr string,
		invokeResponseModeStr string,
		responseBandwidthRateStr string,
		responseBandwidthBurstSizeStr string,
	) {
		request := makeDirectInvokeRequest(payload, reservationToken, invokeID,
			invokedFunctionArn, versionID, contentType, custHeadersStr, maxPayloadSizeStr,
			invokeResponseModeStr, responseBandwidthRateStr, responseBandwidthBurstSizeStr)

		token := createDummyToken()
		responseRecorder := httptest.NewRecorder()

		receivedInvoke, err := ReceiveDirectInvoke(responseRecorder, request, token)

		// default values used if header values are empty
		responseMode := interop.InvokeResponseModeBuffered
		maxDirectResponseSize := interop.MaxPayloadSize

		custHeaders := CustomerHeaders{}

		if err != nil {
			if err = custHeaders.Load(custHeadersStr); err != nil {
				assertBadRequestErrorType(t, responseRecorder, interop.ErrMalformedCustomerHeaders)
				return
			}

			if !isValidMaxPayloadSize(maxPayloadSizeStr) {
				assertBadRequestErrorType(t, responseRecorder, interop.ErrInvalidMaxPayloadSize)
				return
			}

			n, _ := strconv.ParseInt(maxPayloadSizeStr, 10, 64)
			maxDirectResponseSize = int(n)

			if invokeResponseModeStr != "" {
				if responseMode, err = convertToInvokeResponseMode(invokeResponseModeStr); err != nil {
					assertBadRequestErrorType(t, responseRecorder, interop.ErrInvalidInvokeResponseMode)
					return
				}
			}

			if isStreamingInvoke(maxDirectResponseSize, responseMode) {
				if !isValidResponseBandwidthRate(responseBandwidthRateStr) {
					assertBadRequestErrorType(t, responseRecorder, interop.ErrInvalidResponseBandwidthRate)
					return
				}

				if !isValidResponseBandwidthBurstSize(responseBandwidthBurstSizeStr) {
					assertBadRequestErrorType(t, responseRecorder, interop.ErrInvalidResponseBandwidthBurstSize)
					return
				}
			}

		} else {
			if isStreamingInvoke(maxDirectResponseSize, responseMode) {
				// FIXME
				// Until WorkerProxy stops sending MaxDirectResponseSize == -1 to identify streaming
				// invokes, the ReceiveDirectInvoke() implementation overrides InvokeResponseMode
				// to avoid setting InvokeResponseMode to buffered (default) for a streaming invoke (MaxDirectResponseSize == -1).
				responseMode = interop.InvokeResponseModeStreaming

				assert.Equal(t, responseRecorder.Header().Values("Trailer"), []string{FunctionErrorTypeTrailer, FunctionErrorBodyTrailer})
			}

			if receivedInvoke.ID != token.InvokeID {
				assertBadRequestErrorType(t, responseRecorder, interop.ErrInvalidInvokeID)
				return
			}

			if receivedInvoke.ReservationToken != token.ReservationToken {
				assertBadRequestErrorType(t, responseRecorder, interop.ErrInvalidReservationToken)
				return
			}

			if receivedInvoke.VersionID != token.VersionID {
				assertBadRequestErrorType(t, responseRecorder, interop.ErrInvalidFunctionVersion)
				return
			}

			if now := metering.Monotime(); now > token.InvackDeadlineNs {
				assertBadRequestErrorType(t, responseRecorder, interop.ErrReservationExpired)
				return
			}

			assert.Equal(t, responseRecorder.Header().Get(VersionIDHeader), token.VersionID)
			assert.Equal(t, responseRecorder.Header().Get(ReservationTokenHeader), token.ReservationToken)
			assert.Equal(t, responseRecorder.Header().Get(InvokeIDHeader), token.InvokeID)

			expectedInvoke := &interop.Invoke{
				ID:                       invokeID,
				ReservationToken:         reservationToken,
				InvokedFunctionArn:       invokedFunctionArn,
				VersionID:                versionID,
				ContentType:              contentType,
				CognitoIdentityID:        custHeaders.CognitoIdentityID,
				CognitoIdentityPoolID:    custHeaders.CognitoIdentityPoolID,
				TraceID:                  token.TraceID,
				LambdaSegmentID:          token.LambdaSegmentID,
				ClientContext:            custHeaders.ClientContext,
				Payload:                  request.Body,
				DeadlineNs:               receivedInvoke.DeadlineNs,
				NeedDebugLogs:            token.NeedDebugLogs,
				InvokeReceivedTime:       receivedInvoke.InvokeReceivedTime,
				InvokeResponseMode:       responseMode,
				RestoreDurationNs:        token.RestoreDurationNs,
				RestoreStartTimeMonotime: token.RestoreStartTimeMonotime,
			}

			assert.Equal(t, expectedInvoke, receivedInvoke)
		}
	})
}

func createDummyToken() interop.Token {
	return interop.Token{
		ReservationToken: "reservation_token",
		TraceID:          "trace_id",
		InvokeID:         "invoke_id",
		InvackDeadlineNs: math.MaxInt64,
		VersionID:        "version_id",
	}
}

func assertBadRequestErrorType(t *testing.T, responseRecorder *httptest.ResponseRecorder, expectedErrType error) {
	assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

	assert.Equal(t, expectedErrType.Error(), responseRecorder.Header().Get(ErrorTypeHeader))
	assert.Equal(t, EndOfResponseComplete, responseRecorder.Header().Get(EndOfResponseTrailer))
}

func isValidResponseBandwidthBurstSize(sizeStr string) bool {
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	return err == nil &&
		interop.MinResponseBandwidthBurstSize <= size && size <= interop.MaxResponseBandwidthBurstSize
}

func isValidResponseBandwidthRate(rateStr string) bool {
	rate, err := strconv.ParseInt(rateStr, 10, 64)
	return err == nil &&
		interop.MinResponseBandwidthRate <= rate && rate <= interop.MaxResponseBandwidthRate
}

func isValidMaxPayloadSize(maxPayloadSizeStr string) bool {
	if maxPayloadSizeStr != "" {
		maxPayloadSize, err := strconv.ParseInt(maxPayloadSizeStr, 10, 64)
		return err == nil && maxPayloadSize >= -1
	}

	return true
}

func makeDirectInvokeRequest(
	payload []byte, reservationToken string, invokeID string, invokedFunctionArn string,
	versionID string, contentType string, custHeadersStr string, maxPayloadSize string,
	invokeResponseModeStr string, responseBandwidthRate string, responseBandwidthBurstSize string,
) *http.Request {
	request := httptest.NewRequest("POST", "http://example.com/", bytes.NewReader(payload))
	request = addReservationToken(request, reservationToken)

	request.Header.Set(InvokeIDHeader, invokeID)
	request.Header.Set(InvokedFunctionArnHeader, invokedFunctionArn)
	request.Header.Set(VersionIDHeader, versionID)
	request.Header.Set(ContentTypeHeader, contentType)
	request.Header.Set(CustomerHeadersHeader, custHeadersStr)
	request.Header.Set(MaxPayloadSizeHeader, maxPayloadSize)
	request.Header.Set(InvokeResponseModeHeader, invokeResponseModeStr)
	request.Header.Set(ResponseBandwidthRateHeader, responseBandwidthRate)
	request.Header.Set(ResponseBandwidthBurstSizeHeader, responseBandwidthBurstSize)

	return request
}

func addReservationToken(r *http.Request, reservationToken string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("reservationtoken", reservationToken)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
