// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cloudlogging

import (
	"time"

	"github.com/luci/luci-go/common/retry"
	"github.com/luci/luci-go/common/sync/parallel"
	"golang.org/x/net/context"
)

const (
	// DefaultBatchSize is the number of log messages that will be sent in a
	// single cloud logging publish request.
	DefaultBatchSize = 1000

	// DefaultParallelPushLimit is the maximum number of goroutines that may
	// simultaneously push cloud logging data.
	DefaultParallelPushLimit = 8
)

// Buffer is a goroutine-safe intermediate buffer that implements Client. Any
// logs that are written to the Buffer are batched together before being sent
// to Cloud Logging.
type Buffer interface {
	Client

	// StopAndFlush flushes the Buffer, blocking until all buffered data has been
	// transmitted. After closing, log writes will be discarded.
	StopAndFlush()

	// Abort stops any current attempts to send messages. It is goroutine-safe and
	// can be called multiple times.
	//
	// If StopAndFlush is blocking on cloud logging send/retry, calling Abort will
	// quickly terminate the attempts, causing StopAndFlush to complete.
	Abort()
}

// BufferOptions specifies configuration parameters for an instantiated Buffer
// instance.
type BufferOptions struct {
	// BatchSize is the number of messages to batch together when uploading to
	// Cloud Logging endpoint. If zero, DefaultBatchSize will be used.
	BatchSize int

	// ParallelPushLimit is the maximum number of PushEntries calls that will be
	// executed at the same time. If zero, DefaultParallelPushLimit will be used.
	ParallelPushLimit int

	// Retry is a generator. If not nil, it will be used to produce a retry
	// Iterator that will be used to retry the PushEntries call to the wrapped
	// Client.
	Retry func() retry.Iterator
}

// bufferImpl is the default implementation of the Buffer interface.
type bufferImpl struct {
	*BufferOptions

	ctx        context.Context    // The context to use for operations.
	cancelFunc context.CancelFunc // The context's cancel function.

	client       Client // The Cloud Logging Client to push through.
	commonLabels map[string]string

	logC      chan *Entry
	finishedC chan struct{}

	testCB *testCallbacks
}

// (Testing) a set of callbacks that can be installed by testing for
// fine-grained control.
type testCallbacks struct {
	// bufferRound is called when the buffer's inner loop has completed a buffer
	// round.
	bufferRound func([]*Entry)
	// receivedLogEntry, if not nil, is called for each LogEntry that is received
	// by the buffer's inner loop the moment that it is received.
	receivedLogEntry func(*Entry) // (Testing) If not nil, callback when a log Entry is received.
}

// NewBuffer instantiates and starts a new cloud logging Buffer.
// implementation.
func NewBuffer(ctx context.Context, o BufferOptions, c Client) Buffer {
	if o.BatchSize <= 0 {
		o.BatchSize = DefaultBatchSize
	}

	if o.ParallelPushLimit <= 0 {
		o.ParallelPushLimit = DefaultParallelPushLimit
	}

	ctx, cancelFunc := context.WithCancel(ctx)
	b := &bufferImpl{
		BufferOptions: &o,

		ctx:        ctx,
		cancelFunc: cancelFunc,
		client:     c,

		// Use a >1 multiple of BatchSize so casual logging doesn't immediately
		// block pending buffer flush.
		logC:      make(chan *Entry, o.BatchSize*4),
		finishedC: make(chan struct{}),
	}

	go b.process()
	return b
}

func (b *bufferImpl) PushEntries(e []*Entry) error {
	for _, entry := range e {
		b.logC <- entry
	}
	return nil
}

func (b *bufferImpl) StopAndFlush() {
	close(b.logC)
	<-b.finishedC
}

func (b *bufferImpl) Abort() {
	b.cancelFunc()
}

// process is run in a separate goroutine to pull log entries and publish them
// to cloud logging.
func (b *bufferImpl) process() {
	defer close(b.finishedC)

	// Create a push semaphore channel; fill with push tokens.
	pushSemC := make(parallel.Semaphore, b.ParallelPushLimit)

	entries := make([]*Entry, b.BatchSize)
	for e := range b.logC {
		// Pull up to our entry capacity.
		entries[0] = e
		count := 1
		b.receivedLogEntry(e)

		// Buffer other logs that are also available in the channel.
		for count < len(entries) {
			moreE := (*Entry)(nil)
			select {
			case moreE = <-b.logC:
				b.receivedLogEntry(moreE)
				break
			default:
				break
			}
			if moreE == nil {
				break
			}
			entries[count] = moreE
			count++
		}

		// Clone entries so we can dispatch to goroutine.
		entryCopy := make([]*Entry, count)
		copy(entryCopy, entries[:count])

		// (Testing) ACK any log entries that were received, for synchronization.
		b.bufferRound(entryCopy)

		// Acquire a push channel semaphore token.
		pushSemC.Lock()
		go func() {
			defer pushSemC.Unlock()
			b.publishLogs(entryCopy)
		}()
	}

	// Acquire all of our push channel semaphore tokens (block until goroutines
	// are done).
	pushSemC.TakeAll()
}

// publishLogs writes a slice of log Entry to the wrapped Client. The underlying
// PushEntries call will be retried.
func (b *bufferImpl) publishLogs(entries []*Entry) {
	// If we are aborted, Retry will detect this and abort.
	err := retry.Retry(b.ctx, b.newRetryIterator, func() error {
		return b.client.PushEntries(entries)
	}, func(err error, delay time.Duration) {
		b.writeError("cloudlogging: Failed to push entries, retrying in %v: %v", delay, err)
	})
	if err != nil {
		b.writeError("cloudlogging: Failed to push entries: %s", err)
	}
}

func (b *bufferImpl) bufferRound(entries []*Entry) {
	if b.testCB != nil && b.testCB.bufferRound != nil {
		b.testCB.bufferRound(entries)
	}
}

func (b *bufferImpl) receivedLogEntry(e *Entry) {
	if b.testCB != nil && b.testCB.receivedLogEntry != nil {
		b.testCB.receivedLogEntry(e)
	}
}

// newRetryIterator generates a new retry iterator. If configured, the iterator
// will be generated by the Retry method; otherwise, a nil retry iterator (no
// retries) will be returned.
func (b *bufferImpl) newRetryIterator() retry.Iterator {
	if b.Retry == nil {
		return nil
	}
	return b.Retry()
}

func (b *bufferImpl) writeError(f string, args ...interface{}) {
	if c, ok := b.client.(*clientImpl); ok {
		c.writeError(f, args...)
	}
}
