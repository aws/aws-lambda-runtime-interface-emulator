// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"github.com/sirupsen/logrus"
	"io"
	"log"
)

// SetOutput configures logging output for standard loggers.
func SetOutput(w io.Writer) {
	log.SetOutput(w)
	logrus.SetOutput(w)
}
