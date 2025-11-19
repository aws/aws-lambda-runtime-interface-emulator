// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package bandwidthlimiter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewChunkIterator(t *testing.T) {
	buf := []byte("abcdefghijk")

	type testCase struct {
		buf            []byte
		chunkSize      int
		expectedResult [][]byte
	}
	testCases := []testCase{
		{buf: nil, chunkSize: 0, expectedResult: [][]byte{}},
		{buf: nil, chunkSize: 1, expectedResult: [][]byte{}},
		{buf: buf, chunkSize: 0, expectedResult: [][]byte{}},
		{buf: buf, chunkSize: 1, expectedResult: [][]byte{
			[]byte("a"), []byte("b"), []byte("c"), []byte("d"), []byte("e"), []byte("f"), []byte("g"), []byte("h"),
			[]byte("i"), []byte("j"), []byte("k"),
		}},
		{buf: buf, chunkSize: 4, expectedResult: [][]byte{[]byte("abcd"), []byte("efgh"), []byte("ijk")}},
		{buf: buf, chunkSize: 5, expectedResult: [][]byte{[]byte("abcde"), []byte("fghij"), []byte("k")}},
		{buf: buf, chunkSize: 11, expectedResult: [][]byte{[]byte("abcdefghijk")}},
		{buf: buf, chunkSize: 12, expectedResult: [][]byte{[]byte("abcdefghijk")}},
	}

	for _, test := range testCases {
		iterator := NewChunkIterator(test.buf, test.chunkSize)
		if test.buf == nil {
			assert.Nil(t, iterator)
		} else {
			for _, expectedChunk := range test.expectedResult {
				assert.Equal(t, expectedChunk, iterator.Next())
			}
			assert.Nil(t, iterator.Next())
		}
	}
}
