// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package invariant

type ViolationError struct {
	Statement string
}

func (err ViolationError) Error() string {
	return "Invariant violation: " + err.Statement
}

type ViolationExecutor interface {
	Exec(ViolationError)
}
