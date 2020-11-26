// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package extensions

import (
	"sync/atomic"
)

var enabled atomic.Value

// Enable or disable extensions
func Enable() {
	enabled.Store(true)
}

func Disable() {
	enabled.Store(false)
}

// AreEnabled returns true if extensions are enabled, false otherwise
// If it was never set defaults to false
func AreEnabled() bool {
	val := enabled.Load()
	if nil == val {
		return false
	}
	return val.(bool)
}
