// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyWithPool_BasicFunctionality(t *testing.T) {

	testCases := []struct {
		name string
		data string
	}{
		{"empty", ""},
		{"small", "hello world"},
		{"large", strings.Repeat("large test data ", 10000)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for range 10 {
				go func() {
					for range 100 {
						src := strings.NewReader(tc.data)
						dst := &bytes.Buffer{}

						written, err := CopyWithPool(dst, src)

						require.NoError(t, err)
						assert.Equal(t, int64(len(tc.data)), written)
						assert.Equal(t, tc.data, dst.String())
					}
				}()
			}
		})
	}
}
