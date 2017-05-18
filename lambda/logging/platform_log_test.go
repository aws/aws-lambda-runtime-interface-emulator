// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlatformLogExtensionLine(t *testing.T) {
	var buf bytes.Buffer
	var tailLogBuf bytes.Buffer
	logger := NewPlatformLogger(&buf, &tailLogBuf)

	logger.LogExtensionInitEvent("agentName", "Registered", "", []string{"INVOKE", "SHUTDOWN"})
	require.Equal(t, "EXTENSION\tName: agentName\tState: Registered\tEvents: [INVOKE,SHUTDOWN]\n", buf.String())
	require.Equal(t, "EXTENSION\tName: agentName\tState: Registered\tEvents: [INVOKE,SHUTDOWN]\n", tailLogBuf.String())
}

func TestPlatformLogExtensionLineWithError(t *testing.T) {
	var buf bytes.Buffer
	var tailLogBuf bytes.Buffer
	logger := NewPlatformLogger(&buf, &tailLogBuf)

	errorType := "Extension.FooBar"
	logger.LogExtensionInitEvent("agentName", "Registered", errorType, []string{"INVOKE", "SHUTDOWN"})
	require.Equal(t, "EXTENSION\tName: agentName\tState: Registered\tEvents: [INVOKE,SHUTDOWN]\tError Type: "+errorType+"\n", buf.String())
	require.Equal(t, "EXTENSION\tName: agentName\tState: Registered\tEvents: [INVOKE,SHUTDOWN]\tError Type: "+errorType+"\n", tailLogBuf.String())
}

func TestPlatformLogPrintf(t *testing.T) {
	var buf bytes.Buffer
	var tailLogBuf bytes.Buffer
	logger := NewPlatformLogger(&buf, &tailLogBuf)

	logger.Printf("bebe %s %d", "as", 12)
	require.Equal(t, "bebe as 12\n", buf.String())
	require.Equal(t, "bebe as 12\n", tailLogBuf.String())
}
