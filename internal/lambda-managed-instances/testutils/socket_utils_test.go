// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateTempSocketPath(t *testing.T) {

	socketPath, err := CreateTempSocketPath(t)
	require.NoError(t, err)

	if !strings.Contains(socketPath, ".sock") {
		t.Errorf("Expected socket path to contain '.sock', got: %s", socketPath)
	}

	parentDir := filepath.Dir(socketPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		t.Errorf("Expected parent directory %s to exist", parentDir)
	}
}
