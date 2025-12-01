// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package timeout

import (
	"container/list"
	"log/slog"
	"sync"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
)

const timeoutCacheSize = 1000

type RecentCache struct {
	entries map[interop.InvokeID]*list.Element
	order   *list.List
	mu      sync.Mutex
}

func NewRecentCache() *RecentCache {
	return &RecentCache{
		entries: make(map[interop.InvokeID]*list.Element, timeoutCacheSize),
		order:   list.New(),
	}
}

func (tc *RecentCache) Register(invokeID interop.InvokeID) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if elem, ok := tc.entries[invokeID]; ok {

		tc.order.MoveToBack(elem)
		return
	}

	if len(tc.entries) == timeoutCacheSize {

		oldestInvokeID := tc.order.Front().Value.(interop.InvokeID)
		_ = tc.tryDelete(oldestInvokeID)
		slog.Warn("evicted invokeID from full timeout cache", "invokeID", oldestInvokeID)
	}

	elem := tc.order.PushBack(invokeID)
	tc.entries[invokeID] = elem
}

func (tc *RecentCache) Consume(invokeID interop.InvokeID) (consumed bool) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	return tc.tryDelete(invokeID)
}

func (tc *RecentCache) tryDelete(invokeID interop.InvokeID) (deleted bool) {
	elem, ok := tc.entries[invokeID]
	if !ok {
		return false
	}
	tc.order.Remove(elem)
	delete(tc.entries, invokeID)
	return true
}
