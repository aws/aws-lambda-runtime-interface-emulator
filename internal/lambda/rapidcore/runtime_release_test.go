// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRuntimeRelease(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *RuntimeRelease
	}{
		{
			"simple",
			"NAME=foo\nVERSION=bar\nLOGGING=baz\n",
			&RuntimeRelease{"foo", "bar", "baz"},
		},
		{
			"no trailing new line",
			"NAME=foo\nVERSION=bar\nLOGGING=baz",
			&RuntimeRelease{"foo", "bar", "baz"},
		},
		{
			"nonexistent keys",
			"LOGGING=baz\n",
			&RuntimeRelease{"", "", "baz"},
		},
		{
			"empty value",
			"NAME=\nVERSION=\nLOGGING=\n",
			&RuntimeRelease{"", "", ""},
		},
		{
			"delimiter in value",
			"NAME=Foo=Bar\nVERSION=bar\nLOGGING=baz\n",
			&RuntimeRelease{"Foo=Bar", "bar", "baz"},
		},
		{
			"empty file",
			"",
			&RuntimeRelease{"", "", ""},
		},
		{
			"quotes",
			"NAME=\"foo\"\nVERSION='bar'\n",
			&RuntimeRelease{"foo", "bar", ""},
		},
		{
			"double quotes",
			"NAME='\"foo\"'\nVERSION=\"'bar'\"\n",
			&RuntimeRelease{"foo", "bar", ""},
		},
		{
			"empty lines", // production runtime-release files have empty line in the end of the file
			"\nNAME=foo\n\nVERSION=bar\n\nLOGGING=baz\n\n",
			&RuntimeRelease{"foo", "bar", "baz"},
		},
		{
			"comments",
			"# comment 1\nNAME=foo\n# comment 2\nVERSION=bar\n# comment 3\nLOGGING=baz\n# comment 4\n",
			&RuntimeRelease{"foo", "bar", "baz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.CreateTemp(os.TempDir(), "runtime-release")
			require.NoError(t, err)
			_, err = f.WriteString(tt.content)
			require.NoError(t, err)
			got, err := GetRuntimeRelease(f.Name())
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetRuntimeRelease_NotFound(t *testing.T) {
	_, err := GetRuntimeRelease("/sys/not-exists")
	assert.Error(t, err)
}

func TestGetRuntimeRelease_InvalidLine(t *testing.T) {
	f, err := os.CreateTemp(os.TempDir(), "runtime-release")
	require.NoError(t, err)
	_, err = f.WriteString("NAME=foo\nVERSION=bar\nLOGGING=baz\nSOMETHING")
	require.NoError(t, err)
	_, err = GetRuntimeRelease(f.Name())
	assert.Error(t, err)
}
