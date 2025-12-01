// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package mockthread

type MockManagedThread struct{}

func (s *MockManagedThread) SuspendUnsafe() {}

func (s *MockManagedThread) Release() {}

func (s *MockManagedThread) Lock() {}

func (s *MockManagedThread) Unlock() {}
