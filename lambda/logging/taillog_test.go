// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDisableDebugLog(t *testing.T) {
	buf := new(bytes.Buffer)
	tailLogWriter := NewTailLogWriter(buf)
	tailLogWriter.Disable()

	tailLogWriter.Write([]byte("hello_world"))
	assert.Len(t, buf.String(), 0)
}

func TestEnableDebugLog(t *testing.T) {
	buf := new(bytes.Buffer)
	tailLogWriter := NewTailLogWriter(buf)
	tailLogWriter.Enable()

	tailLogWriter.Write([]byte("hello_world"))
	assert.Equal(t, "hello_world", buf.String())
}
