// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package bandwidthlimiter

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBucket(t *testing.T) {
	type testCase struct {
		capacity          int64
		initialTokenCount int64
		refillNumber      int64
		refillInterval    time.Duration
		bucketCreated     bool
	}
	testCases := []testCase{
		{capacity: 8, initialTokenCount: 6, refillNumber: 2, refillInterval: 100 * time.Millisecond, bucketCreated: true},
		{capacity: 8, initialTokenCount: 6, refillNumber: 2, refillInterval: -100 * time.Millisecond, bucketCreated: false},
		{capacity: 8, initialTokenCount: 6, refillNumber: -5, refillInterval: 100 * time.Millisecond, bucketCreated: false},
		{capacity: 8, initialTokenCount: -2, refillNumber: 2, refillInterval: 100 * time.Millisecond, bucketCreated: false},
		{capacity: -2, initialTokenCount: 6, refillNumber: 2, refillInterval: 100 * time.Millisecond, bucketCreated: false},
		{capacity: 8, initialTokenCount: 10, refillNumber: 2, refillInterval: 100 * time.Millisecond, bucketCreated: false},
	}

	for _, test := range testCases {
		bucket, err := NewBucket(test.capacity, test.initialTokenCount, test.refillNumber, test.refillInterval)
		if test.bucketCreated {
			assert.NoError(t, err)
			assert.NotNil(t, bucket)
		} else {
			assert.Error(t, err)
			assert.Nil(t, bucket)
		}
	}
}

func TestBucket_produceTokens_consumeTokens(t *testing.T) {
	var consumed bool
	bucket, err := NewBucket(16, 8, 6, 100*time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, int64(8), bucket.getTokenCount())

	consumed = bucket.consumeTokens(5)
	assert.Equal(t, int64(3), bucket.getTokenCount())
	assert.True(t, consumed)

	bucket.produceTokens()
	assert.Equal(t, int64(9), bucket.getTokenCount())

	bucket.produceTokens()
	assert.Equal(t, int64(15), bucket.getTokenCount())

	bucket.produceTokens()
	assert.Equal(t, int64(16), bucket.getTokenCount())

	bucket.produceTokens()
	assert.Equal(t, int64(16), bucket.getTokenCount())

	consumed = bucket.consumeTokens(18)
	assert.Equal(t, int64(16), bucket.getTokenCount())
	assert.False(t, consumed)

	consumed = bucket.consumeTokens(16)
	assert.Equal(t, int64(0), bucket.getTokenCount())
	assert.True(t, consumed)
}

func TestNewThrottler(t *testing.T) {
	bucket, err := NewBucket(16, 8, 6, 100*time.Millisecond)
	assert.NoError(t, err)

	throttler, err := NewThrottler(bucket)
	assert.NoError(t, err)
	assert.NotNil(t, throttler)

	throttler, err = NewThrottler(nil)
	assert.Error(t, err)
	assert.Nil(t, throttler)
}

func TestNewThrottler_start_stop(t *testing.T) {
	bucket, err := NewBucket(16, 8, 6, 100*time.Millisecond)
	assert.NoError(t, err)

	throttler, err := NewThrottler(bucket)
	assert.NoError(t, err)

	assert.False(t, throttler.running)

	throttler.start()
	assert.True(t, throttler.running)

	<-time.Tick(2 * throttler.b.refillInterval)
	assert.LessOrEqual(t, int64(14), throttler.b.getTokenCount())
	assert.True(t, throttler.running)

	throttler.start()
	assert.True(t, throttler.running)
	<-time.Tick(2 * throttler.b.refillInterval)
	assert.Equal(t, int64(16), throttler.b.getTokenCount())
	assert.True(t, throttler.running)

	throttler.stop()
	assert.False(t, throttler.running)

	throttler.stop()
	assert.False(t, throttler.running)

	throttler.start()
	assert.True(t, throttler.running)

	throttler.stop()
	assert.False(t, throttler.running)
}

type ByteBufferWriter interface {
	Write(p []byte) (n int, err error)
	Bytes() []byte
}

type FixedSizeBufferWriter struct {
	buf []byte
}

func (w *FixedSizeBufferWriter) Write(p []byte) (n int, err error) {
	n = copy(w.buf, p)
	return
}

func (w *FixedSizeBufferWriter) Bytes() []byte {
	return w.buf
}

func TestNewThrottler_bandwidthLimitingWrite(t *testing.T) {
	var size10mb int64 = 10 * 1024 * 1024

	type testCase struct {
		capacity          int64
		initialTokenCount int64
		writer            ByteBufferWriter
		inputBuffer       []byte
		expectedN         int
		expectedError     error
	}
	testCases := []testCase{
		{
			capacity:          16,
			initialTokenCount: 8,
			writer:            bytes.NewBuffer(make([]byte, 0, 14)),
			inputBuffer:       []byte(strings.Repeat("a", 12)),
			expectedN:         12,
			expectedError:     nil,
		},
		{
			capacity:          16,
			initialTokenCount: 8,
			writer:            bytes.NewBuffer(make([]byte, 0, 12)),
			inputBuffer:       []byte(strings.Repeat("a", 14)),
			expectedN:         14,
			expectedError:     nil,
		},
		{
			capacity:          size10mb,
			initialTokenCount: size10mb,
			writer:            bytes.NewBuffer(make([]byte, 0, size10mb)),
			inputBuffer:       []byte(strings.Repeat("a", int(size10mb))),
			expectedN:         int(size10mb),
			expectedError:     nil,
		},
		{
			capacity:          16,
			initialTokenCount: 8,
			writer:            bytes.NewBuffer(make([]byte, 0, 18)),
			inputBuffer:       []byte(strings.Repeat("a", 18)),
			expectedN:         0,
			expectedError:     ErrBufferSizeTooLarge,
		},
		{
			capacity:          16,
			initialTokenCount: 8,
			writer:            &FixedSizeBufferWriter{buf: make([]byte, 12)},
			inputBuffer:       []byte(strings.Repeat("a", 14)),
			expectedN:         12,
			expectedError:     nil,
		},
	}

	for _, test := range testCases {
		bucket, err := NewBucket(test.capacity, test.initialTokenCount, 2, 100*time.Millisecond)
		assert.NoError(t, err)

		throttler, err := NewThrottler(bucket)
		assert.NoError(t, err)

		writer := test.writer
		throttler.start()
		n, err := throttler.bandwidthLimitingWrite(writer, test.inputBuffer)
		assert.Equal(t, test.expectedN, n)
		assert.Equal(t, test.expectedError, err)

		if test.expectedError == nil {
			assert.Equal(t, test.inputBuffer[:n], test.writer.Bytes())
		} else {
			assert.Equal(t, []byte{}, test.writer.Bytes())
		}
		throttler.stop()
	}
}
