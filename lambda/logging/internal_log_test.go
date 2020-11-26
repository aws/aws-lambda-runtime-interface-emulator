// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"bytes"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
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

func BenchmarkLogPrint(b *testing.B) {
	SetOutput(ioutil.Discard)
	for n := 0; n < b.N; n++ {
		log.Print(1, "two", true)
	}
}

func BenchmarkLogrusPrint(b *testing.B) {
	SetOutput(ioutil.Discard)
	for n := 0; n < b.N; n++ {
		logrus.Print(1, "two", true)
	}
}

func BenchmarkLogPrintf(b *testing.B) {
	SetOutput(ioutil.Discard)
	for n := 0; n < b.N; n++ {
		log.Printf("field:%v,field:%v,field:%v", 1, "two", true)
	}
}

func BenchmarkLogrusPrintf(b *testing.B) {
	SetOutput(ioutil.Discard)
	for n := 0; n < b.N; n++ {
		logrus.Printf("field:%v,field:%v,field:%v", 1, "two", true)
	}
}

func BenchmarkLogrusDebugLogLevelDisabled(b *testing.B) {
	SetOutput(ioutil.Discard)
	logrus.SetLevel(logrus.InfoLevel)
	for n := 0; n < b.N; n++ {
		logrus.Debug(1, "two", true)
	}
}

func BenchmarkLogrusDebugLogLevelEnabled(b *testing.B) {
	SetOutput(ioutil.Discard)
	logrus.SetLevel(logrus.DebugLevel)
	for n := 0; n < b.N; n++ {
		logrus.Debug(1, "two", true)
	}
}

func BenchmarkLogrusDebugWithFieldLogLevelDisabled(b *testing.B) {
	SetOutput(ioutil.Discard)
	logrus.SetLevel(logrus.InfoLevel)
	for n := 0; n < b.N; n++ {
		logrus.WithField("field", "value").Debug(1, "two", true)
	}
}
