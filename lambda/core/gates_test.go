// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	"testing"
)

func TestWalkThrough(t *testing.T) {
	g := NewGate(1)
	assert.NoError(t, g.WalkThrough())
}

func TestWalkThroughTwice(t *testing.T) {
	g := NewGate(1)
	assert.NoError(t, g.WalkThrough())
	assert.Equal(t, ErrGateIntegrity, g.WalkThrough())
}

func TestSetCount(t *testing.T) {
	g := NewGate(2)
	assert.NoError(t, g.WalkThrough())
	assert.NoError(t, g.WalkThrough()) // arrived is now 2
	assert.Equal(t, ErrGateIntegrity, g.SetCount(1))
	assert.NoError(t, g.SetCount(2))                   // set to 2
	assert.Equal(t, ErrGateIntegrity, g.WalkThrough()) // can't go to 3
	assert.NoError(t, g.SetCount(3))                   // set to 3
	assert.NoError(t, g.WalkThrough())
}

func TestReset(t *testing.T) {
	g := NewGate(1)
	assert.NoError(t, g.WalkThrough())
	g.Reset()
	assert.NoError(t, g.WalkThrough())
}

func TestCancel(t *testing.T) {
	g := NewGate(1)

	var errg errgroup.Group
	errg.Go(g.AwaitGateCondition)
	g.CancelWithError(nil)

	assert.Equal(t, ErrGateCanceled, errg.Wait())
}

func TestCancelWithError(t *testing.T) {
	g := NewGate(1)

	var errg errgroup.Group
	errg.Go(g.AwaitGateCondition)

	err := errors.New("MyErr")
	g.CancelWithError(err)

	assert.Equal(t, err, errg.Wait())
}

func TestUseAfterCancel(t *testing.T) {
	g := NewGate(1)
	err := errors.New("MyErr")
	g.CancelWithError(err)
	assert.Equal(t, err, g.AwaitGateCondition())
	g.Reset()
	assert.Equal(t, err, g.AwaitGateCondition())
}

func BenchmarkAwaitGateCondition(b *testing.B) {
	g := NewGate(1)

	for n := 0; n < b.N; n++ {
		go func() { g.WalkThrough() }()
		if err := g.AwaitGateCondition(); err != nil {
			panic(err)
		}
		g.Reset()
	}
}

// go test -run=XXX -bench=. -benchtime 10000000x -cpu 1 -blockprofile /tmp/pprof/block3.out src/go.amzn.com/lambda/core/*
// goos: linux
// goarch: amd64
// BenchmarkAwaitGateCondition 	10000000	      1834 ns/op
// PASS
// ok  	command-line-arguments	18.449s
