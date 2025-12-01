// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvironmentVariableSplitting(t *testing.T) {
	envVar := "FOO=BAR"
	k, v, err := SplitEnvironmentVariable(envVar)
	assert.NoError(t, err)
	assert.Equal(t, k, "FOO")
	assert.Equal(t, v, "BAR")

	envVar = "FOO=BAR=BAZ"
	k, v, err = SplitEnvironmentVariable(envVar)
	assert.NoError(t, err)
	assert.Equal(t, k, "FOO")
	assert.Equal(t, v, "BAR=BAZ")

	envVar = "FOO="
	k, v, err = SplitEnvironmentVariable(envVar)
	assert.NoError(t, err)
	assert.Equal(t, k, "FOO")
	assert.Equal(t, v, "")

	envVar = "FOO"
	k, v, err = SplitEnvironmentVariable(envVar)
	assert.Error(t, err)
	assert.Equal(t, k, "")
	assert.Equal(t, v, "")
}
