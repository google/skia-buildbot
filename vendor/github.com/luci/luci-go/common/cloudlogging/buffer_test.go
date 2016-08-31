// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cloudlogging

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/luci/luci-go/common/retry"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

type testClient struct {
	callback func([]*Entry) error
	pushes   int
}

var _ Client = (*testClient)(nil)

func (c *testClient) PushEntries(entries []*Entry) error {
	c.pushes++
	if c.callback != nil {
		return c.callback(entries)
	}
	return nil
}

type infiniteRetryIterator struct{}

func (infiniteRetryIterator) Next(context.Context, error) time.Duration {
	return 0
}

func TestBuffer(t *testing.T) {
	t.Parallel()

	Convey(`A Buffer instance`, t, func() {
		ctx := context.Background()

		entriesC := make(chan []*Entry, 1)
		client := &testClient{
			callback: func(entries []*Entry) error {
				entriesC <- entries
				return nil
			},
		}

		options := BufferOptions{
			Retry: func() retry.Iterator {
				return &retry.Limited{
					Retries: 5,
				}
			},
		}

		b := NewBuffer(ctx, options, client).(*bufferImpl)
		defer b.StopAndFlush()

		// Allow synchronization when a log entry is ingested. Set "ackC" to nil to
		// disable synchronization.
		var bufferC chan []*Entry
		var ackC chan *Entry
		b.testCB = &testCallbacks{
			bufferRound: func(e []*Entry) {
				if bufferC != nil {
					bufferC <- e
				}
			},
			receivedLogEntry: func(e *Entry) {
				if ackC != nil {
					ackC <- e
				}
			},
		}

		So(b.BatchSize, ShouldEqual, DefaultBatchSize)

		Convey(`Will push a single entry.`, func() {
			err := b.PushEntries([]*Entry{
				{
					InsertID: "a",
				},
			})
			So(err, ShouldBeNil)

			entries := <-entriesC
			So(len(entries), ShouldEqual, 1)
			So(entries[0], ShouldResemble, &Entry{
				InsertID: "a",
			})
			So(client.pushes, ShouldEqual, 1)
		})

		Convey(`Will batch logging data.`, func() {
			bufferC = make(chan []*Entry)
			ackC = make(chan *Entry)

			// The first message will be read immediately.
			err := b.PushEntries([]*Entry{
				{
					InsertID: "a",
				},
			})
			So(err, ShouldBeNil)
			<-ackC
			<-bufferC

			// The next set of messages will be batched, since we haven't allowed our
			// client stub to finish its PushEntries.
			entries := make([]*Entry, b.BatchSize)
			for i := range entries {
				entries[i] = &Entry{
					InsertID: fmt.Sprintf("%d", i),
				}
			}
			err = b.PushEntries(entries)
			So(err, ShouldBeNil)

			// Read the first bundle.
			bundle := <-entriesC
			So(len(bundle), ShouldEqual, 1)
			So(bundle[0].InsertID, ShouldEqual, "a")

			// Read the second bundle.
			for range entries {
				<-ackC
			}
			<-bufferC
			bundle = <-entriesC

			So(len(bundle), ShouldEqual, b.BatchSize)
			for i := range bundle {
				So(bundle[i].InsertID, ShouldEqual, fmt.Sprintf("%d", i))
			}
			So(client.pushes, ShouldEqual, 2)
		})

		Convey(`Will retry on failure.`, func() {
			failures := 3
			client.callback = func(entries []*Entry) error {
				if failures > 0 {
					failures--
					return errors.New("test: induced failure")
				}
				entriesC <- entries
				return nil
			}

			err := b.PushEntries([]*Entry{
				{
					InsertID: "a",
				},
			})
			So(err, ShouldBeNil)

			entries := <-entriesC
			So(len(entries), ShouldEqual, 1)
			So(entries[0], ShouldResemble, &Entry{
				InsertID: "a",
			})
			So(client.pushes, ShouldEqual, 4)
		})
	})

	Convey(`A Buffer instance configured to retry forever will stop if aborted.`, t, func() {
		entriesC := make(chan []*Entry)
		defer close(entriesC)

		client := &testClient{
			callback: func(entries []*Entry) error {
				entriesC <- entries
				return errors.New("test: failure")
			},
		}

		options := BufferOptions{
			Retry: func() retry.Iterator {
				return infiniteRetryIterator{}
			},
		}

		b := NewBuffer(context.Background(), options, client)
		err := b.PushEntries([]*Entry{
			{
				InsertID: "a",
			},
		})
		So(err, ShouldBeNil)

		// Wait for the buffer to finish.
		finishedC := make(chan struct{})
		go func() {
			defer close(finishedC)
			b.StopAndFlush()
		}()

		// Wait for the first attempt. This ensures that we've entered the retry
		// loop.
		<-entriesC
		go func() {
			// Consume any other attempts.
			for range entriesC {
			}
		}()

		// Abort the buffer.
		b.Abort()

		// Assert that it will stop eventually. Rather than deadlock/panic, we wait
		// one real second and fail if it didn't terminate. Since there is no
		// underlying latency, one second (in the failure case) is acceptable.
		closed := false
		select {
		case <-finishedC:
			closed = true

		case <-time.After(1 * time.Second):
			break
		}
		So(closed, ShouldBeTrue)
	})
}
