// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"errors"

	"github.com/luci/luci-go/logdog/api/logpb"
)

// binaryThreshold is the amount of binary data that we will willingly yield
// if not allowed to split. This helps build larger binary stream chunks.
const defaultBinaryThreshold = 8 * 1024

// binaryParser is a parser implementation for the LogDog BINARY stream type.
type binaryParser struct {
	baseParser

	offset    int64
	threshold int
}

var _ parser = (*binaryParser)(nil)

func (p *binaryParser) nextEntry(c *constraints) (*logpb.LogEntry, error) {
	threshold := p.getThreshold()
	if c.allowSplit {
		// If we're allowed to split, return _any_ available data.
		threshold = 0
	}

	count := p.Len()
	if count <= int64(threshold) {
		return nil, nil
	}
	if count > int64(c.limit) {
		count = int64(c.limit)
	}

	// The integer conversion, since count has been bounded by our "int" limit.
	size := int(count)

	data := make([]byte, size)
	size, _ = p.View().Read(data)
	memoryCorruptionIf(int64(size) != count, errors.New("partial buffer read"))

	ts, _ := p.firstChunkTime()
	e := p.baseLogEntry(ts)
	e.Content = &logpb.LogEntry_Binary{Binary: &logpb.Binary{
		Offset: uint64(p.offset),
		Data:   data[:size],
	}}
	e.Sequence = uint64(p.offset)

	p.Consume(int64(size))
	p.offset += int64(size)
	return e, nil
}

func (p *binaryParser) getThreshold() int {
	result := p.threshold
	if result == 0 {
		result = defaultBinaryThreshold
	}
	return result
}
