// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invoke

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
)

func TestRuntimePool_Add_Success(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(3)

	err := pool.Add(newMockRunningInvoke(t))
	require.NoError(t, err)
	assert.Equal(t, 1, pool.Counts().Total)
}

func TestRuntimePool_Add_AtCapacity(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(2)

	require.NoError(t, pool.Add(newMockRunningInvoke(t)))
	require.NoError(t, pool.Add(newMockRunningInvoke(t)))

	err := pool.Add(newMockRunningInvoke(t))
	assert.ErrorIs(t, err, errTooManyIdleRuntimes)
	assert.Equal(t, 2, pool.Counts().Total)
}

func TestRuntimePool_Reserve_Success(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(5)

	require.NoError(t, pool.Add(newMockRunningInvoke(t)))

	err := pool.Reserve("invoke-1", 100*time.Millisecond, func() {})
	require.NoError(t, err)

	assert.Equal(t, 1, pool.Counts().Total)
	assert.Equal(t, 1, pool.ReservedCount())
}

func TestRuntimePool_Reserve_NoIdleRuntimes(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(5)

	err := pool.Reserve("invoke-1", 100*time.Millisecond, func() {})
	assert.ErrorIs(t, err, ErrInvokeNoReadyRuntime)
	assert.Equal(t, 0, pool.ReservedCount())
}

func TestRuntimePool_Reserve_DuplicateInvokeID(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(5)

	require.NoError(t, pool.Add(newMockRunningInvoke(t)))
	require.NoError(t, pool.Add(newMockRunningInvoke(t)))

	err := pool.Reserve("invoke-dup", 100*time.Millisecond, func() {})
	require.NoError(t, err)

	err = pool.Reserve("invoke-dup", 100*time.Millisecond, func() {})
	assert.ErrorIs(t, err, ErrInvokeIdAlreadyExists)

	assert.Equal(t, 2, pool.Counts().Total)
	assert.Equal(t, 1, pool.ReservedCount())
}

func TestRuntimePool_Reserve_OnlyIdleAreReserved(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(5)

	require.NoError(t, pool.Add(newMockRunningInvoke(t)))

	err := pool.Reserve("invoke-1", 100*time.Millisecond, func() {})
	require.NoError(t, err)

	err = pool.Reserve("invoke-2", 100*time.Millisecond, func() {})
	assert.ErrorIs(t, err, ErrInvokeNoReadyRuntime)
}

func TestRuntimePool_Acquire_WithReservation(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(5)

	rt := newMockRunningInvoke(t)
	require.NoError(t, pool.Add(rt))

	err := pool.Reserve("invoke-acq", 500*time.Millisecond, func() {})
	require.NoError(t, err)

	acquired, wasReserved, acquireErr := pool.Acquire("invoke-acq")
	require.NoError(t, acquireErr)
	assert.Equal(t, rt, acquired)
	assert.True(t, wasReserved)

	assert.Equal(t, 0, pool.Counts().Total)
	assert.Equal(t, 0, pool.ReservedCount())
}

func TestRuntimePool_Acquire_FallbackToIdle(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(5)

	rt := newMockRunningInvoke(t)
	require.NoError(t, pool.Add(rt))

	acquired, wasReserved, err := pool.Acquire("no-reservation")
	require.NoError(t, err)
	assert.Equal(t, rt, acquired)
	assert.False(t, wasReserved)

	assert.Equal(t, 0, pool.Counts().Total)
}

func TestRuntimePool_Acquire_NoRuntimesAvailable(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(5)

	_, _, err := pool.Acquire("empty-pool")
	assert.ErrorIs(t, err, ErrInvokeNoReadyRuntime)
}

func TestRuntimePool_ExpireReservation_ReturnsToIdle(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(5)

	require.NoError(t, pool.Add(newMockRunningInvoke(t)))

	err := pool.Reserve("invoke-exp", 500*time.Millisecond, func() {})
	require.NoError(t, err)
	assert.Equal(t, 1, pool.ReservedCount())

	expired := pool.ExpireReservation("invoke-exp")
	assert.True(t, expired)

	assert.Equal(t, 1, pool.Counts().Total)
	assert.Equal(t, 0, pool.ReservedCount())
}

func TestRuntimePool_ExpireReservation_AlreadyConsumed(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(5)

	require.NoError(t, pool.Add(newMockRunningInvoke(t)))

	err := pool.Reserve("invoke-consumed", 500*time.Millisecond, func() {})
	require.NoError(t, err)

	_, _, acquireErr := pool.Acquire("invoke-consumed")
	require.NoError(t, acquireErr)

	expired := pool.ExpireReservation("invoke-consumed")
	assert.False(t, expired)
	assert.Equal(t, 0, pool.Counts().Total)
	assert.Equal(t, 0, pool.ReservedCount())
}

func TestRuntimePool_Reserve_TimerFiresExpiration(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(5)

	require.NoError(t, pool.Add(newMockRunningInvoke(t)))

	expired := make(chan struct{})
	err := pool.Reserve("invoke-timer", 20*time.Millisecond, func() {
		pool.ExpireReservation("invoke-timer")
		close(expired)
	})
	require.NoError(t, err)
	assert.Equal(t, 1, pool.ReservedCount())

	<-expired

	assert.Equal(t, 1, pool.Counts().Total)
	assert.Equal(t, 0, pool.ReservedCount())
}

func TestRuntimePool_Acquire_ReservationStopsTimer(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(5)

	require.NoError(t, pool.Add(newMockRunningInvoke(t)))

	expireCalled := false
	err := pool.Reserve("invoke-stop-timer", 50*time.Millisecond, func() {
		expireCalled = true
	})
	require.NoError(t, err)

	_, _, acquireErr := pool.Acquire("invoke-stop-timer")
	require.NoError(t, acquireErr)

	time.Sleep(100 * time.Millisecond)
	assert.False(t, expireCalled, "expiration callback should not fire after Acquire")
}

func TestRuntimePool_Add_AfterExpiration_ReclaimsCapacity(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(1)

	require.NoError(t, pool.Add(newMockRunningInvoke(t)))

	expired := make(chan struct{})
	err := pool.Reserve("invoke-cap", 20*time.Millisecond, func() {
		pool.ExpireReservation("invoke-cap")
		close(expired)
	})
	require.NoError(t, err)

	assert.Equal(t, 1, pool.Counts().Total)
	assert.Error(t, pool.Add(newMockRunningInvoke(t)))

	<-expired

	assert.Equal(t, 1, pool.Counts().Total)

	_, _, acquireErr := pool.Acquire("some-invoke")
	require.NoError(t, acquireErr)

	require.NoError(t, pool.Add(newMockRunningInvoke(t)))
}

func TestRuntimePool_ConcurrentReserveAndAcquire(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(100)

	for i := 0; i < 50; i++ {
		require.NoError(t, pool.Add(newMockRunningInvoke(t)))
	}

	var wg sync.WaitGroup
	reserveErrors := make(chan error, 50)
	acquireErrors := make(chan error, 50)

	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			invokeID := interop.InvokeID(fmt.Sprintf("concurrent-%d", id))
			err := pool.Reserve(invokeID, 200*time.Millisecond, func() {
				pool.ExpireReservation(invokeID)
			})
			reserveErrors <- err
		}(i)
	}

	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			invokeID := interop.InvokeID(fmt.Sprintf("concurrent-%d", id))
			_, _, err := pool.Acquire(invokeID)
			acquireErrors <- err
		}(i)
	}

	wg.Wait()
	close(reserveErrors)
	close(acquireErrors)

	var reserveSuccesses, acquireSuccesses int
	for err := range reserveErrors {
		if err == nil {
			reserveSuccesses++
		}
	}
	for err := range acquireErrors {
		if err == nil {
			acquireSuccesses++
		}
	}

	assert.LessOrEqual(t, reserveSuccesses+acquireSuccesses, 50)
	t.Logf("reserve successes: %d, acquire successes: %d", reserveSuccesses, acquireSuccesses)
}

func TestRuntimePool_Reserve_DrainsIdleIntoReserved(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(5)

	require.NoError(t, pool.Add(newMockRunningInvoke(t)))
	require.NoError(t, pool.Add(newMockRunningInvoke(t)))
	require.NoError(t, pool.Add(newMockRunningInvoke(t)))

	err := pool.Reserve("r1", 500*time.Millisecond, func() {})
	require.NoError(t, err)
	err = pool.Reserve("r2", 500*time.Millisecond, func() {})
	require.NoError(t, err)

	assert.Equal(t, 3, pool.Counts().Total)
	assert.Equal(t, 2, pool.ReservedCount())

	_, _, acquireErr := pool.Acquire("no-reservation")
	require.NoError(t, acquireErr)

	assert.Equal(t, 2, pool.Counts().Total)
	assert.Equal(t, 2, pool.ReservedCount())
}

func TestRuntimePool_ExpireReservation_MovesBackToIdle(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(5)

	rt := newMockRunningInvoke(t)
	require.NoError(t, pool.Add(rt))

	err := pool.Reserve("expire-back", 500*time.Millisecond, func() {})
	require.NoError(t, err)

	assert.Equal(t, 1, pool.ReservedCount())

	expired := pool.ExpireReservation("expire-back")
	assert.True(t, expired)

	assert.Equal(t, 0, pool.ReservedCount())
	assert.Equal(t, 1, pool.Counts().Total)

	acquired, wasReserved, acquireErr := pool.Acquire("different-invoke")
	require.NoError(t, acquireErr)
	assert.Equal(t, rt, acquired)
	assert.False(t, wasReserved)
}

func TestRuntimePool_Add_RespectsSharedCapacity(t *testing.T) {
	t.Parallel()
	pool := NewRuntimePool(3)

	require.NoError(t, pool.Add(newMockRunningInvoke(t)))
	require.NoError(t, pool.Add(newMockRunningInvoke(t)))
	require.NoError(t, pool.Add(newMockRunningInvoke(t)))

	err := pool.Reserve("cap-test", 500*time.Millisecond, func() {})
	require.NoError(t, err)

	assert.Error(t, pool.Add(newMockRunningInvoke(t)))

	_, _, acquireErr := pool.Acquire("cap-test")
	require.NoError(t, acquireErr)

	require.NoError(t, pool.Add(newMockRunningInvoke(t)))
}
