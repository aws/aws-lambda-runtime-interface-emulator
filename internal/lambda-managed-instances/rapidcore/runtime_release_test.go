// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rapidcore

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRuntimeRelease(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *RuntimeRelease
		wantErr bool
	}{
		{
			name:    "simple",
			content: "NAME=foo\nVERSION=bar\nLOGGING=baz\n",
			want:    &RuntimeRelease{"foo", "bar", "baz"},
		},
		{
			name:    "no trailing new line",
			content: "NAME=foo\nVERSION=bar\nLOGGING=baz",
			want:    &RuntimeRelease{"foo", "bar", "baz"},
		},
		{
			name:    "nonexistent keys",
			content: "LOGGING=baz\n",
			want:    &RuntimeRelease{"", "", "baz"},
		},
		{
			name:    "empty value",
			content: "NAME=\nVERSION=\nLOGGING=\n",
			want:    &RuntimeRelease{"", "", ""},
		},
		{
			name:    "delimiter in value",
			content: "NAME=Foo=Bar\nVERSION=bar\nLOGGING=baz\n",
			want:    &RuntimeRelease{"Foo=Bar", "bar", "baz"},
		},
		{
			name: "empty file",
			want: &RuntimeRelease{"", "", ""},
		},
		{
			name:    "quotes",
			content: "NAME=\"foo\"\nVERSION='bar'\n",
			want:    &RuntimeRelease{"foo", "bar", ""},
		},
		{
			name:    "double quotes",
			content: "NAME='\"foo\"'\nVERSION=\"'bar'\"\n",
			want:    &RuntimeRelease{"foo", "bar", ""},
		},
		{
			name:    "empty lines",
			content: "\nNAME=foo\n\nVERSION=bar\n\nLOGGING=baz\n\n",
			want:    &RuntimeRelease{"foo", "bar", "baz"},
		},
		{
			name:    "comments",
			content: "# comment 1\nNAME=foo\n# comment 2\nVERSION=bar\n# comment 3\nLOGGING=baz\n# comment 4\n",
			want:    &RuntimeRelease{"foo", "bar", "baz"},
		},
		{
			name:    "file exceeds size limit",
			content: "NAME=foo\nVERSION=bar\nLOGGING=" + strings.Repeat("a", runtimeReleaseFileSizeLimitBytes),
			wantErr: true,
		},
		{
			name:    "invalid format",
			content: "NAME=foo\nVERSION=bar\nLOGGING=baz\nLAST_LINE_IS_NOT_KV_PAIR",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.CreateTemp("", "runtime-release")
			require.NoError(t, err)
			defer func() {
				require.NoError(t, os.Remove(f.Name()))
			}()

			_, err = f.WriteString(tt.content)
			require.NoError(t, err)

			got, err := GetRuntimeRelease(f.Name())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestGetRuntimeRelease_NotFound(t *testing.T) {
	_, err := GetRuntimeRelease("/sys/not-exists")
	assert.Error(t, err)
}

func TestRuntimeRelease_GetUAProduct(t *testing.T) {
	tests := []struct {
		name    string
		rr      *RuntimeRelease
		want    string
		wantErr bool
	}{
		{
			name: "no name",
			rr: &RuntimeRelease{
				Version: "2.7.7-6419c85c",
			},
			wantErr: true,
		},
		{
			name: "no version",
			rr: &RuntimeRelease{
				Name: "Ruby",
			},
			want: "Ruby",
		},
		{
			name: "name and version",
			rr: &RuntimeRelease{
				Name:    "Ruby",
				Version: "2.7.7-6419c85c",
			},
			want: "Ruby/2.7.7-6419c85c",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.rr.GetUAProduct()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
