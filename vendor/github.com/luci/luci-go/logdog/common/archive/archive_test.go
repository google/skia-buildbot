// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package archive

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/data/recordio"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/iotools"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/logpb"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func gen(i int) *logpb.LogEntry {
	return &logpb.LogEntry{
		TimeOffset:  google.NewDuration(time.Duration(i) * time.Second),
		PrefixIndex: uint64(i) * 2,
		StreamIndex: uint64(i),
		Sequence:    uint64(i),
		Content: &logpb.LogEntry_Text{
			Text: &logpb.Text{
				Lines: []*logpb.Text_Line{
					{Value: strconv.Itoa(i), Delimiter: "\n"},
				},
			},
		},
	}
}

type testSource struct {
	logs []*logpb.LogEntry
	err  error
}

func (s *testSource) addLogEntry(le *logpb.LogEntry) {
	s.logs = append(s.logs, le)
}

func (s *testSource) add(indices ...int) {
	for _, i := range indices {
		s.addLogEntry(gen(i))
	}
}

func (s *testSource) addEntries(entries ...*logpb.LogEntry) {
	s.logs = append(s.logs, entries...)
}

func (s *testSource) NextLogEntry() (le *logpb.LogEntry, err error) {
	if err = s.err; err != nil {
		return
	}
	if len(s.logs) == 0 {
		err = ErrEndOfStream
		return
	}

	le, s.logs = s.logs[0], s.logs[1:]
	return
}

type errWriter struct {
	io.Writer
	err error
}

func (w *errWriter) Write(d []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	return w.Writer.Write(d)
}

type indexChecker struct {
	fixedSize int
}

func (ic *indexChecker) size(pb proto.Message) int {
	if ic.fixedSize > 0 {
		return ic.fixedSize
	}
	return proto.Size(pb)
}

// shouldContainIndexFor validates the correctness and completeness of the
// supplied index.
//
// actual should be a *bytes.Buffer that contains a serialized index protobuf.
//
// expected[0] should be the log stream descriptor.
// expected[1] should be a *bytes.Buffer that contains the log RecordIO stream.
//
// If additional expected elements are supplied, they are the specific integers
// that should appear in the index. Otherwise, it is assumed that the log is a
// complete index.
func (ic *indexChecker) shouldContainIndexFor(actual interface{}, expected ...interface{}) string {
	indexB := actual.(*bytes.Buffer)

	if len(expected) < 2 {
		return "at least two expected arguments are required"
	}
	desc := expected[0].(*logpb.LogStreamDescriptor)
	logB := expected[1].(*bytes.Buffer)
	expected = expected[2:]

	// Load our log index.
	index := logpb.LogIndex{}
	if err := proto.Unmarshal(indexB.Bytes(), &index); err != nil {
		return fmt.Sprintf("failed to unmarshal index protobuf: %v", err)
	}

	// Descriptors must match.
	if err := ShouldResemble(index.Desc, desc); err != "" {
		return err
	}

	// Catalogue the log entries in "expected".
	entries := map[uint64]*logpb.LogEntry{}
	offsets := map[uint64]int64{}
	csizes := map[uint64]uint64{}
	var eidx []uint64

	// Skip the first frame (descriptor).
	cr := iotools.CountingReader{
		Reader: logB,
	}
	csize := uint64(0)
	r := recordio.NewReader(&cr, 1024*1024)
	d, err := r.ReadFrameAll()
	if err != nil {
		return "failed to skip descriptor frame"
	}
	for {
		offset := cr.Count()
		d, err = r.ReadFrameAll()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Sprintf("failed to read entry #%d: %v", len(entries), err)
		}

		le := logpb.LogEntry{}
		if err := proto.Unmarshal(d, &le); err != nil {
			return fmt.Sprintf("failed to unmarshal entry #%d: %v", len(entries), err)
		}
		entries[le.StreamIndex] = &le
		offsets[le.StreamIndex] = offset
		csizes[le.StreamIndex] = csize

		csize += uint64(ic.size(&le))
		eidx = append(eidx, le.StreamIndex)
	}

	// Determine our expected archive indexes.
	if len(expected) > 0 {
		eidx = make([]uint64, 0, len(expected))
		for _, e := range expected {
			eidx = append(eidx, uint64(e.(int)))
		}
	}

	iidx := make([]uint64, len(index.Entries))
	for i, e := range index.Entries {
		iidx[i] = e.StreamIndex
	}
	if err := ShouldResemble(iidx, eidx); err != "" {
		return err
	}

	for i, cur := range index.Entries {
		cidx := eidx[i]
		le, offset := entries[uint64(cidx)], offsets[cidx]
		if le == nil {
			return fmt.Sprintf("no log entry for stream index %d", cidx)
		}

		if cur.StreamIndex != le.StreamIndex {
			return fmt.Sprintf("index entry %d has incorrect stream index (%d != %d)", i, cur.StreamIndex, le.StreamIndex)
		}
		if cur.Offset != uint64(offset) {
			return fmt.Sprintf("index entry %d has incorrect offset (%d != %d)", i, cur.Offset, offset)
		}
		if cur.PrefixIndex != le.PrefixIndex {
			return fmt.Sprintf("index entry %d has incorrect prefix index (%d != %d)", i, cur.StreamIndex, le.PrefixIndex)
		}
		if cur.TimeOffset.Duration() != le.TimeOffset.Duration() {
			return fmt.Sprintf("index entry %d has incorrect time offset (%v != %v)",
				i, cur.TimeOffset.Duration(), le.TimeOffset.Duration())
		}
		if cur.Sequence != le.Sequence {
			return fmt.Sprintf("index entry %d has incorrect sequence (%d != %d)", i, cur.Sequence, le.Sequence)
		}
	}
	return ""
}

func TestArchive(t *testing.T) {
	Convey(`A Manifest connected to Buffer Writers`, t, func() {
		var logB, indexB, dataB bytes.Buffer
		desc := &logpb.LogStreamDescriptor{
			Prefix: "test",
			Name:   "foo",
		}
		ic := indexChecker{}
		ts := testSource{}
		m := Manifest{
			Desc:        desc,
			Source:      &ts,
			LogWriter:   &logB,
			IndexWriter: &indexB,
			DataWriter:  &dataB,
		}

		Convey(`A sequence of logs will build a complete index.`, func() {
			ts.add(0, 1, 2, 3, 4, 5, 6)
			So(Archive(m), ShouldBeNil)

			So(&indexB, ic.shouldContainIndexFor, desc, &logB)
			So(dataB.String(), ShouldEqual, "0\n1\n2\n3\n4\n5\n6\n")
		})

		Convey(`A sequence of non-contiguous logs will build a complete index.`, func() {
			ts.add(0, 1, 3, 6)
			So(Archive(m), ShouldBeNil)

			So(&indexB, ic.shouldContainIndexFor, desc, &logB, 0, 1, 3, 6)
			So(dataB.String(), ShouldEqual, "0\n1\n3\n6\n")
		})

		Convey(`Out of order logs are ignored`, func() {
			Convey(`When StreamIndex is out of order.`, func() {
				ts.add(0, 2, 1, 3)
				So(Archive(m), ShouldBeNil)

				So(&indexB, ic.shouldContainIndexFor, desc, &logB, 0, 2, 3)
			})

			Convey(`When PrefixIndex is out of order.`, func() {
				ts.add(0, 1)
				le := gen(2)
				le.PrefixIndex = 1
				ts.addEntries(le)
				ts.add(3, 4)
				So(Archive(m), ShouldBeNil)

				So(&indexB, ic.shouldContainIndexFor, desc, &logB, 0, 1, 3, 4)
			})

			Convey(`When Sequence is out of order.`, func() {
				ts.add(0, 1)
				le := gen(2)
				le.Sequence = 0
				ts.addEntries(le)
				ts.add(3, 4)
				So(Archive(m), ShouldBeNil)

				So(&indexB, ic.shouldContainIndexFor, desc, &logB, 0, 1, 3, 4)
			})
		})

		Convey(`When TimeOffset is out of order, it is bumped to max.`, func() {
			ts.add(0, 1)
			le := gen(2)
			le.TimeOffset = nil // 0
			ts.addEntries(le)
			ts.add(3, 4)
			So(Archive(m), ShouldBeNil)

			So(&indexB, ic.shouldContainIndexFor, desc, &logB, 0, 1, 2, 3, 4)
		})

		Convey(`Source errors will be returned`, func() {
			Convey(`nil LogEntry`, func() {
				ts.addLogEntry(nil)
				So(Archive(m), ShouldErrLike, "nil LogEntry")
			})

			Convey(`Error returned`, func() {
				ts.add(0, 1, 2, 3, 4, 5)
				ts.err = errors.New("test error")
				So(Archive(m), ShouldErrLike, "test error")
			})
		})

		Convey(`Writer errors will be returned`, func() {
			ts.add(0, 1, 2, 3, 4, 5)

			Convey(`For log writer errors.`, func() {
				m.LogWriter = &errWriter{m.LogWriter, errors.New("test error")}
				So(errors.SingleError(Archive(m)), ShouldErrLike, "test error")
			})

			Convey(`For index writer errors.`, func() {
				m.IndexWriter = &errWriter{m.IndexWriter, errors.New("test error")}
				So(errors.SingleError(Archive(m)), ShouldErrLike, "test error")
			})

			Convey(`For data writer errors.`, func() {
				m.DataWriter = &errWriter{m.DataWriter, errors.New("test error")}
				So(errors.SingleError(Archive(m)), ShouldErrLike, "test error")
			})

			Convey(`When all Writers fail.`, func() {
				m.LogWriter = &errWriter{m.LogWriter, errors.New("test error")}
				m.IndexWriter = &errWriter{m.IndexWriter, errors.New("test error")}
				m.DataWriter = &errWriter{m.DataWriter, errors.New("test error")}
				So(Archive(m), ShouldNotBeNil)
			})
		})

		Convey(`When building sparse index`, func() {
			ts.add(0, 1, 2, 3, 4, 5)

			Convey(`Can build an index for every 3 StreamIndex.`, func() {
				m.StreamIndexRange = 3
				So(Archive(m), ShouldBeNil)

				So(&indexB, ic.shouldContainIndexFor, desc, &logB, 0, 3)
			})

			Convey(`Can build an index for every 3 PrefixIndex.`, func() {
				m.PrefixIndexRange = 3
				So(Archive(m), ShouldBeNil)

				// Note that in our generated logs, PrefixIndex = 2*StreamIndex.
				So(&indexB, ic.shouldContainIndexFor, desc, &logB, 0, 2, 4)
			})

			Convey(`Can build an index for every 13 bytes.`, func() {
				ic.fixedSize = 5
				m.ByteRange = 13
				m.sizeFunc = func(pb proto.Message) int {
					// Stub all LogEntry to be 5 bytes.
					return 5
				}
				So(Archive(m), ShouldBeNil)

				So(&indexB, ic.shouldContainIndexFor, desc, &logB, 0, 2, 5)
			})
		})
	})
}
