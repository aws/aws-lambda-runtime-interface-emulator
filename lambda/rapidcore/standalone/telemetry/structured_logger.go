// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"github.com/sirupsen/logrus"
	"os"
)

var log = getLogger()

func getLogger() *logrus.Logger {
	formatter := logrus.JSONFormatter{}
	formatter.DisableTimestamp = true
	logger := new(logrus.Logger)
	logger.Out = os.Stdout
	logger.Formatter = &formatter
	logger.Level = logrus.InfoLevel
	return logger
}
