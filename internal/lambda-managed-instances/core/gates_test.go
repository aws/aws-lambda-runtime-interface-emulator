// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
)

func TestWalkThrough(t *testing.T) {
	g := NewGate(1)
	assert.NoError(t, g.WalkThrough(nil))
}

func TestWalkThroughTwice(t *testing.T) {
	g := NewGate(1)
	assert.NoError(t, g.WalkThrough(nil))
	assert.Equal(t, ErrGateIntegrity, g.WalkThrough(nil))
}

func TestSetCount(t *testing.T) {
	g := NewGate(2)
	assert.NoError(t, g.WalkThrough(nil))
	assert.NoError(t, g.WalkThrough(nil))
	assert.Equal(t, ErrGateIntegrity, g.SetCount(1))
	assert.NoError(t, g.SetCount(2))
	assert.Equal(t, ErrGateIntegrity, g.WalkThrough(nil))
	assert.NoError(t, g.SetCount(3))
	assert.NoError(t, g.WalkThrough(nil))
}

func TestReset(t *testing.T) {
	g := NewGate(1)
	assert.NoError(t, g.WalkThrough(nil))
	g.Reset()
	assert.NoError(t, g.WalkThrough(nil))
}

func TestCancel(t *testing.T) {
	g := NewGate(1)
	ch := make(chan error)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Done()
		_, err := g.AwaitGateCondition(context.Background())
		ch <- err
	}()

	wg.Wait()
	g.CancelWithError(nil)
	err := <-ch

	assert.Equal(t, ErrGateCanceled, err)
}

func TestCancelWithError(t *testing.T) {
	g := NewGate(1)
	ch := make(chan error)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Done()
		_, err := g.AwaitGateCondition(context.Background())
		ch <- err
	}()

	wg.Wait()
	err := errors.New("MyErr")
	g.CancelWithError(err)
	receivedErr := <-ch

	assert.Equal(t, err, receivedErr)
}

func TestUseAfterCancel(t *testing.T) {
	g := NewGate(1)
	err := errors.New("MyErr")
	g.CancelWithError(err)
	_, awaitErr := g.AwaitGateCondition(context.Background())
	assert.Equal(t, err, awaitErr)
	g.Reset()
	_, awaitErr = g.AwaitGateCondition(context.Background())
	assert.Equal(t, err, awaitErr)
}

func TestAwaitGateConditionWithDeadlineWalkthrough(t *testing.T) {
	g := NewGate(1)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	require.NoError(t, g.WalkThrough(nil))

	_, err := g.AwaitGateCondition(ctx)

	assert.Equal(t, nil, err)
}

func TestAwaitGateConditionWithDeadlineTimeout(t *testing.T) {
	g := NewGate(1)

	ctx, cancel := context.WithCancel(context.Background())

	cancel()

	require.NoError(t, g.WalkThrough(nil))

	_, err := g.AwaitGateCondition(ctx)

	assert.Equal(t, interop.ErrTimeout, err)
}

func BenchmarkAwaitGateCondition(b *testing.B) {
	g := NewGate(1)

	for n := 0; n < b.N; n++ {
		go func() { require.NoError(b, g.WalkThrough(nil)) }()
		_, err := g.AwaitGateCondition(context.Background())
		if err != nil {
			panic(err)
		}
		g.Reset()
	}
}
