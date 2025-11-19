// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package bandwidthlimiter

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func NewChunkIterator(buf []byte, chunkSize int) *ChunkIterator {
	if buf == nil {
		return nil
	}
	return &ChunkIterator{
		buf:       buf,
		chunkSize: chunkSize,
		offset:    0,
	}
}

type ChunkIterator struct {
	buf       []byte
	chunkSize int
	offset    int
}

func (i *ChunkIterator) Next() []byte {
	begin := i.offset
	end := min(i.offset+i.chunkSize, len(i.buf))
	i.offset = end

	if begin == end {
		return nil
	}
	return i.buf[begin:end]
}
