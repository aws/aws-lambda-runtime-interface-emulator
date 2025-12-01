// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/utils"
)

type fileInfo struct {
	name   string
	mode   os.FileMode
	size   int64
	target string
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

func createFileTree(root string, fs []fileInfo) error {
	for _, info := range fs {
		filename := info.name
		dir := path.Join(root, path.Dir(filename))
		name := path.Base(filename)
		err := os.MkdirAll(dir, 0o775)
		if err != nil && !os.IsExist(err) {
			return err
		}
		switch {
		case os.ModeDir == info.mode&os.ModeDir:
			err := os.Mkdir(path.Join(dir, name), info.mode&os.ModePerm)
			if err != nil {
				return err
			}
		case os.ModeSymlink == info.mode&os.ModeSymlink:
			target := path.Join(root, info.target)
			_, err = os.Stat(target)
			if err != nil {
				return err
			}
			err := os.Symlink(target, path.Join(dir, name))
			if err != nil {
				return err
			}
		default:
			file, err := os.OpenFile(path.Join(dir, name), os.O_RDWR|os.O_CREATE, info.mode&os.ModePerm)
			if err != nil {
				return err
			}
			if err := file.Truncate(info.size); err != nil {
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
		}
	}

	return nil
}

func TestBaseEmpty(t *testing.T) {
	assert := assert.New(t)

	fs := []fileInfo{
		mkDir("/opt/extensions", 0o777),
	}

	tmpDir, err := os.MkdirTemp("", "ext-")
	require.NoError(t, err)

	require.NoError(t, createFileTree(tmpDir, fs))
	defer func() { require.NoError(t, os.RemoveAll(tmpDir)) }()

	fileUtils := utils.NewFileUtil()

	agents := ListExternalAgentPaths(fileUtils, path.Join(tmpDir, "/opt/extensions"), "/")
	assert.Equal(0, len(agents))
}

func TestBaseNotExist(t *testing.T) {
	assert := assert.New(t)

	fileUtils := utils.NewFileUtil()

	agents := ListExternalAgentPaths(fileUtils, "/path/which/does/not/exist", "/")
	assert.Equal(0, len(agents))
}

func TestChrootNotExist(t *testing.T) {
	assert := assert.New(t)

	fileUtils := utils.NewFileUtil()

	agents := ListExternalAgentPaths(fileUtils, "/bin", "/does/not/exist")
	assert.Equal(0, len(agents))
}

func TestBaseNotDir(t *testing.T) {
	assert := assert.New(t)

	fs := []fileInfo{
		mkFile("/opt/extensions", 1, 0o777),
	}
	tmpDir, err := os.MkdirTemp("", "ext-")
	require.NoError(t, err)

	require.NoError(t, createFileTree(tmpDir, fs))
	defer func() { require.NoError(t, os.RemoveAll(tmpDir)) }()

	path := path.Join(tmpDir, "/opt/extensions")

	fileUtils := utils.NewFileUtil()
	agents := ListExternalAgentPaths(fileUtils, path, "/")
	assert.Equal(0, len(agents))
}

func TestFindAgentMixed(t *testing.T) {
	assert := assert.New(t)

	listed := []fileInfo{
		mkFile("/opt/extensions/ok2", 1, 0o777),
		mkFile("/opt/extensions/ok1", 1, 0o777),
		mkFile("/opt/extensions/not_exec", 1, 0o666),
		mkFile("/opt/extensions/not_read", 1, 0o333),
		mkFile("/opt/extensions/empty_file", 0, 0o777),
		mkLink("/opt/extensions/link", "/opt/extensions/ok1"),
	}

	unlisted := []fileInfo{
		mkDir("/opt/extensions/empty_dir", 0o777),
		mkDir("/opt/extensions/nonempty_dir", 0o777),
		mkFile("/opt/extensions/nonempty_dir/notok", 1, 0o777),
	}

	fs := append([]fileInfo{}, listed...)
	fs = append(fs, unlisted...)

	tmpDir, err := os.MkdirTemp("", "ext-")
	require.NoError(t, err)

	require.NoError(t, createFileTree(tmpDir, fs))
	defer func() { require.NoError(t, os.RemoveAll(tmpDir)) }()

	path := path.Join(tmpDir, "/opt/extensions")
	fileUtils := utils.NewFileUtil()
	agentPaths := ListExternalAgentPaths(fileUtils, path, "/")
	assert.Equal(len(listed), len(agentPaths))
	last := ""
	for index := range listed {
		if len(last) > 0 {
			assert.GreaterOrEqual(agentPaths[index], last)
		}
		last = agentPaths[index]
	}
}

func TestFindAgentMixedInChroot(t *testing.T) {
	assert := assert.New(t)

	listed := []fileInfo{
		mkFile("/opt/extensions/ok2", 1, 0o777),
		mkFile("/opt/extensions/ok1", 1, 0o777),
		mkFile("/opt/extensions/not_exec", 1, 0o666),
		mkFile("/opt/extensions/not_read", 1, 0o333),
		mkFile("/opt/extensions/empty_file", 0, 0o777),
		mkLink("/opt/extensions/link", "/opt/extensions/ok1"),
	}

	unlisted := []fileInfo{
		mkDir("/opt/extensions/empty_dir", 0o777),
		mkDir("/opt/extensions/nonempty_dir", 0o777),
		mkFile("/opt/extensions/nonempty_dir/notok", 1, 0o777),
	}

	fs := append([]fileInfo{}, listed...)
	fs = append(fs, unlisted...)

	rootDir, err := os.MkdirTemp("", "rootfs")
	require.NoError(t, err)

	require.NoError(t, createFileTree(rootDir, fs))
	defer func() { require.NoError(t, os.RemoveAll(rootDir)) }()
	fileUtils := utils.NewFileUtil()
	agentPaths := ListExternalAgentPaths(fileUtils, "/opt/extensions", rootDir)
	assert.Equal(len(listed), len(agentPaths))
	last := ""
	for index := range listed {
		if len(last) > 0 {
			assert.GreaterOrEqual(agentPaths[index], last)
		}
		last = agentPaths[index]
	}
}
