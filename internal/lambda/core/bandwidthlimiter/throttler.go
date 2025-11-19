// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package bandwidthlimiter

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/metering"
)

var ErrBufferSizeTooLarge = errors.New("buffer size cannot be greater than bucket size")

func NewBucket(capacity int64, initialTokenCount int64, refillNumber int64, refillInterval time.Duration) (*Bucket, error) {
	if capacity <= 0 || initialTokenCount < 0 || refillNumber <= 0 || refillInterval <= 0 ||
		capacity < initialTokenCount {
		errorMsg := fmt.Sprintf("invalid bucket parameters (capacity: %d, initialTokenCount: %d, refillNumber: %d,"+
			"refillInterval: %d)", capacity, initialTokenCount, refillInterval, refillInterval)
		log.Error(errorMsg)
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
		b.tokenCount = min64(b.tokenCount+b.refillNumber, b.capacity)
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
		log.Error(errorMsg)
		return nil, errors.New(errorMsg)
	}
	return &Throttler{
		b:        bucket,
		running:  false,
		produced: make(chan int64),
		done:     make(chan struct{}),
		// FIXME:
		// The runtime tells whether the function response mode is streaming or not.
		// Ideally, we would want to use that value here. Since I'm just rebasing, I will leave
		// as-is, but we should use that instead of relying on our memory to set this here
		// because we "know" it's a streaming code path.
		metrics: &interop.InvokeResponseMetrics{FunctionResponseMode: interop.FunctionResponseModeStreaming},
	}, nil
}

type Throttler struct {
	b        *Bucket
	running  bool
	produced chan int64
	done     chan struct{}
	metrics  *interop.InvokeResponseMetrics
}

func (th *Throttler) start() {
	if th.running {
		return
	}
	th.running = true
	th.metrics.StartReadingResponseMonoTimeMs = metering.Monotime()
	go func() {
		ticker := time.NewTicker(th.b.refillInterval)
		for {
			select {
			case <-ticker.C:
				th.b.produceTokens()
				select {
				case th.produced <- metering.Monotime():
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
	th.metrics.FinishReadingResponseMonoTimeMs = metering.Monotime()
	durationMs := (th.metrics.FinishReadingResponseMonoTimeMs - th.metrics.StartReadingResponseMonoTimeMs) / int64(time.Millisecond)
	if durationMs > 0 {
		th.metrics.OutboundThroughputBps = (th.metrics.ProducedBytes / durationMs) * int64(time.Second/time.Millisecond)
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
			return
		}
		waitStart := metering.Monotime()
		elapsed := <-th.produced - waitStart
		if elapsed > 0 {
			th.metrics.TimeShapedNs += elapsed
		}
	}
}
