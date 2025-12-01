// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invariant

type PanicViolationExecutor struct{}

var _ ViolationExecutor = (*PanicViolationExecutor)(nil)

func NewPanicViolationExecuter() *PanicViolationExecutor {
	return &PanicViolationExecutor{}
}

func (executor *PanicViolationExecutor) Exec(err ViolationError) {
	panic(err)
}
