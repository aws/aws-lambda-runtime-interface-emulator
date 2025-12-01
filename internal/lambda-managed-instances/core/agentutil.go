// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

var errInvalidEventType = errors.New("ErrorInvalidEventType")

type disallowEverything struct{}

func (s *disallowEverything) Register(events []Event) error { return ErrNotAllowed }

func (s *disallowEverything) Ready() error { return ErrNotAllowed }

func (s *disallowEverything) InitError(errorType model.ErrorType) error { return ErrNotAllowed }

func (s *disallowEverything) ExitError(errorType model.ErrorType) error { return ErrNotAllowed }

func (s *disallowEverything) ShutdownFailed() error { return ErrNotAllowed }

func (s *disallowEverything) Exited() error { return ErrNotAllowed }

func (s *disallowEverything) LaunchError(model.ErrorType) error { return ErrNotAllowed }
