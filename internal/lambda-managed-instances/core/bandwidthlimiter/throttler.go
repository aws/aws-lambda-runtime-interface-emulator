// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package bandwidthlimiter

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
)

var ErrBufferSizeTooLarge = errors.New("buffer size cannot be greater than bucket size")

func NewBucket(capacity int64, initialTokenCount int64, refillNumber int64, refillInterval time.Duration) (*Bucket, error) {
	if capacity <= 0 || initialTokenCount < 0 || refillNumber <= 0 || refillInterval <= 0 ||
		capacity < initialTokenCount {
		errorMsg := fmt.Sprintf("invalid bucket parameters (capacity: %d, initialTokenCount: %d, refillNumber: %d,"+
			"refillInterval: %d)", capacity, initialTokenCount, refillInterval, refillInterval)
		slog.Error(errorMsg)
		return nil, errors.New(errorMsg)
	}
	return &Bucket{
		capacity:       capacity,
		tokenCount:     initialTokenCount,
		refillNumber:   refillNumber,
		refillInterval: refillInterval,
		mutex:          sync.Mutex{},
	}, nil
}

type Bucket struct {
	capacity       int64
	tokenCount     int64
	refillNumber   int64
	refillInterval time.Duration
	mutex          sync.Mutex
}

func (b *Bucket) produceTokens() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.tokenCount < b.capacity {
		b.tokenCount = min(b.tokenCount+b.refillNumber, b.capacity)
	}
}

func (b *Bucket) consumeTokens(n int64) bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if n <= b.tokenCount {
		b.tokenCount -= n
		return true
	}
	return false
}

func (b *Bucket) getTokenCount() int64 {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.tokenCount
}

func NewThrottler(bucket *Bucket) (*Throttler, error) {
	if bucket == nil {
		errorMsg := "cannot create a throttler with nil bucket"
		slog.Error(errorMsg)
		return nil, errors.New(errorMsg)
	}

	now := time.Now()

	return &Throttler{
		b:        bucket,
		running:  false,
		produced: make(chan time.Time),
		done:     make(chan struct{}),

		metrics: &interop.InvokeResponseMetrics{
			StartReadingResponseTime:  now,
			FinishReadingResponseTime: now,
			OutboundThroughputBps:     -1,
			FunctionResponseMode:      interop.FunctionResponseModeStreaming,
		},
	}, nil
}

type Throttler struct {
	b        *Bucket
	running  bool
	produced chan time.Time
	done     chan struct{}
	metrics  *interop.InvokeResponseMetrics
}

func (th *Throttler) start() {
	if th.running {
		return
	}
	th.running = true
	th.metrics.StartReadingResponseTime = time.Now()
	go func() {
		ticker := time.NewTicker(th.b.refillInterval)
		for {
			select {
			case <-ticker.C:
				th.b.produceTokens()
				select {
				case th.produced <- time.Now():
				default:
				}
			case <-th.done:
				ticker.Stop()
				return
			}
		}
	}()
}

func (th *Throttler) stop() {
	if !th.running {
		return
	}
	th.running = false
	th.metrics.FinishReadingResponseTime = time.Now()
	duration := th.metrics.StartReadingResponseTime.Sub(th.metrics.FinishReadingResponseTime)
	if duration > 0 {
		th.metrics.OutboundThroughputBps = (th.metrics.ProducedBytes / duration.Milliseconds()) * int64(time.Second/time.Millisecond)
	} else {
		th.metrics.OutboundThroughputBps = -1
	}
	th.done <- struct{}{}
}

func (th *Throttler) bandwidthLimitingWrite(w io.Writer, p []byte) (written int, err error) {
	n := int64(len(p))
	if n > th.b.capacity {
		return 0, ErrBufferSizeTooLarge
	}
	for {
		if th.b.consumeTokens(n) {
			written, err = w.Write(p)
			th.metrics.ProducedBytes += int64(written)
			return written, err
		}
		waitStart := time.Now()
		elapsed := (<-th.produced).Sub(waitStart)
		if elapsed > 0 {
			th.metrics.TimeShaped += elapsed
		}
	}
}
