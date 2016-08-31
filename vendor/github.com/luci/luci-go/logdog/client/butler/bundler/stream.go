// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/luci/luci-go/logdog/api/logpb"
)

var (
	// dataBufferSize is the size (in bytes) of the Data objects that a Stream
	// will lease.
	dataBufferSize = 4096
)

// Stream is an individual Bundler Stream. Data is added to the Stream as a
// series of ordered binary chunks.
//
// A Stream is not goroutine-safe.
type Stream interface {
	// LeaseData allocates and returns a Data block that stream data can be
	// loaded into. The caller should Release() the Data, or transfer ownership to
	// something that will (e.g., Append()).
	LeaseData() Data

	// Append adds a sequential chunk of data to the Stream. Append may block if
	// the data isn't ready to be consumed.
	//
	// Append takes possession of the supplied Data, and will Release it when
	// finished.
	Append(Data) error

	// Close closes the Stream, flushing any remaining data.
	Close()
}

// streamConfig is the set of static configuration parameters for the stream.
type streamConfig struct {
	// name is the name of this stream.
	name string

	// parser is the stream parser to use.
	parser parser

	// maximumBufferedBytes is the maximum number of bytes that this stream will
	// retain in its parser before blocking subsequent Append attempts.
	maximumBufferedBytes int64
	// maximumBufferDuration is the maximum amount of time that a block of data
	// can be comfortably buffered in the stream.
	maximumBufferDuration time.Duration

	// template is the minimally-populated Butler log bundle entry.
	template logpb.ButlerLogBundle_Entry

	// onAppend, if not nil, is invoked when an attempt to append data to the
	// stream occurs. If true is passed, the data was successfully appended. If
	// false was passed, the data could not be appended immediately and the stream
	// will block pending data consumption.
	//
	// The stream's append lock will be held when this method is called.
	onAppend func(bool)
}

// streamImpl is a Stream implementation that is bound to a Bundler.
type streamImpl struct {
	c *streamConfig

	// drained is true if the stream is finished emitting data, including its
	// terminal state.
	//
	// It is an atomic value, with zero indicating not drained and non-zero
	// indicating drained. It should be accessed via isDrained, and set with
	// setDrained.
	drained int32

	// parserLock is a Mutex protecting the stream's parser instance and its
	// underlying chunk.Buffer. Any access to either of these fields must be done
	// while holding this lock.
	parserLock sync.Mutex

	// dataConsumedSignalC is a channel that can be used to signal when data has
	// been consumed. It is set via signalDataConsumed.
	dataConsumedSignalC chan struct{}

	// stateLock protects stream state against concurrent access.
	stateLock sync.Mutex

	// closed, if non-zero, indicates that we have been closed and our stream has
	// finished reading.
	//
	// stateLock must be held when accessing this field.
	closed bool

	// lastLogEntry is a pointer to the last LogEntry that was exported.
	//
	// stateLock must be held when accessing this field.
	lastLogEntry *logpb.LogEntry

	// appendErr is the error that should be returned by Append. It is set when
	// stream content processing hits a fatal state.
	appendErr error

	// testAppendWaitCallback, if not nil, is called before Append blocks.
	// This callback is used for testing coordination.
	testAppendWaitCallback func()
}

func newStream(c streamConfig) *streamImpl {
	return &streamImpl{
		c: &c,

		dataConsumedSignalC: make(chan struct{}, 1),
	}
}

func (s *streamImpl) LeaseData() Data {
	return globalDataPoolRegistry.getPool(dataBufferSize).getData()
}

func (s *streamImpl) Append(d Data) error {
	defer func() {
		if d != nil {
			d.Release()
		}
	}()

	// Block/loop until we've successfully appended the data.
	for {
		dLen := int64(len(d.Bytes()))
		if err := s.appendError(); err != nil || dLen == 0 {
			return err
		}

		s.withParserLock(func() error {
			if s.c.parser.bufferedBytes() == 0 ||
				s.c.parser.bufferedBytes()+dLen <= s.c.maximumBufferedBytes {
				s.c.parser.appendData(d)
				d = nil
			}
			return nil
		})

		// The data was appended; we're done.
		if s.c.onAppend != nil {
			s.c.onAppend(d == nil)
		}
		if d == nil {
			break
		}

		// Not ready to append; wait for a data event and re-evaluate.
		<-s.dataConsumedSignalC
	}

	return nil
}

// Signals our Append loop that data has been consumed.
func (s *streamImpl) signalDataConsumed() {
	select {
	case s.dataConsumedSignalC <- struct{}{}:
		break

	default:
		break
	}
}

func (s *streamImpl) Close() {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()
	s.closeLocked()
}

func (s *streamImpl) closeLocked() {
	s.closed = true
}

func (s *streamImpl) name() string {
	return s.c.name
}

// isDrained returns true if this stream is finished emitting data, including
// its terminal state.
//
// This can happen if either:
// - The stream is closed and has no more buffered data, or
// - The strema has encountered a fatal error during processing.
func (s *streamImpl) isDrained() bool {
	return atomic.LoadInt32(&s.drained) != 0
}

// setDrained marks this stream as drained.
func (s *streamImpl) setDrained() {
	atomic.StoreInt32(&s.drained, 1)
}

// noMoreDataLocked returns true if our stream has been closed and its buffer
// is empty.
//
// The stream's stateLock must be held when calling this method.
func (s *streamImpl) noMoreDataLocked() bool {
	if !s.closed {
		return false
	}

	// If we have an append error, we will no longer accept or consume data.
	if s.appendErr != nil {
		return true
	}

	var bufSize int64
	s.withParserLock(func() error {
		bufSize = s.c.parser.bufferedBytes()
		return nil
	})
	return bufSize == 0
}

// expireTime returns the Time when the oldest chunk in the stream will expire.
//
// This is calculated ask:
// oldest.Timestamp + stream.maximumBufferDuration
// If there is no buffered data, oldest will return nil.
func (s *streamImpl) expireTime() (t time.Time, has bool) {
	s.withParserLock(func() error {
		t, has = s.c.parser.firstChunkTime()
		return nil
	})

	if has {
		t = t.Add(s.c.maximumBufferDuration)
	}
	return
}

// nextBundleEntry generates bundles for this stream. The total bundle data size
// must not exceed the supplied size.
//
// If no bundle entry could be generated given the constraints, nil will be
// returned.
//
// It is possible for some entries to be returned alongside an error.
func (s *streamImpl) nextBundleEntry(bb *builder, aggressive bool) bool {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()

	// If we're not drained, try and get the next bundle.
	modified := false
	if !s.noMoreDataLocked() {
		err := error(nil)
		modified, err = s.nextBundleEntryLocked(bb, aggressive)
		if err != nil {
			s.setAppendErrorLocked(err)
		}

		if modified {
			s.signalDataConsumed()
		}
	}

	// If we're drained, populate our terminal state.
	if s.noMoreDataLocked() {
		if s.lastLogEntry != nil {
			bb.setStreamTerminal(&s.c.template, s.lastLogEntry.StreamIndex)
		}
		s.setDrained()
	}

	return modified
}

func (s *streamImpl) nextBundleEntryLocked(bb *builder, aggressive bool) (bool, error) {
	c := constraints{
		allowSplit: aggressive,
		closed:     s.closed,
	}

	// Extract as many entries as possible from the stream. As we extract, adjust
	// our byte size.
	//
	// If we're closed, this will continue to consume until finished. If an error
	// occurs, shut down data collection.
	modified := false

	for c.limit = bb.remaining(); c.limit > 0; c.limit = bb.remaining() {
		emittedLog := false
		err := s.withParserLock(func() error {
			le, err := s.c.parser.nextEntry(&c)
			if err != nil {
				return err
			}

			if le == nil {
				return nil
			}

			// Enforce basic log entry consistency.
			if err := s.fixupLogEntry(s.lastLogEntry, le); err != nil {
				return err
			}

			emittedLog = true
			modified = true

			bb.add(&s.c.template, le)
			s.lastLogEntry = le
			return nil
		})

		if err != nil || !emittedLog {
			return modified, err
		}
	}
	return modified, nil
}

// fixupLogEntry asserts and corrects a log entry's stream offset and ordering
// given the previous entry in the stream.
//
// If prev is nil, that means that cur is expected to be the first log entry
// in the stream.
func (s *streamImpl) fixupLogEntry(prev, cur *logpb.LogEntry) error {
	if prev == nil {
		if cur.StreamIndex != 0 {
			return fmt.Errorf("first log entry is not zero index (%d)", cur.StreamIndex)
		}
	} else {
		if cur.StreamIndex != prev.StreamIndex+1 {
			return fmt.Errorf("non-contiguous stream indices (%d != %d)", cur.StreamIndex, prev.StreamIndex+1)
		}

		if cur.TimeOffset.Duration() < prev.TimeOffset.Duration() {
			to := *prev.TimeOffset
			cur.TimeOffset = &to
		}
	}

	return nil
}

func (s *streamImpl) withParserLock(f func() error) error {
	s.parserLock.Lock()
	defer s.parserLock.Unlock()

	return f()
}

func (s *streamImpl) appendError() error {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()

	return s.appendErr
}

func (s *streamImpl) setAppendErrorLocked(err error) {
	s.appendErr = err

	s.closeLocked()
	s.signalDataConsumed()
}
