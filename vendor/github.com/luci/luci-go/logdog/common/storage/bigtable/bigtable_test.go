// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bigtable

import (
	"testing"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/logdog/common/storage"
	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBigTable(t *testing.T) {
	t.Parallel()

	Convey(`Testing BigTable internal functions`, t, func() {
		var bt btTableTest
		defer bt.close()

		s := newBTStorage(context.Background(), Options{
			Project:  "test-project",
			Instance: "test-instance",
			LogTable: "test-log-table",
		}, nil, nil)

		s.raw = &bt
		defer s.Close()

		Convey(`Given a fake BigTable row`, func() {
			fakeRow := bigtable.Row{
				"log": []bigtable.ReadItem{
					{
						Row:    "testrow",
						Column: logColName,
						Value:  []byte("here is my data"),
					},
				},
			}

			Convey(`Can extract log data.`, func() {
				d, err := getLogRowData(fakeRow)
				So(err, ShouldBeNil)
				So(d, ShouldResemble, []byte("here is my data"))
			})

			Convey(`Will fail to extract if the column is missing.`, func() {
				fakeRow["log"][0].Column = "not-data"

				_, err := getLogRowData(fakeRow)
				So(err, ShouldEqual, storage.ErrDoesNotExist)
			})

			Convey(`Will fail to extract if the family does not exist.`, func() {
				So(getReadItem(fakeRow, "invalid", "invalid"), ShouldBeNil)
			})

			Convey(`Will fail to extract if the column does not exist.`, func() {
				So(getReadItem(fakeRow, "log", "invalid"), ShouldBeNil)
			})
		})

		Convey(`When pushing a configuration`, func() {
			cfg := storage.Config{
				MaxLogAge: 1 * time.Hour,
			}

			Convey(`Can successfully apply configuration.`, func() {
				So(s.Config(cfg), ShouldBeNil)
				So(bt.maxLogAge, ShouldEqual, cfg.MaxLogAge)
			})

			Convey(`With return an error if the configuration fails to apply.`, func() {
				bt.err = errors.New("test error")

				So(s.Config(cfg), ShouldEqual, bt.err)
			})
		})
	})
}
