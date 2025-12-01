// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invariant

import (
	"fmt"
	"sync"
)

func Check(cond bool, statement string) {
	if !cond {
		Violate(statement)
	}
}

func Checkf(cond bool, format string, args ...any) {
	if !cond {
		Violatef(format, args...)
	}
}

func Violate(statement string) {
	std.mtx.Lock()
	defer std.mtx.Unlock()

	std.executor.Exec(ViolationError{Statement: statement})
}

func Violatef(format string, args ...any) {
	Violate(fmt.Sprintf(format, args...))
}

func SetViolationExecutor(exector ViolationExecutor) {
	std.mtx.Lock()
	defer std.mtx.Unlock()

	std.executor = exector
}

var std = struct {
	executor ViolationExecutor
	mtx      sync.Mutex
}{
	executor: NewPanicViolationExecuter(),
}
