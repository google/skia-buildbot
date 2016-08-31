// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/logdog/api/logpb"
	. "github.com/smartystreets/goconvey/convey"
)

type testParserCommand struct {
	// data is the data content of this command.
	data []byte
	// ts is the timestamp, which is valid if this is a data command.
	ts time.Time
	// splitToggle, if not null, causes this command to set the "allowSplit"
	// parser constraint.
	splitToggle *bool
	// closedToggle, if not null, causes this command to set the "closed"
	// parser constraint.
	closedToggle *bool
	// err, if not nil, is returned when this command is encountered.
	err error
}

var errTestInduced = errors.New("test error")

// testParser is a parser implementation that allows specifically-configured
// data to be emitted. It consumes commands, some of which alter its behavior
// and others of which present data. The resulting state affects how it emits
// LogEntry records via nextEntry.
type testParser struct {
	commands []*testParserCommand

	appendedData []byte
	truncateOn   bool
	closedOn     bool
	err          error

	// nextIndex is the next stream index to assign.
	nextIndex uint64
}

func (p *testParser) addCommand(r *testParserCommand) {
	p.commands = append(p.commands, r)
}

func (p *testParser) nextCommand(pop bool) *testParserCommand {
	if len(p.commands) == 0 {
		return nil
	}
	cmd := p.commands[0]
	if pop {
		p.commands = p.commands[1:]
	}
	return cmd
}

func (p *testParser) popData() (r *testParserCommand) {
	for i, cmd := range p.commands {
		if cmd.data != nil {
			p.commands = p.commands[i+1:]
			return cmd
		}
	}
	return nil
}

func (p *testParser) tags(ts time.Time, commands ...string) {
	for _, c := range commands {
		p.addTag(c, ts)
	}
}

func (p *testParser) addError(err error) {
	p.addCommand(&testParserCommand{
		err: err,
	})
}

func (p *testParser) addTag(tag string, ts time.Time) {
	p.addData([]byte(tag), ts)
}

func (p *testParser) addData(d []byte, ts time.Time) {
	p.addCommand(&testParserCommand{
		data: d,
		ts:   ts,
	})
}

func (p *testParser) setAllowSplit(value bool) {
	p.addCommand(&testParserCommand{
		splitToggle: &value,
	})
}

func (p *testParser) setClosed(value bool) {
	p.addCommand(&testParserCommand{
		closedToggle: &value,
	})
}

func (p *testParser) appendData(d Data) {
	p.addData(d.Bytes(), d.Timestamp())
}

func (p *testParser) nextEntry(c *constraints) (*logpb.LogEntry, error) {
	// Process records until we hit data or run out.
	for p.err == nil {
		rec := p.nextCommand(false)
		if rec == nil {
			return nil, p.err
		}

		// If this is a data record, process.
		if rec.data != nil {
			break
		}

		// Ingest commands, repeat.
		if rec.err != nil {
			p.err = rec.err
			break
		}

		if rec.splitToggle != nil {
			p.truncateOn = *rec.splitToggle
		}
		if rec.closedToggle != nil {
			p.closedOn = *rec.closedToggle
		}
		p.nextCommand(true)
	}

	if p.err != nil {
		return nil, p.err
	}

	// This is a data record. If we're configured to not yield it, leave it and
	// return nil.
	if p.truncateOn && (!c.allowSplit || (p.closedOn && !c.closed)) {
		return nil, nil
	}

	// Consume this record.
	rec := p.nextCommand(true)
	le := logpb.LogEntry{
		StreamIndex: p.nextIndex,
		Content: &logpb.LogEntry_Text{Text: &logpb.Text{
			Lines: []*logpb.Text_Line{
				{Value: string(rec.data)},
			},
		}},
	}
	p.nextIndex++
	return &le, nil
}

func (p *testParser) bufferedBytes() (r int64) {
	for _, rec := range p.commands {
		r += int64(len(rec.data))
	}
	return
}

func (p *testParser) firstChunkTime() (time.Time, bool) {
	for _, c := range p.commands {
		if c.data != nil {
			return c.ts, true
		}
	}
	return time.Time{}, false
}

func TestStream(t *testing.T) {
	Convey(`A testing stream config`, t, func() {
		tc := testclock.New(time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC))
		tp := testParser{}
		c := streamConfig{
			name:   "test",
			parser: &tp,
			template: logpb.ButlerLogBundle_Entry{
				Desc: &logpb.LogStreamDescriptor{
					Prefix: "test-prefix",
					Name:   "test",
				},
			},
		}

		Convey(`With a 64-byte maximum buffer and 1 second maximum duration`, func() {
			c.maximumBufferedBytes = 64
			c.maximumBufferDuration = time.Second
			s := newStream(c)

			Convey(`Is not drained by default`, func() {
				So(s.isDrained(), ShouldBeFalse)
			})

			Convey(`With no data, has no expiration time.`, func() {
				_, has := s.expireTime()
				So(has, ShouldBeFalse)
			})

			Convey(`Append will ignore a 0-byte chunk.`, func() {
				d := data(tc.Now())
				So(s.Append(d), ShouldBeNil)
				So(d.released, ShouldBeTrue)
			})

			Convey(`Append will add two 32-byte chunks.`, func() {
				content := bytes.Repeat([]byte{0xAA}, 32)
				So(s.Append(data(tc.Now(), content...)), ShouldBeNil)
				So(s.Append(data(tc.Now(), content...)), ShouldBeNil)
			})

			Convey(`Append will add a large chunk when there are no other Data blocks.`, func() {
				d := data(tc.Now(), bytes.Repeat([]byte{0xAA}, 128)...)
				So(s.Append(d), ShouldBeNil)

				Convey(`Will use that data's timestamp as expiration time.`, func() {
					t, has := s.expireTime()
					So(has, ShouldBeTrue)
					So(t.Equal(tc.Now().Add(time.Second)), ShouldBeTrue)
				})
			})

			Convey(`Append will block if the chunk exceeds the buffer size.`, func() {
				signalC := make(chan struct{})
				s.c.onAppend = func(appended bool) {
					if !appended {
						// We're waiting.
						close(signalC)
					}
				}

				// Add one chunk so we don't hit the "only byte" condition.
				So(s.Append(data(tc.Now(), bytes.Repeat([]byte{0xAA}, 34)...)), ShouldBeNil)

				// Wait until we get the signal that Append() will block, then consume
				// some data and unblock Append().
				blocked := false
				go func() {
					<-signalC

					s.withParserLock(func() error {
						tp.popData()
						return nil
					})
					blocked = true
					s.signalDataConsumed()
				}()

				// Add one chunk so we don't hit the "only byte" condition.
				So(s.Append(data(tc.Now(), bytes.Repeat([]byte{0xBB}, 32)...)), ShouldBeNil)
				So(blocked, ShouldBeTrue)
			})

			Convey(`Append in an error state`, func() {
				terr := errors.New("test error")

				Convey(`Will return the error state.`, func() {
					s.appendErr = terr

					d := data(tc.Now(), bytes.Repeat([]byte{0xAA}, 32)...)
					So(s.Append(d), ShouldEqual, terr)
					So(d.released, ShouldBeTrue)
				})

				Convey(`Will block if the chunk exceeds buffer size, and return error state.`, func() {
					signalC := make(chan struct{})
					s.c.onAppend = func(appended bool) {
						if !appended {
							// Waiting, notify our goroutine that we're going to be waiting.
							close(signalC)
						}
					}

					// Add one chunk so we don't hit the "only byte" condition.
					So(s.Append(data(tc.Now(), bytes.Repeat([]byte{0xAA}, 34)...)), ShouldBeNil)

					// Wait until we get the signal that Append() will block, then consume
					// some data and unblock Append().
					go func() {
						<-signalC

						s.stateLock.Lock()
						defer s.stateLock.Unlock()
						s.setAppendErrorLocked(terr)
					}()

					// Add one chunk so we don't hit the "only byte" condition.
					for _, sz := range []int{32, 1, 0} {
						d := data(tc.Now(), bytes.Repeat([]byte{0xAA}, sz)...)
						So(s.Append(d), ShouldEqual, terr)
						So(d.released, ShouldBeTrue)
					}
				})
			})
		})

		Convey(`When building bundle entries`, func() {
			bb := &builder{
				size: 1024,
			}
			s := newStream(c)

			Convey(`Returns nil with no buffered data.`, func() {
				So(s.nextBundleEntry(bb, false), ShouldBeFalse)
				So(bb.bundle(), shouldHaveBundleEntries)
			})

			Convey(`With a single record, returns that entry.`, func() {
				tp.tags(tc.Now(), "a", "b")

				So(s.nextBundleEntry(bb, false), ShouldBeTrue)
				So(bb.bundle(), shouldHaveBundleEntries, "test:a:b")
			})

			Convey(`When split is allowed, returns nil.`, func() {
				tp.tags(tc.Now(), "a", "b")
				tp.setAllowSplit(true)
				tp.tags(tc.Now(), "c")

				So(s.nextBundleEntry(bb, false), ShouldBeTrue)
				So(bb.bundle(), shouldHaveBundleEntries, "test:a:b")
				So(s.nextBundleEntry(bb, false), ShouldBeFalse)

				So(s.nextBundleEntry(bb, true), ShouldBeTrue)
				So(bb.bundle(), shouldHaveBundleEntries, "test:a:b:c")
			})

			Convey(`When an error occurs during stream parsing, drains stream.`, func() {
				So(s.isDrained(), ShouldBeFalse)
				tp.tags(tc.Now(), "a")
				tp.addError(errTestInduced)
				tp.tags(tc.Now(), "b")

				So(s.nextBundleEntry(bb, false), ShouldBeTrue)
				So(s.isDrained(), ShouldBeTrue)
				So(bb.bundle(), shouldHaveBundleEntries, "+test:a")
				So(s.nextBundleEntry(bb, false), ShouldBeFalse)
			})

			Convey(`With only an error, returns no bundle entry.`, func() {
				So(s.isDrained(), ShouldBeFalse)
				tp.addError(errTestInduced)
				tp.tags(tc.Now(), "a")
				tp.tags(tc.Now(), "b")

				So(s.nextBundleEntry(bb, false), ShouldBeFalse)
				So(bb.bundle(), shouldHaveBundleEntries)
				So(s.isDrained(), ShouldBeTrue)
			})
		})
	})
}

// TestStreamSmoke tests a Stream in an actual multi-goroutine workflow.
func TestStreamSmoke(t *testing.T) {
	Convey(`When running a smoke test`, t, func() {
		tc := testclock.New(time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC))
		tp := testParser{}
		c := streamConfig{
			name:   "test",
			parser: &tp,
			template: logpb.ButlerLogBundle_Entry{
				Desc: &logpb.LogStreamDescriptor{
					Prefix: "test-prefix",
					Name:   "test",
				},
			},
		}
		s := newStream(c)

		// Appender goroutine, constantly appends data.
		//
		// This will be inherently throttled by the nextBundle consumption.
		dataTokenC := make(chan struct{}, 512)
		go func() {
			defer func() {
				close(dataTokenC)
				s.Close()
			}()

			for i := 0; i < 512; i++ {
				s.Append(data(tc.Now(), []byte(fmt.Sprintf("%d", i))...))

				// Note that data has been sent.
				dataTokenC <- struct{}{}
			}
		}()

		// The consumer goroutine will consume bundles from the stream.
		consumerC := make(chan struct{})
		bundleC := make(chan *logpb.ButlerLogBundle)
		for i := 0; i < 32; i++ {
			go func() {
				defer func() {
					consumerC <- struct{}{}
				}()

				b := (*builder)(nil)
				for !s.isDrained() {
					if b == nil {
						b = &builder{
							size: 128,
						}
					}

					s.nextBundleEntry(b, false)
					if b.hasContent() {
						bundleC <- b.bundle()
						b = nil
					} else {
						// No content! Sleep for a second and check again.
						<-dataTokenC
					}
				}
			}()
		}

		// Collect all bundles.
		gotIt := map[int]struct{}{}
		collectDoneC := make(chan struct{})
		go func() {
			defer close(collectDoneC)

			for bundle := range bundleC {
				for _, be := range bundle.Entries {
					for _, le := range be.Logs {
						idx, _ := strconv.Atoi(logEntryName(le))
						gotIt[idx] = struct{}{}
					}
				}
			}
		}()

		// Awaken all sleeping goroutines.
		tc.Add(32 * time.Second)
		for i := 0; i < 32; i++ {
			<-consumerC
		}
		close(bundleC)

		// Did we get them all?
		<-collectDoneC
		for i := 0; i < 512; i++ {
			_, ok := gotIt[i]
			So(ok, ShouldBeTrue)
		}
	})
}
