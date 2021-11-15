// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"io/ioutil"
	"os"
	"testing"

	"go.amzn.com/lambda/rapidcore/env"

	"github.com/stretchr/testify/assert"
)

func TestBootstrap(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "lcis-test-invalid-bootstrap")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tmpFile, err := ioutil.TempFile("", "lcis-test-bootstrap")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Setup cmd candidates
	nonExistent := []string{"/foo/bar/baz"}
	dir := []string{tmpDir, "--arg1", "foo"}
	file := []string{tmpFile.Name(), "--arg1 s", "foo"}
	cmdCandidates := [][]string{nonExistent, dir, file}

	// Setup working dir
	cwd, err := os.Getwd()
	assert.NoError(t, err)

	// Setup environment
	environment := env.NewEnvironment()
	environment.StoreRuntimeAPIEnvironmentVariable("host:port")
	environment.StoreEnvironmentVariablesFromInit(map[string]string{}, "", "", "", "", "", "")

	// Test
	b := NewBootstrap(cmdCandidates, cwd)
	bCwd, err := b.Cwd()
	assert.NoError(t, err)
	assert.Equal(t, cwd, bCwd)
	assert.ElementsMatch(t, environment.RuntimeExecEnv(), b.Env(environment))

	cmd, err := b.Cmd()
	assert.NoError(t, err)
	assert.Equal(t, file, cmd)
}

func TestBootstrapEmptyCandidate(t *testing.T) {
	// we expect newBootstrap to succeed and bootstrap.Cmd() to fail.
	// We want to postpone the failure to be able to propagate error description to slicer and write it to customer log
	invalidBootstrapCandidate := []string{}
	bs := NewBootstrap([][]string{invalidBootstrapCandidate}, "/")
	_, err := bs.Cmd()
	assert.Error(t, err)
}

func TestBootstrapSingleCmd(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "lcis-test-invalid-bootstrap")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tmpFile, err := ioutil.TempFile("", "lcis-test-bootstrap")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Setup single cmd candidate
	file := []string{tmpFile.Name(), "--arg1 s", "foo"}
	cmdCandidate := file

	// Setup working dir
	cwd, err := os.Getwd()
	assert.NoError(t, err)

	// Setup environment
	environment := env.NewEnvironment()
	environment.StoreRuntimeAPIEnvironmentVariable("host:port")
	environment.StoreEnvironmentVariablesFromInit(map[string]string{}, "", "", "", "", "", "")

	// Test
	b := NewBootstrapSingleCmd(cmdCandidate, cwd)
	bCwd, err := b.Cwd()
	assert.NoError(t, err)
	assert.Equal(t, cwd, bCwd)
	assert.ElementsMatch(t, environment.RuntimeExecEnv(), b.Env(environment))

	cmd, err := b.Cmd()
	assert.NoError(t, err)
	assert.Equal(t, file, cmd)
}

func TestBootstrapSingleCmdNonExistingCandidate(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "lcis-test-invalid-bootstrap")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Setup inexistent single cmd candidate
	file := []string{"/foo/bar", "--arg1 s", "foo"}
	cmdCandidate := file

	// Setup working dir
	cwd, err := os.Getwd()
	assert.NoError(t, err)

	// Setup environment
	environment := env.NewEnvironment()
	environment.StoreRuntimeAPIEnvironmentVariable("host:port")
	environment.StoreEnvironmentVariablesFromInit(map[string]string{}, "", "", "", "", "", "")

	// Test
	b := NewBootstrapSingleCmd(cmdCandidate, cwd)
	bCwd, err := b.Cwd()
	assert.NoError(t, err)
	assert.Equal(t, cwd, bCwd)
	assert.ElementsMatch(t, environment.RuntimeExecEnv(), b.Env(environment))

	// No validations run against single candidates
	cmd, err := b.Cmd()
	assert.NoError(t, err)
	assert.Equal(t, file, cmd)
}

// Test our ability to locate bootstrap files in the file system
func TestFindCustomRuntimeIfExists(t *testing.T) {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "tmp-")
	if err != nil {
		t.Fatal("Cannot create temporary file", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile2, err := ioutil.TempFile(os.TempDir(), "tmp-")
	if err != nil {
		t.Fatal("Cannot create temporary file", err)
	}
	defer os.Remove(tmpFile2.Name())

	// one bootstrap argument was given and it exists
	bootstrap := NewBootstrap([][]string{[]string{tmpFile.Name()}}, "/")
	cmd, err := bootstrap.Cmd()
	assert.NoError(t, err)
	assert.Equal(t, []string{tmpFile.Name()}, cmd)
	assert.Nil(t, err)

	// two bootstrap arguments given, both exist but first one is returned
	bootstrap = NewBootstrap([][]string{[]string{tmpFile.Name()}, []string{tmpFile2.Name()}}, "/")
	cmd, err = bootstrap.Cmd()
	assert.NoError(t, err)
	assert.Equal(t, []string{tmpFile.Name()}, cmd)
	assert.Nil(t, err)

	// two bootstrap arguments given, first one does not exist, second exists and is returned
	bootstrap = NewBootstrap([][]string{[]string{"mk"}, []string{tmpFile2.Name()}}, "/")
	cmd, err = bootstrap.Cmd()
	assert.NoError(t, err)
	assert.Equal(t, []string{tmpFile2.Name()}, cmd)
	assert.Nil(t, err)

	// two bootstrap arguments given, none exists
	bootstrap = NewBootstrap([][]string{[]string{"mk"}, []string{"mk2"}}, "/")
	cmd, err = bootstrap.Cmd()
	assert.EqualError(t, err, "Couldn't find valid bootstrap(s): [mk mk2]")
	assert.Equal(t, []string{}, cmd)
}

func TestCwdIsAbsolute(t *testing.T) {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "tmp-")
	if err != nil {
		t.Fatal("Cannot create temporary file", err)
	}
	defer os.Remove(tmpFile.Name())

	cmdCandidates := [][]string{[]string{tmpFile.Name()}}

	// no errors when currentWorkingDir is absolute
	bootstrap := NewBootstrap(cmdCandidates, "/tmp")
	cwd, err := bootstrap.Cwd()
	assert.Nil(t, err)
	assert.Equal(t, "/tmp", cwd)

	bootstrap = NewBootstrap(cmdCandidates, "tmp")
	_, err = bootstrap.Cwd()
	assert.EqualError(t, err, "the working directory 'tmp' is invalid, it needs to be an absolute path")

	bootstrap = NewBootstrap(cmdCandidates, "./")
	_, err = bootstrap.Cwd()
	assert.EqualError(t, err, "the working directory './' is invalid, it needs to be an absolute path")
}

func TestBootstrapMissingWorkingDirectory(t *testing.T) {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "cwd-test-bootstrap")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	tmpDir, err := ioutil.TempDir("", "cwd-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// cwd argument exists
	bootstrap := NewBootstrap([][]string{[]string{tmpFile.Name()}}, tmpDir)
	cwd, err := bootstrap.Cwd()
	assert.Equal(t, cwd, tmpDir)
	assert.NoError(t, err)

	// cwd argument doesn't exist
	bootstrap = NewBootstrap([][]string{[]string{tmpFile.Name()}}, "/foo")
	_, err = bootstrap.Cwd()
	assert.EqualError(t, err, "the working directory doesn't exist: /foo")
}

func TestDefaultWorkeringDirectory(t *testing.T) {
	bootstrap := NewBootstrap([][]string{[]string{}}, "")
	cwd, err := bootstrap.Cwd()
	assert.NoError(t, err)
	assert.Equal(t, "/", cwd)
}

func TestBootstrapSingleCmdDefaultWorkingDir(t *testing.T) {
	b := NewBootstrapSingleCmd([]string{}, "")
	bCwd, err := b.Cwd()
	assert.NoError(t, err)
	assert.Equal(t, "/", bCwd)
}
