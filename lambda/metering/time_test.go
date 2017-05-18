// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package metering

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMonoToEpochPrecision(t *testing.T) {
	a := time.Now().UnixNano()
	b := MonoToEpoch(Monotime())

	// Conversion error is less than a millisecond.
	assert.True(t, math.Abs(float64(a-b)) < float64(time.Millisecond))
}
