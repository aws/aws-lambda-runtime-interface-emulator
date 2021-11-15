// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"log"
	"strings"
)

// SetOutput configures logging output for standard loggers.
func SetOutput(w io.Writer) {
	log.SetOutput(w)
	logrus.SetOutput(w)
}

type InternalFormatter struct{}

// format RAPID's internal log like the rest of the sandbox log
func (f *InternalFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}

	// time with comma separator for fraction of second
	time := entry.Time.Format("02 Jan 2006 15:04:05.000")
	time = strings.Replace(time, ".", ",", 1)
	fmt.Fprint(b, time)

	// level
	level := strings.ToUpper(entry.Level.String())
	fmt.Fprintf(b, " [%s]", level)

	// label
	fmt.Fprint(b, " (rapid)")

	// message
	fmt.Fprintf(b, " %s", entry.Message)

	// from WithField and WithError
	for field, value := range entry.Data {
		fmt.Fprintf(b, " %s=%s", field, value)
	}

	fmt.Fprintf(b, "\n")
	return b.Bytes(), nil
}
