// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
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

// populate a temporary directory with a list of files and directories
// returns the name of the temporary root directory
func createFileTree(fs []fileInfo) (string, error) {

	root, err := ioutil.TempDir(os.TempDir(), "tmp-")
	if err != nil {
		return "", err
	}

	for _, info := range fs {
		filename := info.name
		dir := path.Join(root, path.Dir(filename))
		name := path.Base(filename)
		err := os.MkdirAll(dir, 0775)
		if err != nil && !os.IsExist(err) {
			return "", err
		}
		if os.ModeDir == info.mode&os.ModeDir {
			err := os.Mkdir(path.Join(dir, name), info.mode&os.ModePerm)
			if err != nil {
				return "", err
			}
		} else if os.ModeSymlink == info.mode&os.ModeSymlink {
			target := path.Join(root, info.target)
			_, err = os.Stat(target)
			if err != nil {
				return "", err
			}
			err := os.Symlink(target, path.Join(dir, name))
			if err != nil {
				return "", err
			}
		} else {
			file, err := os.OpenFile(path.Join(dir, name), os.O_RDWR|os.O_CREATE, info.mode&os.ModePerm)
			if err != nil {
				return "", err
			}
			file.Truncate(info.size)
			file.Close()
		}
	}

	return root, nil
}

// executes a given closure inside a temporary directory populated with the given FS tree
func within(fs []fileInfo, closure func()) error {

	var root string
	var cwd string
	var err error

	if root, err = createFileTree(fs); err != nil {
		return err
	}

	defer os.RemoveAll(root)

	if cwd, err = os.Getwd(); err != nil {
		return err
	}

	if err = os.Chdir(root); err != nil {
		return err
	}

	defer os.Chdir(cwd)

	closure()
	return nil
}

// - Actual tests

// If the agents folder is empty it is not an error
func TestRootEmpty(t *testing.T) {

	assert := assert.New(t)

	fs := []fileInfo{
		mkDir("/opt/extensions", 0777),
	}

	assert.NoError(within(fs, func() {
		agents := ListExternalAgentPaths("opt/extensions")
		assert.Equal(0, len(agents))
	}))
}

// Test that non-existant /opt/extensions is treated as if no agents were found
func TestRootNotExist(t *testing.T) {

	assert := assert.New(t)

	agents := ListExternalAgentPaths("/path/which/does/not/exist")
	assert.Equal(0, len(agents))
}

// Test that non-directory /opt/extensions is treated as if no agents were found
func TestRootNotDir(t *testing.T) {

	assert := assert.New(t)

	fs := []fileInfo{
		mkFile("/opt/extensions", 1, 0777),
	}

	assert.NoError(within(fs, func() {
		agents := ListExternalAgentPaths("opt/extensions")
		assert.Equal(0, len(agents))
	}))
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

	assert.NoError(within(fs, func() {
		agentPaths := ListExternalAgentPaths("opt/extensions")
		assert.Equal(len(listed), len(agentPaths))
		last := ""
		for index := range listed {
			if len(last) > 0 {
				assert.GreaterOrEqual(agentPaths[index], last)
			}
			last = agentPaths[index]
		}
	}))
}

// Test our ability to start agents
func TestAgentStart(t *testing.T) {
	assert := assert.New(t)
	agent := NewExternalAgentProcess("../testdata/agents/bash_true.sh", []string{}, &mockWriter{})
	assert.Nil(agent.Start())
	assert.Nil(agent.Wait())
}

// Test that execution of invalid agents is correctly reported
func TestInvalidAgentStart(t *testing.T) {
	assert := assert.New(t)
	agent := NewExternalAgentProcess("/bin/none", []string{}, &mockWriter{})
	assert.True(os.IsNotExist(agent.Start()))
}

// Test that execution of invalid agents is correctly reported
func TestAgentTelemetry(t *testing.T) {
	assert := assert.New(t)
	buffer := &mockWriter{}

	agent := NewExternalAgentProcess("../testdata/agents/bash_echo.sh", []string{}, buffer)

	assert.NoError(agent.Start())
	assert.NoError(agent.Wait())

	message := "hello world\n|barbaz\n|hello world\n|barbaz2"
	assert.Equal(message, string(bytes.Join(buffer.bytesReceived, []byte("|"))))
}

type mockWriter struct {
	bytesReceived [][]byte
}

func (m *mockWriter) Write(bytes []byte) (int, error) {
	m.bytesReceived = append(m.bytesReceived, bytes)
	return len(bytes), nil
}
