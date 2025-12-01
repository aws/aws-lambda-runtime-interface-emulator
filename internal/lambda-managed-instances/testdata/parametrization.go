// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package testdata

var SuppressInitTests = []struct {
	TestName     string
	SuppressInit bool
}{
	{"Unsuppressed", false},
	{"Suppressed", true},
}
