// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
)

var errInvalidEventType = errors.New("ErrorInvalidEventType")
var errEventNotSupportedForInternalAgent = errors.New("ShutdownEventNotSupportedForInternalExtension")

type disallowEverything struct {
}

// Register
func (s *disallowEverything) Register(events []Event) error { return ErrNotAllowed }

// Ready
func (s *disallowEverything) Ready() error { return ErrNotAllowed }

// InitError
func (s *disallowEverything) InitError(errorType string) error { return ErrNotAllowed }

// ExitError
func (s *disallowEverything) ExitError(errorType string) error { return ErrNotAllowed }

// ShutdownFailed
func (s *disallowEverything) ShutdownFailed() error { return ErrNotAllowed }

// Exited
func (s *disallowEverything) Exited() error { return ErrNotAllowed }

// LaunchError
func (s *disallowEverything) LaunchError(error) error { return ErrNotAllowed }
