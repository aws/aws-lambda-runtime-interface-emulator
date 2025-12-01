// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invariant

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPanicExecutor(t *testing.T) {
	executor := NewPanicViolationExecuter()

	for i := 0; i < 2; i++ {
		err := ViolationError{Statement: fmt.Sprintf("oops %v", i)}
		assert.PanicsWithValue(t, err, func() { executor.Exec(err) })
	}
}
