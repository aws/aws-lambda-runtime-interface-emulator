// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"fmt"
	"io"
	"log"
	"strings"
)

// TODO PlatformLogger interface has this LogExtensionInitEvent() method so it's easier to assert against it in standalone tests;
// TODO However, this makes interface harder to maintain (you are supposed to add new method to PlatformLogger for each event type)
// TODO We need to remove those methods and make PlatformLogger just a log.Logger interface

// PlatformLogger is a logger that logs platform lines to customers' logs
type PlatformLogger interface {
	Printf(fmt string, args ...interface{})
	LogExtensionInitEvent(agentName, state, errorType string, subscriptions []string)
}

// FormattedPlatformLogger formats and logs platform lines to customers' logs via Telemetry API
type FormattedPlatformLogger struct {
	logger *log.Logger
}

// NewPlatformLogger is a logger for logging Platform log lines into customers' logs
func NewPlatformLogger(output, tailLogWriter io.Writer) *FormattedPlatformLogger {
	prefix, flags := "", 0
	return &FormattedPlatformLogger{
		logger: log.New(io.MultiWriter(output, tailLogWriter), prefix, flags),
	}
}

// LogExtensionInitEvent formats and logs a line containing agent info
func (l *FormattedPlatformLogger) LogExtensionInitEvent(agentName, state, errorType string, subscriptions []string) {
	format := "EXTENSION\tName: %s\tState: %s\tEvents: [%s]"
	line := fmt.Sprintf(format, agentName, state, strings.Join(subscriptions, ","))
	if len(errorType) > 0 {
		line += fmt.Sprintf("\tError Type: %s", errorType)
	}
	l.logger.Println(line)
}

func (l *FormattedPlatformLogger) Printf(fmt string, args ...interface{}) {
	fmt += "\n" // we append newline to the logline because that's how they are separated on recepient
	l.logger.Printf(fmt, args...)
}

func SupernovaInvalidTaskConfigRepr(err error) func(error) string {
	return func(unused error) string {
		return fmt.Sprintf("IMAGE\tInvalid task config: %s", err)
	}
}

func SupernovaLaunchErrorRepr(entrypoint []string, cmd []string, workingDir string) func(error) string {
	return func(err error) string {
		return fmt.Sprintf("IMAGE\tLaunch error: %s\tEntrypoint: [%s]\tCmd: [%s]\tWorkingDir: [%s]",
			err,
			strings.Join(entrypoint, ","),
			strings.Join(cmd, ","),
			workingDir)
	}
}
