// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package mockthread

// MockManagedThread implements core.Suspendable interface but
// does not suspend running thread on condition.
type MockManagedThread struct{}

// SuspendUnsafe does not suspend running thread.
func (s *MockManagedThread) SuspendUnsafe() {}

// Release resumes suspended thread.
func (s *MockManagedThread) Release() {}

// Lock: no-op
func (s *MockManagedThread) Lock() {}

// Unlock: no-op
func (s *MockManagedThread) Unlock() {}
