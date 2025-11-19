// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package extensions

import (
	"os"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

const (
	disableExtensionsFile = "/opt/disable-extensions-jwigqn8j"
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

func DisableViaMagicLayer() {
	_, err := os.Stat(disableExtensionsFile)
	if err == nil {
		log.Infof("Extensions disabled by attached layer (%s)", disableExtensionsFile)
		Disable()
	}
}
