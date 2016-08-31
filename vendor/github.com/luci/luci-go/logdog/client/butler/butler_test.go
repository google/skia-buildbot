// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package butler

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/luci/luci-go/common/clock/testclock"
	. "github.com/luci/luci-go/common/testing/assertions"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/client/butler/output"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"
	"github.com/luci/luci-go/logdog/common/types"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

type testOutput struct {
	sync.Mutex

	err      error
	maxSize  int
	streams  map[string][]*logpb.LogEntry
	terminal map[string]struct{}
	closed   bool
}

func (to *testOutput) SendBundle(b *logpb.ButlerLogBundle) error {
	if to.err != nil {
		return to.err
	}

	to.Lock()
	defer to.Unlock()

	if to.streams == nil {
		to.streams = map[string][]*logpb.LogEntry{}
		to.terminal = map[string]struct{}{}
	}
	for _, be := range b.Entries {
		name := string(be.Desc.Name)

		to.streams[name] = append(to.streams[name], be.Logs...)
		if be.TerminalIndex >= 0 {
			to.terminal[name] = struct{}{}
		}
	}
	return nil
}

func (to *testOutput) MaxSize() int {
	return to.maxSize
}

func (to *testOutput) Stats() output.Stats {
	return &output.StatsBase{}
}

func (to *testOutput) Record() *output.EntryRecord { return nil }

func (to *testOutput) Close() {
	if to.closed {
		panic("double close")
	}
	to.closed = true
}

func (to *testOutput) logs(name string) []*logpb.LogEntry {
	to.Lock()
	defer to.Unlock()

	return to.streams[name]
}

func (to *testOutput) isTerminal(name string) bool {
	to.Lock()
	defer to.Unlock()

	_, isTerminal := to.terminal[name]
	return isTerminal
}

func shouldHaveTextLogs(actual interface{}, expected ...interface{}) string {
	exp := make([]string, len(expected))
	for i, e := range expected {
		exp[i] = e.(string)
	}

	var lines []string
	var prev string
	for _, le := range actual.([]*logpb.LogEntry) {
		if le.GetText() == nil {
			return fmt.Sprintf("non-text entry: %T %#v", le, le)
		}

		for _, l := range le.GetText().Lines {
			prev += l.Value
			if l.Delimiter != "" {
				lines = append(lines, prev)
				prev = ""
			}
		}
	}

	// Add any trailing (non-delimited) data.
	if prev != "" {
		lines = append(lines, prev)
	}

	return ShouldResemble(lines, exp)
}

type testStreamData struct {
	data []byte
	err  error
}

type testStream struct {
	inC     chan *testStreamData
	closedC chan struct{}

	properties *streamproto.Properties
}

func newTestStream(p streamproto.Properties) *testStream {
	return &testStream{
		inC:        make(chan *testStreamData, 16),
		closedC:    make(chan struct{}),
		properties: &p,
	}
}

func (ts *testStream) data(d []byte, err error) {
	ts.inC <- &testStreamData{
		data: d,
		err:  err,
	}
	if err == io.EOF {
		// If EOF is hit, continue reporting EOF.
		close(ts.inC)
	}
}

func (ts *testStream) err(err error) {
	ts.data(nil, err)
}

func (ts *testStream) isClosed() bool {
	select {
	case <-ts.closedC:
		return true
	default:
		return false
	}
}

func (ts *testStream) Close() error {
	if ts.isClosed() {
		panic("double close")
	}

	close(ts.closedC)
	return nil
}

func (ts *testStream) Read(b []byte) (int, error) {
	select {
	case <-ts.closedC:
		return 0, io.EOF

	case d, ok := <-ts.inC:
		// We have data on "inC", but we want closed status to trump this.
		if !ok || ts.isClosed() {
			return 0, io.EOF
		}
		return copy(b, d.data), d.err
	}
}

type testStreamServer struct {
	err     error
	onNext  func()
	streamC chan *testStream
}

func newTestStreamServer() *testStreamServer {
	return &testStreamServer{
		streamC: make(chan *testStream),
	}
}

func (tss *testStreamServer) Listen() error {
	return tss.err
}

func (tss *testStreamServer) Next() (io.ReadCloser, *streamproto.Properties) {
	if tss.onNext != nil {
		tss.onNext()
	}

	ts, ok := <-tss.streamC
	if !ok {
		return nil, nil
	}
	return ts, ts.properties
}

func (tss *testStreamServer) Close() {
	close(tss.streamC)
}

func (tss *testStreamServer) enqueue(ts *testStream) {
	tss.streamC <- ts
}

func TestConfig(t *testing.T) {
	t.Parallel()

	Convey(`A Config instance`, t, func() {
		to := testOutput{
			maxSize: 1024,
		}
		conf := Config{
			Output:  &to,
			Prefix:  "unit/test",
			Project: "test-project",
		}

		Convey(`Will validate.`, func() {
			So(conf.Validate(), ShouldBeNil)
		})

		Convey(`Will not validate with a nil Output.`, func() {
			conf.Output = nil
			So(conf.Validate(), ShouldErrLike, "an Output must be supplied")
		})

		Convey(`Will not validate with an invalid Prefix.`, func() {
			conf.Prefix = "!!!!invalid!!!!"
			So(conf.Validate(), ShouldErrLike, "invalid prefix")
		})

		Convey(`Will not validate with an invalid Project.`, func() {
			conf.Project = "!!!!invalid!!!!"
			So(conf.Validate(), ShouldErrLike, "invalid project")
		})
	})
}

// mkb is shorthand for "make Butler". It calls "new" and panics if there is
// an error.
func mkb(c context.Context, config Config) *Butler {
	b, err := New(c, config)
	if err != nil {
		panic(err)
	}
	return b
}

func TestButler(t *testing.T) {
	t.Parallel()

	Convey(`A testing Butler instance`, t, func() {
		c, _ := testclock.UseTime(context.Background(), testclock.TestTimeUTC)
		teeStdout := bytes.Buffer{}
		teeStderr := bytes.Buffer{}
		to := testOutput{
			maxSize: 1024,
		}

		conf := Config{
			Output:        &to,
			OutputWorkers: 1,
			BufferLogs:    false,
			Prefix:        "unit/test",
			Project:       "test-project",
			TeeStdout:     &teeStdout,
			TeeStderr:     &teeStderr,
		}

		Convey(`Will error if an invalid Config is passed.`, func() {
			conf.Output = nil
			So(conf.Validate(), ShouldNotBeNil)

			_, err := New(c, conf)
			So(err, ShouldNotBeNil)
		})

		Convey(`Will Run until Activated, then shut down.`, func() {
			conf.BufferLogs = true // (Coverage)

			b := mkb(c, conf)
			b.Activate()
			So(b.Wait(), ShouldBeNil)
		})

		Convey(`Will forward a Shutdown error.`, func() {
			conf.BufferLogs = true // (Coverage)

			b := mkb(c, conf)
			b.shutdown(errors.New("shutdown error"))
			So(b.Wait(), ShouldErrLike, "shutdown error")
		})

		Convey(`Will retain the first error and ignore duplicate shutdowns.`, func() {
			b := mkb(c, conf)
			b.shutdown(errors.New("first error"))
			b.shutdown(errors.New("second error"))
			So(b.Wait(), ShouldErrLike, "first error")
		})

		Convey(`Using a generic stream Properties`, func() {
			props := streamproto.Properties{
				LogStreamDescriptor: logpb.LogStreamDescriptor{
					Name:        "test",
					StreamType:  logpb.StreamType_TEXT,
					ContentType: string(types.ContentTypeText),
				},
			}

			Convey(`Will not add a stream with an invalid configuration.`, func() {
				// No content type.
				props.ContentType = ""

				s := newTestStream(props)
				b := mkb(c, conf)
				So(b.AddStream(s, *s.properties), ShouldNotBeNil)
			})

			Convey(`Will not add a stream with a duplicate stream name.`, func() {
				b := mkb(c, conf)

				s0 := newTestStream(props)
				So(b.AddStream(s0, *s0.properties), ShouldBeNil)

				s1 := newTestStream(props)
				So(b.AddStream(s1, *s1.properties), ShouldErrLike, "a stream has already been registered")
			})

			Convey(`Will not accept invalid tee configuration`, func() {
				conf.TeeStdout = nil
				conf.TeeStderr = nil

				for _, tc := range []struct {
					tee streamproto.TeeType
					err string
				}{
					{streamproto.TeeStdout, "no STDOUT is configured"},
					{streamproto.TeeStderr, "no STDERR is configured"},
					{streamproto.TeeType(0xFFFFFFFF), "invalid tee value"},
				} {
					Convey(fmt.Sprintf(`Rejects stream with TeeType [%v], when no tee outputs are configured.`, tc.tee), func() {
						props.Tee = tc.tee

						b := mkb(c, conf)
						s := newTestStream(props)
						So(b.AddStream(s, *s.properties), ShouldErrLike, tc.err)
					})
				}
			})

			Convey(`When adding a stream configured to tee through STDOUT/STDERR, tees.`, func() {
				props.Name = "stdout"
				props.Tee = streamproto.TeeStdout
				stdout := newTestStream(props)

				props.Name = "stderr"
				props.Tee = streamproto.TeeStderr
				stderr := newTestStream(props)

				b := mkb(c, conf)
				So(b.AddStream(stdout, *stdout.properties), ShouldBeNil)
				So(b.AddStream(stderr, *stderr.properties), ShouldBeNil)

				stdout.data([]byte("Hello, STDOUT"), io.EOF)
				stderr.data([]byte("Hello, STDERR"), io.EOF)

				b.Activate()
				So(b.Wait(), ShouldBeNil)

				So(teeStdout.String(), ShouldEqual, "Hello, STDOUT")
				So(to.logs("stdout"), shouldHaveTextLogs, "Hello, STDOUT")
				So(to.isTerminal("stdout"), ShouldBeTrue)

				So(teeStderr.String(), ShouldEqual, "Hello, STDERR")
				So(to.logs("stderr"), shouldHaveTextLogs, "Hello, STDERR")
				So(to.isTerminal("stderr"), ShouldBeTrue)
			})

			Convey(`Run with 256 streams, stream{0..256} will deplete and finish.`, func() {
				b := mkb(c, conf)
				streams := make([]*testStream, 256)
				for i := range streams {
					props.Name = fmt.Sprintf("stream%d", i)
					streams[i] = newTestStream(props)
				}

				for _, s := range streams {
					So(b.AddStream(s, *s.properties), ShouldBeNil)
					s.data([]byte("stream data 0!\n"), nil)
					s.data([]byte("stream data 1!\n"), nil)
				}

				// Add data to the streams after shutdown.
				for _, s := range streams {
					s.data([]byte("stream data 2!\n"), io.EOF)
				}

				b.Activate()
				So(b.Wait(), ShouldBeNil)

				for _, s := range streams {
					name := string(s.properties.Name)

					So(to.logs(name), shouldHaveTextLogs, "stream data 0!", "stream data 1!", "stream data 2!")
					So(to.isTerminal(name), ShouldBeTrue)
				}
			})

			Convey(`Shutdown with 256 in-progress streams, stream{0..256} will terminate if they emitted logs.`, func() {
				b := mkb(c, conf)
				streams := make([]*testStream, 256)
				for i := range streams {
					props.Name = fmt.Sprintf("stream%d", i)
					streams[i] = newTestStream(props)
				}

				for _, s := range streams {
					So(b.AddStream(s, *s.properties), ShouldBeNil)
					s.data([]byte("stream data!\n"), nil)
				}

				b.shutdown(errors.New("test shutdown"))
				So(b.Wait(), ShouldErrLike, "test shutdown")

				for _, s := range streams {
					if len(to.logs(s.properties.Name)) > 0 {
						So(to.isTerminal(string(s.properties.Name)), ShouldBeTrue)
					} else {
						So(to.isTerminal(string(s.properties.Name)), ShouldBeFalse)
					}
				}
			})

			Convey(`Using ten test stream servers`, func() {
				servers := make([]*testStreamServer, 10)
				for i := range servers {
					servers[i] = newTestStreamServer()
				}
				streams := []*testStream(nil)

				Convey(`Can register both before Run and will retain streams.`, func() {
					b := mkb(c, conf)
					for i, tss := range servers {
						b.AddStreamServer(tss)

						props.Name = fmt.Sprintf("stream%d", i)
						s := newTestStream(props)
						streams = append(streams, s)
						s.data([]byte("test data"), io.EOF)
						tss.enqueue(s)
					}

					b.Activate()
					So(b.Wait(), ShouldBeNil)

					for _, s := range streams {
						So(to.logs(s.properties.Name), shouldHaveTextLogs, "test data")
						So(to.isTerminal(s.properties.Name), ShouldBeTrue)
					}
				})

				Convey(`Can register both during Run and will retain streams.`, func() {
					b := mkb(c, conf)
					for i, tss := range servers {
						b.AddStreamServer(tss)

						props.Name = fmt.Sprintf("stream%d", i)
						s := newTestStream(props)
						streams = append(streams, s)
						s.data([]byte("test data"), io.EOF)
						tss.enqueue(s)
					}

					b.Activate()
					So(b.Wait(), ShouldBeNil)

					for _, s := range streams {
						So(to.logs(s.properties.Name), shouldHaveTextLogs, "test data")
						So(to.isTerminal(s.properties.Name), ShouldBeTrue)
					}
				})
			})

			Convey(`Will ignore stream registration errors, allowing re-registration.`, func() {
				tss := newTestStreamServer()

				// Generate an invalid stream for "tss" to register.
				sGood := newTestStream(props)
				sGood.data([]byte("good test data"), io.EOF)

				props.ContentType = ""
				sBad := newTestStream(props)
				sBad.data([]byte("bad test data"), io.EOF)

				b := mkb(c, conf)
				b.AddStreamServer(tss)
				tss.enqueue(sBad)
				tss.enqueue(sGood)
				b.Activate()
				So(b.Wait(), ShouldBeNil)

				So(sBad.isClosed(), ShouldBeTrue)
				So(sGood.isClosed(), ShouldBeTrue)
				So(to.logs(props.Name), shouldHaveTextLogs, "good test data")
				So(to.isTerminal(props.Name), ShouldBeTrue)
			})
		})

		Convey(`Will terminate if the stream server panics.`, func() {
			tss := newTestStreamServer()
			tss.onNext = func() {
				panic("test panic")
			}

			b := mkb(c, conf)
			b.AddStreamServer(tss)
			So(b.Wait(), ShouldErrLike, "test panic")
		})
	})
}
