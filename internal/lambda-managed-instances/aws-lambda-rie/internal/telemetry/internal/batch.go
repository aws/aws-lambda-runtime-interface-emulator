// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"encoding/json"
	"time"
)

type batch struct {
	events    []json.RawMessage
	sizeBytes int
	flushAt   <-chan time.Time
	bufCfg    BufferingConfig
	doneCh    chan struct{}
}

func newBatch(bufCfg BufferingConfig) *batch {
	return &batch{
		bufCfg:  bufCfg,
		flushAt: time.After(time.Duration(bufCfg.Timeout)),
		doneCh:  make(chan struct{}),
	}
}

func (b *batch) addEvent(event json.RawMessage) (full bool) {
	b.events = append(b.events, event)
	b.sizeBytes += len(event)
	return b.isFull()
}

func (b *batch) isFull() bool {
	if len(b.events) >= b.bufCfg.MaxItems {
		return true
	}
	if b.sizeBytes >= b.bufCfg.MaxBytes {
		return true
	}
	return false
}
