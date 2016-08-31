// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"fmt"
	"time"

	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/logpb"
	. "github.com/smartystreets/goconvey/convey"
)

type testData struct {
	Data
	released bool
}

func (d *testData) Release() {
	d.released = true
}

func data(ts time.Time, b ...byte) *testData {
	return &testData{
		Data: &streamData{
			buffer:     b,
			bufferBase: b,
			ts:         ts,
		},
	}
}

func dstr(ts time.Time, s string) *testData {
	return data(ts, []byte(s)...)
}

func shouldMatchLogEntry(actual interface{}, expected ...interface{}) string {
	if actual == (*logpb.LogEntry)(nil) {
		return fmt.Sprintf("actual should not be nil")
	}
	if len(expected) != 1 || expected[0] == (*logpb.LogEntry)(nil) {
		return fmt.Sprintf("expected should not be nil")
	}

	return ShouldResemble(actual, expected[0])
}

type parserTestStream struct {
	now         time.Time
	prefixIndex int64
	streamIndex int64
	offset      time.Duration
	sequence    int64
}

func (s *parserTestStream) base() baseParser {
	return baseParser{
		counter: &counter{
			current: s.prefixIndex,
		},
		timeBase: s.now,
	}
}

func (s *parserTestStream) next(d interface{}) *logpb.LogEntry {
	le := &logpb.LogEntry{
		TimeOffset:  google.NewDuration(s.offset),
		PrefixIndex: uint64(s.prefixIndex),
		StreamIndex: uint64(s.streamIndex),
	}

	switch t := d.(type) {
	case logpb.Text:
		le.Content = &logpb.LogEntry_Text{Text: &t}

	case logpb.Binary:
		le.Content = &logpb.LogEntry_Binary{Binary: &t}

	case logpb.Datagram:
		le.Content = &logpb.LogEntry_Datagram{Datagram: &t}

	default:
		panic(fmt.Errorf("unknown content type: %T", t))
	}

	s.prefixIndex++
	s.streamIndex++
	return le
}

func (s *parserTestStream) add(d time.Duration) *parserTestStream {
	s.offset += d
	return s
}

func (s *parserTestStream) le(seq int64, d interface{}) *logpb.LogEntry {
	le := s.next(d)
	le.Sequence = uint64(seq)
	return le
}
