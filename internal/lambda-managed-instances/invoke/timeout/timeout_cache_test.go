// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package timeout_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/invoke/timeout"
)

func TestRecentCache(t *testing.T) {
	cache := timeout.NewRecentCache()

	assert.False(t, cache.Consume("invoke-0"))

	cache.Register("invoke-0")
	assert.True(t, cache.Consume("invoke-0"))

	assert.False(t, cache.Consume("invoke-0"))

	for i := range 1000 {
		cache.Register(fmt.Sprintf("invoke-%d", i))
	}

	cache.Register("invoke-0")

	cache.Register("invoke-1000")
	assert.False(t, cache.Consume("invoke-1"))

	assert.True(t, cache.Consume("invoke-0"))
}

func TestRecentCache_Concurrency(t *testing.T) {
	cache := timeout.NewRecentCache()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup

	for i := range 2000 {
		invokeID := fmt.Sprintf("invoke-%d", i)

		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:

				}

				cache.Register(invokeID)
			}
		})

		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:

				}

				cache.Consume(invokeID)
			}
		})
	}

	wg.Wait()
}
