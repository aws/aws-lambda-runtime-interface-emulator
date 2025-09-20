// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"testing"
)

func TestLogPrint(t *testing.T) {
	buf := new(bytes.Buffer)
	SetOutput(buf)
	log.Print("hello log")
	assert.Contains(t, buf.String(), "hello log")
}

func TestLogrusPrint(t *testing.T) {
	buf := new(bytes.Buffer)
	SetOutput(buf)
	logrus.Print("hello logrus")
	assert.Contains(t, buf.String(), "hello logrus")
}

func TestInternalFormatter(t *testing.T) {
	pattern := `^([0-9]{2}\s[A-Za-z]{3}\s[0-9]{4}\s[0-9]{2}:[0-9]{2}:[0-9]{2}(?:,[0-9]{3})?)\s(?:\s\{sandbox:([0-9]+)\}\s)?\[([A-Za-z]+)\]\s(\(([^\)]+)\)(?:\s\[Logging Metrics\]\sSBLOG:([a-zA-Z:]+) ([0-9]+))?\s?.*)`

	buf := new(bytes.Buffer)
	SetOutput(buf)
	logrus.SetFormatter(&InternalFormatter{})

	logrus.Print("hello logrus")
	assert.Regexp(t, pattern, buf.String())

	buf.Reset()
	err := fmt.Errorf("error message")
	logrus.WithError(err).Warning("hello logrus")
	assert.Regexp(t, pattern, buf.String())

	buf.Reset()
	logrus.WithFields(logrus.Fields{
		"field1": "val1",
		"field2": "val2",
		"field3": "val3",
	}).Info("hello logrus")
	assert.Regexp(t, pattern, buf.String())

	// no caller logged
	buf.Reset()
	logrus.WithFields(logrus.Fields{
		"field1": "val1",
		"field2": "val2",
		"field3": "val3",
	}).Info("hello logrus")
	assert.Regexp(t, pattern, buf.String())

	// invalid format without InternalFormatter
	buf.Reset()
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.Print("hello logrus")
	assert.NotRegexp(t, pattern, buf.String())
}

func BenchmarkLogPrint(b *testing.B) {
	SetOutput(io.Discard)
	for n := 0; n < b.N; n++ {
		log.Print(1, "two", true)
	}
}

func BenchmarkLogrusPrint(b *testing.B) {
	SetOutput(io.Discard)
	for n := 0; n < b.N; n++ {
		logrus.Print(1, "two", true)
	}
}

func BenchmarkLogrusPrintInternalFormatter(b *testing.B) {
	var l = logrus.New()
	l.SetFormatter(&InternalFormatter{})
	l.SetOutput(io.Discard)
	for n := 0; n < b.N; n++ {
		l.Print(1, "two", true)
	}
}

func BenchmarkLogPrintf(b *testing.B) {
	SetOutput(io.Discard)
	for n := 0; n < b.N; n++ {
		log.Printf("field:%v,field:%v,field:%v", 1, "two", true)
	}
}

func BenchmarkLogrusPrintf(b *testing.B) {
	SetOutput(io.Discard)
	for n := 0; n < b.N; n++ {
		logrus.Printf("field:%v,field:%v,field:%v", 1, "two", true)
	}
}

func BenchmarkLogrusPrintfInternalFormatter(b *testing.B) {
	var l = logrus.New()
	l.SetFormatter(&InternalFormatter{})
	l.SetOutput(io.Discard)
	for n := 0; n < b.N; n++ {
		l.Printf("field:%v,field:%v,field:%v", 1, "two", true)
	}
}

func BenchmarkLogrusDebugLogLevelDisabled(b *testing.B) {
	SetOutput(io.Discard)
	logrus.SetLevel(logrus.InfoLevel)
	for n := 0; n < b.N; n++ {
		logrus.Debug(1, "two", true)
	}
}

func BenchmarkLogrusDebugLogLevelDisabledInternalFormatter(b *testing.B) {
	var l = logrus.New()
	l.SetOutput(io.Discard)
	l.SetLevel(logrus.InfoLevel)
	for n := 0; n < b.N; n++ {
		l.Debug(1, "two", true)
	}
}

func BenchmarkLogrusDebugLogLevelEnabled(b *testing.B) {
	SetOutput(io.Discard)
	logrus.SetLevel(logrus.DebugLevel)
	for n := 0; n < b.N; n++ {
		logrus.Debug(1, "two", true)
	}
}

func BenchmarkLogrusDebugLogLevelEnabledInternalFormatter(b *testing.B) {
	var l = logrus.New()
	l.SetFormatter(&InternalFormatter{})
	l.SetOutput(io.Discard)
	l.SetLevel(logrus.DebugLevel)
	for n := 0; n < b.N; n++ {
		l.Debug(1, "two", true)
	}
}

func BenchmarkLogrusDebugWithFieldLogLevelDisabled(b *testing.B) {
	SetOutput(io.Discard)
	logrus.SetLevel(logrus.InfoLevel)
	for n := 0; n < b.N; n++ {
		logrus.WithField("field", "value").Debug(1, "two", true)
	}
}

func BenchmarkLogrusDebugWithFieldLogLevelDisabledInternalFormatter(b *testing.B) {
	var l = logrus.New()
	l.SetFormatter(&InternalFormatter{})
	l.SetOutput(io.Discard)
	l.SetLevel(logrus.InfoLevel)
	for n := 0; n < b.N; n++ {
		l.WithField("field", "value").Debug(1, "two", true)
	}
}
