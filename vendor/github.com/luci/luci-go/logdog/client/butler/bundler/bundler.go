// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/common/sync/cancelcond"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"
)

// Config is the Bundler configuration.
type Config struct {
	// Clock is the clock instance that will be used for Bundler and stream
	// timing.
	Clock clock.Clock

	// Project is the project to use.
	Project config.ProjectName
	// Prefix is the common prefix for this set of streams.
	Prefix types.StreamName

	// MaxBufferedBytes is the maximum number of bytes to buffer in memory per
	// stream.
	MaxBufferedBytes int64

	// MaxBundleSize is the maximum bundle size in bytes that may be generated.
	//
	// If this value is zero, no size constraint will be applied to generated
	// bundles.
	MaxBundleSize int

	// MaxBufferDelay is the maximum amount of time we're willing to buffer
	// bundled data. Other factors can cause the bundle to be sent before this,
	// but it is an upper bound.
	MaxBufferDelay time.Duration
}

type bundlerStream interface {
	isDrained() bool
	name() string
	expireTime() (time.Time, bool)
	nextBundleEntry(*builder, bool) bool
}

// Bundler is the main Bundler instance. It exposes goroutine-safe endpoints for
// stream registration and bundle consumption.
type Bundler struct {
	c *Config

	// finishedC is closed when makeBundles goroutine has terminated.
	finishedC chan struct{}
	bundleC   chan *logpb.ButlerLogBundle

	// streamsLock is a lock around the `streams` map and its contents.
	streamsLock sync.Mutex
	// streamsCond is a Cond bound to streamsLock, used to signal Next() when new
	// data is available.
	streamsCond *cancelcond.Cond
	// streams is the set of currently-registered Streams.
	streams map[string]bundlerStream
	// flushing is true if we're blocking on CloseAndFlush().
	flushing bool

	// prefixCounter is a global counter for Prefix-wide streams.
	prefixCounter counter
}

// New instantiates a new Bundler instance.
func New(c Config) *Bundler {
	b := Bundler{
		c:         &c,
		finishedC: make(chan struct{}),
		bundleC:   make(chan *logpb.ButlerLogBundle),
		streams:   map[string]bundlerStream{},
	}
	b.streamsCond = cancelcond.New(&b.streamsLock)

	go b.makeBundles()
	return &b
}

// Register adds a new stream to the Bundler, returning a reference to the
// registered stream.
func (b *Bundler) Register(p streamproto.Properties) (Stream, error) {
	// Our Properties must validate.
	if err := p.Validate(); err != nil {
		return nil, err
	}

	// Enforce that the log stream descriptor's Prefix is empty.
	p.Prefix = ""

	// Construct a parser for this stream.
	c := streamConfig{
		name: p.Name,
		template: logpb.ButlerLogBundle_Entry{
			Desc: &p.LogStreamDescriptor,
		},
		maximumBufferDuration: b.c.MaxBufferDelay,
		maximumBufferedBytes:  b.c.MaxBufferedBytes,
		onAppend: func(appended bool) {
			if appended {
				b.signalStreamUpdate()
			}
		},
	}

	err := error(nil)
	c.parser, err = newParser(&p, &b.prefixCounter)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream parser: %s", err)
	}

	b.streamsLock.Lock()
	defer b.streamsLock.Unlock()

	// Ensure that this is not a duplicate stream name.
	if s := b.streams[p.Name]; s != nil {
		return nil, fmt.Errorf("a Stream is already registered for %q", p.Name)
	}

	// Create a new stream. This will kick off its processing goroutine, which
	// will not stop until it is closed.
	s := newStream(c)
	b.registerStreamLocked(s)
	return s, nil
}

// CloseAndFlush closes the Bundler, alerting it that no more streams will be
// added and that existing data may be aggressively output.
//
// CloseAndFlush will block until all buffered data has been consumed.
func (b *Bundler) CloseAndFlush() {
	// Mark that we're flushing. This will cause us to perform more aggressive
	// bundling in Next().
	b.startFlushing()
	<-b.finishedC
}

// Next returns the next bundle, blocking until it is available.
func (b *Bundler) Next() *logpb.ButlerLogBundle {
	return <-b.bundleC
}

func (b *Bundler) startFlushing() {
	b.streamsLock.Lock()
	defer b.streamsLock.Unlock()

	if !b.flushing {
		b.flushing = true
		b.signalStreamUpdateLocked()
	}
}

// makeBundles is run in its own goroutine. It runs continuously, responding
// to Stream constraints and availability and sending ButlerLogBundles through
// bundleC when available.
//
// makeBundles will terminate when closeC is closed and all streams are drained.
func (b *Bundler) makeBundles() {
	defer close(b.finishedC)
	defer close(b.bundleC)

	b.streamsLock.Lock()
	defer b.streamsLock.Unlock()

	var bb *builder
	defer func() {
		if bb != nil && bb.hasContent() {
			b.bundleC <- bb.bundle()
		}
	}()

	for {
		bb = &builder{
			size: b.c.MaxBundleSize,
			template: logpb.ButlerLogBundle{
				Timestamp: google.NewTimestamp(b.getClock().Now()),
				Project:   string(b.c.Project),
				Prefix:    string(b.c.Prefix),
			},
		}
		var oldestContentTime time.Time

		for {
			state := b.getStreamStateLocked()

			// Attempt to create more bundles.
			sendNow := b.bundleRoundLocked(bb, state)

			// Prune and unregister any drained streams.
			state.forEachStream(func(s bundlerStream) bool {
				if s.isDrained() {
					state.removeStream(s.name())
					b.unregisterStreamLocked(s)
				}

				return true
			})

			if b.flushing && len(state.streams) == 0 {
				// We're flushing, and there are no more streams, so we're completely
				// finished.
				//
				// If we have any content in our builder, it will be exported via defer.
				return
			}

			// If we have content, consider emitting this bundle.
			if bb.hasContent() && (b.c.MaxBufferDelay == 0 || sendNow || bb.ready()) {
				break
			}

			// Mark the first time this round where we actually saw data.
			if oldestContentTime.IsZero() && bb.hasContent() {
				oldestContentTime = state.now
			}

			// We will yield our stream lock and sleep, waiting for either:
			// 1) The earliest expiration time.
			// 2) A streams channel signal.
			//
			// We use a Cond here because we want Streams to be able to be added
			// while we're waiting for stream data.
			nextExpire, has := state.nextExpire()

			// If we have an oldest content time, that also means that we have
			// content. Factor this constraint in.
			if !oldestContentTime.IsZero() {
				roundExpire := oldestContentTime.Add(b.c.MaxBufferDelay)
				if !roundExpire.After(state.now) {
					break
				}

				if !has || roundExpire.Before(nextExpire) {
					nextExpire = roundExpire
					has = true
				}
			}

			// If we had no data or expire constraints, wait indefinitely for
			// something to change.
			//
			// This will release our state lock during switch execution. The lock will
			// be held after the switch statement has finished.
			condC := context.Background()
			switch {
			case has && nextExpire.After(state.now):
				// No immediate data, so block until the next known data expiration
				// time.
				condC, _ = context.WithDeadline(condC, nextExpire)
				b.streamsCond.Wait(condC)

			case has:
				// There is more data, and it has already expired, so go immediately.
				break

			default:
				// No data, and no enqueued stream data, so block indefinitely until we
				// get a signal.
				b.streamsCond.Wait(condC)
			}
		}

		// If our bundler has contents, send them.
		if bb.hasContent() {
			b.bundleC <- bb.bundle()
		}
	}
}

// Implements a single bundle building round. This incrementally adds data from
// the stream state to the supplied builder.
//
// This method will block until a suitable bundle is available. Availability
// is subject both to time and data constraints:
//   - If buffered data, which is timestampped at ingest, has exceeded its
//     buffer duration threshold, a Bundle will be cut immediately.
//   - If no data is set to expire, the Bundler may wait for more data to
//     produce a more optimally-packed bundle.
//
// At a high level, Next operates as follows:
//   1) Freeze all stream state.
//   2) Scan streams for data that has exceeded its threshold; if data is found:
//     - Aggressively pack expired data into a Bundle.
//     - Optimally pack the remainder of the Bundle with any available data.
//     - Return the Bundle.
//
//   3) Examine the remaining data sizes, waiting for either:
//     - Enough stream data to fill our Bundle.
//     - Our timeout, if the Bundler is not closed.
//   4) Pack a Bundle with the remaining data optimally, emphasizing streams
//      with older data.
//
// Returns true if bundle some data was added that should be sent immediately.
func (b *Bundler) bundleRoundLocked(bb *builder, state *streamState) bool {
	sendNow := false

	// First pass: non-blocking data that has exceeded its storage threshold.
	for bb.remaining() > 0 {
		s := state.next()
		if s == nil || s.isDrained() {
			break
		}

		if et, has := s.expireTime(); !has || et.After(state.now) {
			// This stream (and all other streams, since we're sorted) expires in
			// the future, so we're done with the first pass.
			break
		}

		// Pull bundles from this stream.
		if modified := s.nextBundleEntry(bb, true); modified {
			state.streamUpdated(s.name())

			// We have at least one time-sensitive bundle, so send this round.
			sendNow = true
		}

		if s.isDrained() {
			state.removeStream(s.name())
		}
	}

	// Second pass: bundle any available data.
	state.forEachStream(func(s bundlerStream) bool {
		if bb.remaining() == 0 {
			return false
		}

		if modified := s.nextBundleEntry(bb, b.flushing); modified {
			state.streamUpdated(s.name())
		}
		return true
	})

	return sendNow
}

func (b *Bundler) getStreamStateLocked() *streamState {
	// Lock and collect each stream.
	state := &streamState{
		streams: make([]bundlerStream, 0, len(b.streams)),
		now:     b.getClock().Now(),
	}

	for _, s := range b.streams {
		state.streams = append(state.streams, s)
	}
	heap.Init(state)

	return state
}

func (b *Bundler) registerStreamLocked(s bundlerStream) {
	b.streams[s.name()] = s
	b.signalStreamUpdateLocked()
}

func (b *Bundler) unregisterStreamLocked(s bundlerStream) {
	delete(b.streams, s.name())
}

func (b *Bundler) signalStreamUpdate() {
	b.streamsLock.Lock()
	defer b.streamsLock.Unlock()

	b.signalStreamUpdateLocked()
}

func (b *Bundler) signalStreamUpdateLocked() {
	b.streamsCond.Broadcast()
}

// nextPrefixIndex is a goroutine-safe method that returns the next prefix index
// for the given Bundler.
func (b *Bundler) nextPrefixIndex() uint64 {
	return uint64(b.prefixCounter.next())
}

func (b *Bundler) getClock() clock.Clock {
	c := b.c.Clock
	if c != nil {
		return c
	}
	return clock.GetSystemClock()
}

// streamState is a snapshot of the current stream registration. All operations
// performed on the state require streamLock to be held.
//
// streamState implements heap.Interface for its streams array. Streams without
// data times (nil) are considered to be greater than those with times.
type streamState struct {
	streams []bundlerStream
	now     time.Time
}

var _ heap.Interface = (*streamState)(nil)

func (s *streamState) next() bundlerStream {
	if len(s.streams) == 0 {
		return nil
	}
	return s.streams[0]
}

func (s *streamState) nextExpire() (time.Time, bool) {
	if next := s.next(); next != nil {
		if ts, ok := next.expireTime(); ok {
			return ts, true
		}
	}
	return time.Time{}, false
}

func (s *streamState) streamUpdated(name string) {
	if si, idx := s.streamIndex(name); si != nil {
		heap.Fix(s, idx)
	}
}

func (s *streamState) forEachStream(f func(bundlerStream) bool) {
	// Clone our streams, since the callback may mutate their order.
	streams := make([]bundlerStream, len(s.streams))
	for i, s := range s.streams {
		streams[i] = s
	}

	for _, s := range streams {
		if !f(s) {
			break
		}
	}
}

// removeStream removes a stream from the stream state.
func (s *streamState) removeStream(name string) bundlerStream {
	if si, idx := s.streamIndex(name); si != nil {
		heap.Remove(s, idx)
		return si
	}
	return nil
}

func (s *streamState) streamIndex(name string) (bundlerStream, int) {
	for i, si := range s.streams {
		if si.name() == name {
			return si, i
		}
	}
	return nil, -1
}

func (s *streamState) Len() int {
	return len(s.streams)
}

func (s *streamState) Less(i, j int) bool {
	si, sj := s.streams[i], s.streams[j]

	if it, ok := si.expireTime(); ok {
		if jt, ok := sj.expireTime(); ok {
			return it.Before(jt)
		}

		// i has data, but j does not, so i < j.
		return true
	}

	// i has no data, so i us greater than all other streams.
	return false
}

func (s *streamState) Swap(i, j int) {
	s.streams[i], s.streams[j] = s.streams[j], s.streams[i]
}

func (s *streamState) Push(x interface{}) {
	s.streams = append(s.streams, x.(bundlerStream))
}

func (s *streamState) Pop() interface{} {
	last := s.streams[len(s.streams)-1]
	s.streams = s.streams[:len(s.streams)-1]
	return last
}
