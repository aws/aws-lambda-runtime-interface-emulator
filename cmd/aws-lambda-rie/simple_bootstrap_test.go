// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"reflect"
	"testing"

	"go.amzn.com/lambda/rapidcore/env"

	"github.com/stretchr/testify/assert"
)

func TestSimpleBootstrap(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "oci-test-bootstrap")
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
	b := NewSimpleBootstrap(cmdCandidate, cwd)
	bCwd, err := b.Cwd()
	assert.NoError(t, err)
	assert.Equal(t, cwd, bCwd)
	assert.True(t, reflect.DeepEqual(environment.RuntimeExecEnv(), b.Env(environment)))

	cmd, err := b.Cmd()
	assert.NoError(t, err)
	assert.Equal(t, file, cmd)
}

func TestSimpleBootstrapCmdNonExistingCandidate(t *testing.T) {
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
	b := NewSimpleBootstrap(cmdCandidate, cwd)
	bCwd, err := b.Cwd()
	assert.NoError(t, err)
	assert.Equal(t, cwd, bCwd)
	assert.True(t, reflect.DeepEqual(environment.RuntimeExecEnv(), b.Env(environment)))

	// No validations run against single candidates
	cmd, err := b.Cmd()
	assert.NoError(t, err)
	assert.Equal(t, file, cmd)
}

func TestSimpleBootstrapCmdDefaultWorkingDir(t *testing.T) {
	b := NewSimpleBootstrap([]string{}, "")
	bCwd, err := b.Cwd()
	assert.NoError(t, err)
	assert.Equal(t, "/", bCwd)
}
