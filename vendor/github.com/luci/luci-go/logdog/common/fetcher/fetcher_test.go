// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package fetcher

import (
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/testing/assertions"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/common/types"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

type testSourceCommand struct {
	logEntries []*logpb.LogEntry

	err   error
	panic bool

	tidx    types.MessageIndex
	tidxSet bool
}

// addLogs adds a text log entry for each named indices. The text entry contains
// a single line, "#x", where "x" is the log index.
func (cmd *testSourceCommand) logs(indices ...int64) *testSourceCommand {
	for _, idx := range indices {
		cmd.logEntries = append(cmd.logEntries, &logpb.LogEntry{
			StreamIndex: uint64(idx),
		})
	}
	return cmd
}

func (cmd *testSourceCommand) terminalIndex(v types.MessageIndex) *testSourceCommand {
	cmd.tidx, cmd.tidxSet = v, true
	return cmd
}

func (cmd *testSourceCommand) error(err error, panic bool) *testSourceCommand {
	cmd.err, cmd.panic = err, panic
	return cmd
}

type testSource struct {
	sync.Mutex

	logs     map[types.MessageIndex]*logpb.LogEntry
	err      error
	panic    bool
	terminal types.MessageIndex

	history []int
}

func newTestSource() *testSource {
	return &testSource{
		terminal: -1,
		logs:     make(map[types.MessageIndex]*logpb.LogEntry),
	}
}

func (ts *testSource) LogEntries(c context.Context, req *LogRequest) ([]*logpb.LogEntry, types.MessageIndex, error) {
	ts.Lock()
	defer ts.Unlock()

	if ts.err != nil {
		if ts.panic {
			panic(ts.err)
		}
		return nil, 0, ts.err
	}

	// We have at least our next log. Build our return value.
	maxCount := req.Count
	if maxCount <= 0 {
		maxCount = len(ts.logs)
	}
	maxBytes := req.Bytes

	var logs []*logpb.LogEntry
	bytes := int64(0)
	index := req.Index
	for {
		if len(logs) >= maxCount {
			break
		}

		log, ok := ts.logs[index]
		if !ok {
			break
		}

		size := int64(5) // We've rigged all logs to have size 5.
		if len(logs) > 0 && maxBytes > 0 && (bytes+size) > maxBytes {
			break
		}
		logs = append(logs, log)
		index++
		bytes += size
	}
	ts.history = append(ts.history, len(logs))
	return logs, ts.terminal, nil
}

func (ts *testSource) send(cmd *testSourceCommand) {
	ts.Lock()
	defer ts.Unlock()

	if cmd.err != nil {
		ts.err, ts.panic = cmd.err, cmd.panic
	}
	if cmd.tidxSet {
		ts.terminal = cmd.tidx
	}
	for _, le := range cmd.logEntries {
		ts.logs[types.MessageIndex(le.StreamIndex)] = le
	}
}

func (ts *testSource) getHistory() []int {
	ts.Lock()
	defer ts.Unlock()

	h := make([]int, len(ts.history))
	copy(h, ts.history)
	return h
}

func loadLogs(f *Fetcher, count int) (result []types.MessageIndex, err error) {
	for {
		// Specific limit, hit that limit.
		if count > 0 && len(result) >= count {
			return
		}

		var le *logpb.LogEntry
		le, err = f.NextLogEntry()
		if le != nil {
			result = append(result, types.MessageIndex(le.StreamIndex))
		}
		if err != nil {
			return
		}
	}
}

func TestFetcher(t *testing.T) {
	t.Parallel()

	Convey(`A testing log Source`, t, func() {
		c, tc := testclock.UseTime(context.Background(), testclock.TestTimeLocal)
		c, cancelFunc := context.WithCancel(c)

		ts := newTestSource()
		o := Options{
			Source: ts,

			// All message byte sizes will be 5.
			sizeFunc: func(proto.Message) int {
				return 5
			},
		}

		newFetcher := func() *Fetcher { return New(c, o) }
		reap := func(f *Fetcher) {
			cancelFunc()
			loadLogs(f, 0)
		}

		Convey(`Uses defaults values when not overridden, and stops when cancelled.`, func() {
			f := newFetcher()
			defer reap(f)

			So(f.o.BufferCount, ShouldEqual, 0)
			So(f.o.BufferBytes, ShouldEqual, DefaultBufferBytes)
			So(f.o.PrefetchFactor, ShouldEqual, 1)
		})

		Convey(`With a Count limit of 3.`, func() {
			o.BufferCount = 3

			Convey(`Will pull 6 sequential log records.`, func() {
				var cmd testSourceCommand
				ts.send(cmd.logs(0, 1, 2, 3, 4, 5).terminalIndex(5))

				f := newFetcher()
				defer reap(f)

				logs, err := loadLogs(f, 0)
				So(err, ShouldEqual, io.EOF)
				So(logs, ShouldResemble, []types.MessageIndex{0, 1, 2, 3, 4, 5})
			})

			Convey(`Can read two log records and be cancelled.`, func() {
				var cmd testSourceCommand
				ts.send(cmd.logs(0, 1, 2, 3, 4, 5))

				f := newFetcher()
				defer reap(f)

				logs, err := loadLogs(f, 2)
				So(err, ShouldBeNil)
				So(logs, ShouldResemble, []types.MessageIndex{0, 1})

				cancelFunc()
				_, err = loadLogs(f, 0)
				So(err, ShouldEqual, context.Canceled)
			})

			Convey(`Will delay for more log records if none are available.`, func() {
				delayed := false
				tc.SetTimerCallback(func(d time.Duration, t clock.Timer) {
					// Add the remaining logs.
					delayed = true

					var cmd testSourceCommand
					ts.send(cmd.logs(1, 2).terminalIndex(2))

					tc.Add(d)
				})

				var cmd testSourceCommand
				ts.send(cmd.logs(0))

				f := newFetcher()
				defer reap(f)

				logs, err := loadLogs(f, 1)
				So(err, ShouldBeNil)
				So(logs, ShouldResemble, []types.MessageIndex{0})

				logs, err = loadLogs(f, 0)
				So(err, ShouldEqual, io.EOF)
				So(logs, ShouldResemble, []types.MessageIndex{1, 2})
				So(delayed, ShouldBeTrue)
			})

			Convey(`When an error is countered getting the terminal index, returns the error.`, func() {
				var cmd testSourceCommand
				ts.send(cmd.error(errors.New("test error"), false))

				f := newFetcher()
				defer reap(f)

				_, err := loadLogs(f, 0)
				So(err, assertions.ShouldErrLike, "test error")
			})

			Convey(`When an error is countered fetching logs, returns the error.`, func() {
				var cmd testSourceCommand
				ts.send(cmd.logs(0, 1, 2).error(errors.New("test error"), false))

				f := newFetcher()
				defer reap(f)

				_, err := loadLogs(f, 0)
				So(err, assertions.ShouldErrLike, "test error")
			})

			Convey(`If the source panics, it is caught and returned as an error.`, func() {
				var cmd testSourceCommand
				ts.send(cmd.error(errors.New("test error"), true))

				f := newFetcher()
				defer reap(f)

				_, err := loadLogs(f, 0)
				So(err, assertions.ShouldErrLike, "panic during fetch")
			})
		})

		Convey(`With a byte limit of 15`, func() {
			o.BufferBytes = 15
			o.PrefetchFactor = 2

			var cmd testSourceCommand
			ts.send(cmd.logs(0, 1, 2, 3, 4, 5, 6).terminalIndex(6))

			f := newFetcher()
			defer reap(f)

			// First fetch should have asked for 30 bytes (2*15), so 6 logs. After
			// first log was kicked, there is a deficit of one log.
			logs, err := loadLogs(f, 0)
			So(err, ShouldEqual, io.EOF)
			So(logs, ShouldResemble, []types.MessageIndex{0, 1, 2, 3, 4, 5, 6})

			So(ts.getHistory(), ShouldResemble, []int{6, 1})
		})

		Convey(`With an index of 1 and a maximum count of 1, fetches exactly 1 log.`, func() {
			o.Index = 1
			o.Count = 1

			var cmd testSourceCommand
			ts.send(cmd.logs(0, 1, 2, 3, 4, 5, 6).terminalIndex(6))

			f := newFetcher()
			defer reap(f)

			// First fetch will ask for exactly one log.
			logs, err := loadLogs(f, 0)
			So(err, ShouldEqual, io.EOF)
			So(logs, ShouldResemble, []types.MessageIndex{1})

			So(ts.getHistory(), ShouldResemble, []int{1})
		})
	})
}
