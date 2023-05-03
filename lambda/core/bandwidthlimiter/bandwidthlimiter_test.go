// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package bandwidthlimiter

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBandwidthLimitingCopy(t *testing.T) {
	var size10mb int64 = 10 * 1024 * 1024

	inputBuffer := []byte(strings.Repeat("a", int(size10mb)))
	reader := bytes.NewReader(inputBuffer)

	bucket, err := NewBucket(size10mb/2, size10mb/4, size10mb/2, time.Millisecond/2)
	assert.NoError(t, err)

	internalWriter := bytes.NewBuffer(make([]byte, 0, size10mb))
	writer, err := NewBandwidthLimitingWriter(internalWriter, bucket)
	assert.NoError(t, err)

	n, err := BandwidthLimitingCopy(writer, reader)
	assert.Equal(t, size10mb, n)
	assert.Equal(t, nil, err)
	assert.Equal(t, inputBuffer, internalWriter.Bytes())
}

type ErrorBufferWriter struct {
	w         ByteBufferWriter
	failAfter int
}

func (w *ErrorBufferWriter) Write(p []byte) (n int, err error) {
	if w.failAfter >= 1 {
		w.failAfter--
	}
	n, err = w.w.Write(p)
	if w.failAfter == 0 {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}

func (w *ErrorBufferWriter) Bytes() []byte {
	return w.w.Bytes()
}

func TestNewBandwidthLimitingWriter(t *testing.T) {
	type testCase struct {
		refillNumber   int64
		internalWriter ByteBufferWriter
		inputBuffer    []byte
		expectedN      int
		expectedError  error
	}
	testCases := []testCase{
		{
			refillNumber:   2,
			internalWriter: bytes.NewBuffer(make([]byte, 0, 36)), // buffer size greater than bucket size
			inputBuffer:    []byte(strings.Repeat("a", 36)),
			expectedN:      36,
			expectedError:  nil,
		},
		{
			refillNumber:   2,
			internalWriter: bytes.NewBuffer(make([]byte, 0, 12)), // buffer size lesser than bucket size
			inputBuffer:    []byte(strings.Repeat("a", 12)),
			expectedN:      12,
			expectedError:  nil,
		},
		{
			// buffer size greater than bucket size and error after two Write() invocations
			refillNumber:   2,
			internalWriter: &ErrorBufferWriter{w: bytes.NewBuffer(make([]byte, 0, 36)), failAfter: 2},
			inputBuffer:    []byte(strings.Repeat("a", 36)),
			expectedN:      32,
			expectedError:  io.ErrUnexpectedEOF,
		},
	}

	for _, test := range testCases {
		bucket, err := NewBucket(16, 8, test.refillNumber, 100*time.Millisecond)
		assert.NoError(t, err)

		writer, err := NewBandwidthLimitingWriter(test.internalWriter, bucket)
		assert.NoError(t, err)
		assert.False(t, writer.th.running)

		n, err := writer.Write(test.inputBuffer)
		assert.True(t, writer.th.running)
		assert.Equal(t, test.expectedN, n)
		assert.Equal(t, test.expectedError, err)
		assert.Equal(t, test.inputBuffer[:n], test.internalWriter.Bytes())

		err = writer.Close()
		assert.Nil(t, err)
		assert.False(t, writer.th.running)
	}
}
