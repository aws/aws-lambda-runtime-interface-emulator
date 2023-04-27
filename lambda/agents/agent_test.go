// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// - Test utilities

// a small struct to hold file metadata
type fileInfo struct {
	name   string
	mode   os.FileMode
	size   int64
	target string // for symlinks
}

func mkFile(name string, size int64, perm os.FileMode) fileInfo {
	return fileInfo{
		name:   name,
		mode:   perm,
		size:   size,
		target: "",
	}
}

func mkDir(name string, perm os.FileMode) fileInfo {
	return fileInfo{
		name:   name,
		mode:   perm | os.ModeDir,
		size:   0,
		target: "",
	}
}

func mkLink(name, target string) fileInfo {
	return fileInfo{
		name:   name,
		mode:   os.ModeSymlink,
		size:   0,
		target: target,
	}
}

// populate a directory with a list of files and directories
func createFileTree(root string, fs []fileInfo) error {

	for _, info := range fs {
		filename := info.name
		dir := path.Join(root, path.Dir(filename))
		name := path.Base(filename)
		err := os.MkdirAll(dir, 0775)
		if err != nil && !os.IsExist(err) {
			return err
		}
		if os.ModeDir == info.mode&os.ModeDir {
			err := os.Mkdir(path.Join(dir, name), info.mode&os.ModePerm)
			if err != nil {
				return err
			}
		} else if os.ModeSymlink == info.mode&os.ModeSymlink {
			target := path.Join(root, info.target)
			_, err = os.Stat(target)
			if err != nil {
				return err
			}
			err := os.Symlink(target, path.Join(dir, name))
			if err != nil {
				return err
			}
		} else {
			file, err := os.OpenFile(path.Join(dir, name), os.O_RDWR|os.O_CREATE, info.mode&os.ModePerm)
			if err != nil {
				return err
			}
			file.Truncate(info.size)
			file.Close()
		}
	}

	return nil
}

// - Actual tests

// If the agents folder is empty it is not an error
func TestBaseEmpty(t *testing.T) {

	assert := assert.New(t)

	fs := []fileInfo{
		mkDir("/opt/extensions", 0777),
	}

	tmpDir, err := os.MkdirTemp("", "ext-")
	require.NoError(t, err)

	createFileTree(tmpDir, fs)
	defer os.RemoveAll(tmpDir)

	agents := ListExternalAgentPaths(path.Join(tmpDir, "/opt/extensions"), "/")
	assert.Equal(0, len(agents))
}

// Test that non-existant /opt/extensions is treated as if no agents were found
func TestBaseNotExist(t *testing.T) {

	assert := assert.New(t)

	agents := ListExternalAgentPaths("/path/which/does/not/exist", "/")
	assert.Equal(0, len(agents))
}

// Test that non-existant root dir is teaded as if no agents were found
func TestChrootNotExist(t *testing.T) {

	assert := assert.New(t)

	agents := ListExternalAgentPaths("/bin", "/does/not/exist")
	assert.Equal(0, len(agents))
}

// Test that non-directory /opt/extensions is treated as if no agents were found
func TestBaseNotDir(t *testing.T) {

	assert := assert.New(t)

	fs := []fileInfo{
		mkFile("/opt/extensions", 1, 0777),
	}
	tmpDir, err := os.MkdirTemp("", "ext-")
	require.NoError(t, err)

	createFileTree(tmpDir, fs)
	defer os.RemoveAll(tmpDir)

	path := path.Join(tmpDir, "/opt/extensions")
	agents := ListExternalAgentPaths(path, "/")
	assert.Equal(0, len(agents))
}

// Test our ability to find agent bootstraps in the FS and return them sorted.
// Even if not all files are valid as executable agents,
// ListExternalAgentPaths() is expected to return all of them.
func TestFindAgentMixed(t *testing.T) {

	assert := assert.New(t)

	listed := []fileInfo{
		mkFile("/opt/extensions/ok2", 1, 0777),                // this is ok
		mkFile("/opt/extensions/ok1", 1, 0777),                // this is ok as well
		mkFile("/opt/extensions/not_exec", 1, 0666),           // this is not executable
		mkFile("/opt/extensions/not_read", 1, 0333),           // this is not readable
		mkFile("/opt/extensions/empty_file", 0, 0777),         // this is empty
		mkLink("/opt/extensions/link", "/opt/extensions/ok1"), // symlink must be ignored
	}

	unlisted := []fileInfo{
		mkDir("/opt/extensions/empty_dir", 0777),              // this is an empty directory
		mkDir("/opt/extensions/nonempty_dir", 0777),           // subdirs should not be listed
		mkFile("/opt/extensions/nonempty_dir/notok", 1, 0777), // files in subdirs should not be listed
	}

	fs := append([]fileInfo{}, listed...)
	fs = append(fs, unlisted...)

	tmpDir, err := os.MkdirTemp("", "ext-")
	require.NoError(t, err)

	createFileTree(tmpDir, fs)
	defer os.RemoveAll(tmpDir)

	path := path.Join(tmpDir, "/opt/extensions")
	agentPaths := ListExternalAgentPaths(path, "/")
	assert.Equal(len(listed), len(agentPaths))
	last := ""
	for index := range listed {
		if len(last) > 0 {
			assert.GreaterOrEqual(agentPaths[index], last)
		}
		last = agentPaths[index]
	}
}

// Test our ability to find agent bootstraps in the FS and return them sorted,
// when using a different mount namespace root for the extensiosn domain.
// Even if not all files are valid as executable agents,
// ListExternalAgentPaths() is expected to return all of them.
func TestFindAgentMixedInChroot(t *testing.T) {

	assert := assert.New(t)

	listed := []fileInfo{
		mkFile("/opt/extensions/ok2", 1, 0777),                // this is ok
		mkFile("/opt/extensions/ok1", 1, 0777),                // this is ok as well
		mkFile("/opt/extensions/not_exec", 1, 0666),           // this is not executable
		mkFile("/opt/extensions/not_read", 1, 0333),           // this is not readable
		mkFile("/opt/extensions/empty_file", 0, 0777),         // this is empty
		mkLink("/opt/extensions/link", "/opt/extensions/ok1"), // symlink must be ignored
	}

	unlisted := []fileInfo{
		mkDir("/opt/extensions/empty_dir", 0777),              // this is an empty directory
		mkDir("/opt/extensions/nonempty_dir", 0777),           // subdirs should not be listed
		mkFile("/opt/extensions/nonempty_dir/notok", 1, 0777), // files in subdirs should not be listed
	}

	fs := append([]fileInfo{}, listed...)
	fs = append(fs, unlisted...)

	rootDir, err := os.MkdirTemp("", "rootfs")
	require.NoError(t, err)

	createFileTree(rootDir, fs)
	defer os.RemoveAll(rootDir)

	agentPaths := ListExternalAgentPaths("/opt/extensions", rootDir)
	assert.Equal(len(listed), len(agentPaths))
	last := ""
	for index := range listed {
		if len(last) > 0 {
			assert.GreaterOrEqual(agentPaths[index], last)
		}
		last = agentPaths[index]
	}
}
