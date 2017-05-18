// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package metering

import (
	_ "runtime" //for nanotime() and walltime()
	_ "unsafe"  //for go:linkname
)

//go:linkname Monotime runtime.nanotime
func Monotime() int64

//go:linkname walltime runtime.walltime
func walltime() (sec int64, nsec int32)

// MonoToEpoch converts monotonic time nanos to epoch time nanos.
func MonoToEpoch(t int64) int64 {
	monoNsec := Monotime()

	wallSec, wallNsec32 := walltime()
	wallNsec := wallSec*1e9 + int64(wallNsec32)

	clockOffset := wallNsec - monoNsec
	return t + clockOffset
}
