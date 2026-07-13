// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
)

type reservedEntry struct {
	runtime runningInvoke
	timer   *time.Timer
}

type RuntimePool struct {
	mu       sync.Mutex
	idle     []runningInvoke
	reserved map[interop.InvokeID]*reservedEntry
	maxSize  int
}

func NewRuntimePool(maxSize int) *RuntimePool {
	return &RuntimePool{
		idle:     make([]runningInvoke, 0, maxSize),
		reserved: make(map[interop.InvokeID]*reservedEntry),
		maxSize:  maxSize,
	}
}

func (p *RuntimePool) Add(runtime runningInvoke) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.idle)+len(p.reserved) >= p.maxSize {
		return errTooManyIdleRuntimes
	}

	p.idle = append(p.idle, runtime)
	return nil
}

func (p *RuntimePool) Reserve(invokeID interop.InvokeID, timeout time.Duration, onExpire func()) error {
	lockStart := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()

	if lockWait := time.Since(lockStart); lockWait > time.Millisecond {
		slog.Warn("RuntimePool.Reserve lock contention", "wait_us", lockWait.Microseconds())
	}

	if _, exists := p.reserved[invokeID]; exists {
		return ErrInvokeIdAlreadyExists
	}

	rt, err := p.dequeueIdle()
	if err != nil {
		return err
	}

	p.reserved[invokeID] = &reservedEntry{
		runtime: rt,
		timer:   time.AfterFunc(timeout, onExpire),
	}

	return nil
}

func (p *RuntimePool) Acquire(invokeID interop.InvokeID) (runningInvoke, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if entry, ok := p.reserved[invokeID]; ok {
		delete(p.reserved, invokeID)
		if entry.timer != nil {
			entry.timer.Stop()
		}
		return entry.runtime, true, nil
	}

	rt, err := p.dequeueIdle()
	if err != nil {
		return nil, false, err
	}
	return rt, false, nil
}

func (p *RuntimePool) dequeueIdle() (runningInvoke, error) {
	if len(p.idle) == 0 {
		return nil, ErrInvokeNoReadyRuntime
	}
	rt := p.idle[0]
	p.idle[0] = nil
	p.idle = p.idle[1:]
	return rt, nil
}

func (p *RuntimePool) ExpireReservation(invokeID interop.InvokeID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry, ok := p.reserved[invokeID]
	if !ok {
		return false
	}

	delete(p.reserved, invokeID)
	p.idle = append(p.idle, entry.runtime)
	return true
}

type RuntimePoolCounts struct {
	Idle     int
	Reserved int
	Total    int
}

func (p *RuntimePool) Counts() RuntimePoolCounts {
	p.mu.Lock()
	defer p.mu.Unlock()
	idle := len(p.idle)
	reserved := len(p.reserved)
	return RuntimePoolCounts{Idle: idle, Reserved: reserved, Total: idle + reserved}
}

func (p *RuntimePool) ReservedCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.reserved)
}

var errTooManyIdleRuntimes = errors.New("too many idle runtimes")
