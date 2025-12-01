// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockReaderWithSleep struct {
	reader    io.Reader
	sleepTime time.Duration
}

func (m *mockReaderWithSleep) Read(p []byte) (n int, err error) {
	time.Sleep(m.sleepTime)
	return m.reader.Read(p)
}

type mockWriterWithSleep struct {
	writer    io.Writer
	sleepTime time.Duration
}

func (m *mockWriterWithSleep) Write(p []byte) (n int, err error) {
	time.Sleep(m.sleepTime)
	return m.writer.Write(p)
}

func TestTimedReader(t *testing.T) {
	t.Parallel()

	testData := "test data for reading"
	mockReader := &mockReaderWithSleep{
		reader:    strings.NewReader(testData),
		sleepTime: 1 * time.Nanosecond,
	}

	timedReader := &TimedReader{
		Reader: mockReader,
		Ctx:    context.Background(),
	}

	testStart := time.Now()

	buffer := make([]byte, len(testData))
	n, err := timedReader.Read(buffer)

	testDuration := time.Since(testStart)

	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)
	assert.Equal(t, testData, string(buffer[:n]))

	assert.GreaterOrEqual(t, timedReader.TotalDuration, 1*time.Nanosecond,
		"TotalTime should be greater than 1ns (the sleep duration)")
	assert.LessOrEqual(t, timedReader.TotalDuration, testDuration,
		"TotalTime should be less than total measured test duration")
}

func TestTimedWriter(t *testing.T) {
	t.Parallel()

	mockWriter := &mockWriterWithSleep{
		writer:    io.Discard,
		sleepTime: 1 * time.Nanosecond,
	}

	ctx := context.Background()
	timedWriter := &TimedWriter{
		Writer: mockWriter,
		Ctx:    ctx,
	}

	testStart := time.Now()

	testData := []byte("test data for writing")
	n, err := timedWriter.Write(testData)

	testDuration := time.Since(testStart)

	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)

	assert.GreaterOrEqual(t, timedWriter.TotalDuration, 1*time.Nanosecond,
		"TotalTime should be greater than 1ns (the sleep duration)")
	assert.LessOrEqual(t, timedWriter.TotalDuration, testDuration,
		"TotalTime should be less than total measured test duration")
}
