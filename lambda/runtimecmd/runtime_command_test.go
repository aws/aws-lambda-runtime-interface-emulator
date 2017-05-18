// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package runtimecmd

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuntimeCommandSetsEnvironmentVariables(t *testing.T) {
	envVars := []string{"foo=1", "bar=2", "baz=3"}

	currentDir, err := os.Getwd()
	assert.NoError(t, err, errors.New("Failed to get working directory to execute helper process"))

	execCmdArgs := []string{"foobar"}
	runtimeCmd := NewCustomRuntimeCmd(context.Background(), execCmdArgs, currentDir, envVars, ioutil.Discard, nil)

	assert.ElementsMatch(t, envVars, runtimeCmd.Env)
	assert.Equal(t, execCmdArgs, runtimeCmd.Args)
}

func TestRuntimeCommandSetsCurrentWorkingDir(t *testing.T) {
	envVars := []string{}

	currentDir, err := os.Getwd()
	assert.NoError(t, err, errors.New("Failed to get working directory to execute helper process"))

	execCmdArgs := []string{"foobar"}
	runtimeCmd := NewCustomRuntimeCmd(context.Background(), execCmdArgs, currentDir, envVars, ioutil.Discard, nil)

	assert.Equal(t, currentDir, runtimeCmd.Dir)
}

func TestRuntimeCommandSetsMultipleArgs(t *testing.T) {
	envVars := []string{}

	currentDir, err := os.Getwd()
	assert.NoError(t, err, errors.New("Failed to get working directory to execute helper process"))

	execCmdArgs := []string{"foobar", "--baz", "22"}
	runtimeCmd := NewCustomRuntimeCmd(context.Background(), execCmdArgs, currentDir, envVars, ioutil.Discard, nil)

	assert.Equal(t, execCmdArgs, runtimeCmd.Args)
}
