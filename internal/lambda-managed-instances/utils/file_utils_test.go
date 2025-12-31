// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateStickyWorldWritableDir(t *testing.T) {
	tempDir := t.TempDir()

	lostFoundDir := filepath.Join(tempDir, "lost+found")
	require.NoError(t, os.Mkdir(lostFoundDir, 0o755))

	uid := os.Getuid()
	gid := os.Getgid()

	require.NoError(t, FixTmpDir(tempDir, uid, gid))

	stat, err := os.Stat(tempDir)
	require.NoError(t, err)

	require.Equal(t, fs.ModeDir|fs.ModeSticky|fs.ModePerm, stat.Mode())

	if sysStat, ok := stat.Sys().(*syscall.Stat_t); ok {
		require.Equal(t, uint32(uid), sysStat.Uid)
		require.Equal(t, uint32(gid), sysStat.Gid)
	}

	_, err = os.Stat(lostFoundDir)
	require.True(t, os.IsNotExist(err))
}
