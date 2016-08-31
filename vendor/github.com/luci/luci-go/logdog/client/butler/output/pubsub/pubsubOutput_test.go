// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package pubsub

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/data/recordio"
	gcps "github.com/luci/luci-go/common/gcloud/pubsub"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/logpb"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

type testTopic struct {
	sync.Mutex

	err error

	msgC          chan *pubsub.Message
	nextMessageID int
}

func (t *testTopic) Name() string { return "test" }

func (t *testTopic) Publish(c context.Context, msgs ...*pubsub.Message) ([]string, error) {
	if t.err != nil {
		return nil, t.err
	}

	ids := make([]string, len(msgs))
	for i, m := range msgs {
		t.msgC <- m
		ids[i] = t.getNextMessageID()
	}
	return ids, nil
}

func (t *testTopic) getNextMessageID() string {
	t.Lock()
	defer t.Unlock()

	id := t.nextMessageID
	t.nextMessageID++
	return fmt.Sprintf("%d", id)
}

func deconstructMessage(msg *pubsub.Message) (*logpb.ButlerMetadata, *logpb.ButlerLogBundle, error) {
	fr := recordio.NewReader(bytes.NewBuffer(msg.Data), gcps.MaxPublishSize)

	// Validate header frame.
	headerBytes, err := fr.ReadFrameAll()
	if err != nil {
		return nil, nil, fmt.Errorf("test: failed to read header frame: %s", err)
	}

	header := logpb.ButlerMetadata{}
	if err := proto.Unmarshal(headerBytes, &header); err != nil {
		return nil, nil, fmt.Errorf("test: failed to unmarshal header: %s", err)
	}

	if header.Type != logpb.ButlerMetadata_ButlerLogBundle {
		return nil, nil, fmt.Errorf("test: unknown frame data type: %v", header.Type)
	}

	// Validate data frame.
	data, err := fr.ReadFrameAll()
	if err != nil {
		return nil, nil, fmt.Errorf("test: failed to read data frame: %s", err)
	}

	switch header.Compression {
	case logpb.ButlerMetadata_ZLIB:
		r, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, nil, fmt.Errorf("test: failed to create zlib reader: %s", err)
		}
		defer r.Close()

		data, err = ioutil.ReadAll(r)
		if err != nil {
			return nil, nil, fmt.Errorf("test: failed to read compressed data: %s", err)
		}
	}

	dataBundle := logpb.ButlerLogBundle{}
	if err := proto.Unmarshal(data, &dataBundle); err != nil {
		return nil, nil, fmt.Errorf("test: failed to unmarshal bundle: %s", err)
	}

	return &header, &dataBundle, nil
}

func TestOutput(t *testing.T) {
	Convey(`An Output using a test Pub/Sub instance`, t, func() {
		ctx, _ := testclock.UseTime(context.Background(), time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC))
		tt := &testTopic{
			msgC: make(chan *pubsub.Message, 1),
		}
		conf := Config{
			Topic: tt,
		}
		o := New(ctx, conf).(*pubSubOutput)
		So(o, ShouldNotBeNil)
		defer o.Close()

		bundle := &logpb.ButlerLogBundle{
			Timestamp: google.NewTimestamp(clock.Now(ctx)),
			Entries: []*logpb.ButlerLogBundle_Entry{
				{},
			},
		}

		Convey(`Can send/receive a bundle.`, func() {
			So(o.SendBundle(bundle), ShouldBeNil)

			h, b, err := deconstructMessage(<-tt.msgC)
			So(err, ShouldBeNil)
			So(h.Compression, ShouldEqual, logpb.ButlerMetadata_NONE)
			So(b, ShouldResemble, bundle)

			Convey(`And records stats.`, func() {
				st := o.Stats()
				So(st.Errors(), ShouldEqual, 0)
				So(st.SentBytes(), ShouldBeGreaterThan, 0)
				So(st.SentMessages(), ShouldEqual, 1)
				So(st.DiscardedMessages(), ShouldEqual, 0)
			})
		})

		Convey(`Will return an error if Publish failed.`, func() {
			tt.err = errors.New("test: error")
			So(o.SendBundle(bundle), ShouldNotBeNil)
		})
	})
}
