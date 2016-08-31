// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cloudlog

import (
	"testing"
	"time"

	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/cloudlogging"
	"github.com/luci/luci-go/common/logging"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

type testClient struct {
	logC chan []*cloudlogging.Entry
	err  error
}

func (c *testClient) PushEntries(entries []*cloudlogging.Entry) error {
	c.logC <- entries
	return c.err
}

func TestCloudLogging(t *testing.T) {
	Convey(`A cloud logging instance using a test client`, t, func() {
		now := time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)
		ctx, _ := testclock.UseTime(context.Background(), now)

		client := &testClient{
			logC: make(chan []*cloudlogging.Entry, 1),
		}

		config := Config{
			InsertIDBase: "totally-random",
		}

		ctx = Use(ctx, config, client)

		Convey(`Can publish logging data.`, func() {
			logging.Fields{
				"foo": "bar",
			}.Infof(ctx, "Message at %s", "INFO")

			entries := <-client.logC
			So(len(entries), ShouldEqual, 1)
			So(entries[0], ShouldResemble, &cloudlogging.Entry{
				InsertID:  "totally-random.0.0",
				Timestamp: now,
				Severity:  cloudlogging.Info,
				Labels: cloudlogging.Labels{
					"foo": "bar",
				},
				TextPayload: `Message at INFO {"foo":"bar"}`,
			})
		})
	})
}
