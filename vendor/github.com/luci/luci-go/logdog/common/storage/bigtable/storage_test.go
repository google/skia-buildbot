// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bigtable

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/luci/gkvlite"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/data/recordio"
	"github.com/luci/luci-go/logdog/common/storage"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

// btTableTest is an in-memory implementation of btTable interface for testing.
//
// This is a simple implementation; not an efficient one.
type btTableTest struct {
	s *gkvlite.Store
	c *gkvlite.Collection

	// err, if true, is the error immediately returned by functions.
	err error

	// maxLogAge is the currently-configured maximum log age.
	maxLogAge time.Duration
}

func (t *btTableTest) close() {
	if t.s != nil {
		t.s.Close()
		t.s = nil
	}
}

func (t *btTableTest) collection() *gkvlite.Collection {
	if t.s == nil {
		var err error
		t.s, err = gkvlite.NewStore(nil)
		if err != nil {
			panic(err)
		}
		t.c = t.s.MakePrivateCollection(bytes.Compare)
	}
	return t.c
}

func (t *btTableTest) putLogData(c context.Context, rk *rowKey, d []byte) error {
	if t.err != nil {
		return t.err
	}

	// Record/count sanity check.
	records, err := recordio.Split(d)
	if err != nil {
		return err
	}
	if int64(len(records)) != rk.count {
		return fmt.Errorf("count mismatch (%d != %d)", len(records), rk.count)
	}

	enc := []byte(rk.encode())
	coll := t.collection()
	if item, _ := coll.Get(enc); item != nil {
		return storage.ErrExists
	}

	clone := make([]byte, len(d))
	copy(clone, d)
	if err := coll.Set(enc, clone); err != nil {
		panic(err)
	}
	return nil
}

func (t *btTableTest) getLogData(c context.Context, rk *rowKey, limit int, keysOnly bool, cb btGetCallback) error {
	if t.err != nil {
		return t.err
	}

	enc := []byte(rk.encode())
	prefix := rk.pathPrefix()
	var ierr error
	err := t.collection().VisitItemsAscend(enc, !keysOnly, func(i *gkvlite.Item) bool {
		var drk *rowKey
		drk, ierr = decodeRowKey(string(i.Key))
		if ierr != nil {
			return false
		}
		if drk.pathPrefix() != prefix {
			return false
		}

		rowData := i.Val
		if keysOnly {
			rowData = nil
		}

		if ierr = cb(drk, rowData); ierr != nil {
			if ierr == errStop {
				ierr = nil
			}
			return false
		}

		if limit > 0 {
			limit--
			if limit == 0 {
				return false
			}
		}

		return true
	})
	if err != nil {
		panic(err)
	}
	return ierr
}

func (t *btTableTest) setMaxLogAge(c context.Context, d time.Duration) error {
	if t.err != nil {
		return t.err
	}
	t.maxLogAge = d
	return nil
}

func (t *btTableTest) dataMap() map[string][]byte {
	result := map[string][]byte{}

	err := t.collection().VisitItemsAscend([]byte(nil), true, func(i *gkvlite.Item) bool {
		result[string(i.Key)] = i.Val
		return true
	})
	if err != nil {
		panic(err)
	}
	return result
}

func TestStorage(t *testing.T) {
	t.Parallel()

	Convey(`A BigTable storage instance bound to a testing BigTable instance`, t, func() {
		bt := btTableTest{}
		defer bt.close()

		s := newBTStorage(context.Background(), Options{
			Project:  "test-project",
			Instance: "test-instance",
			LogTable: "test-log-table",
		}, nil, nil)

		s.raw = &bt
		defer s.Close()

		project := config.ProjectName("test-project")
		get := func(path string, index int, limit int, keysOnly bool) ([]string, error) {
			req := storage.GetRequest{
				Project:  project,
				Path:     types.StreamPath(path),
				Index:    types.MessageIndex(index),
				Limit:    limit,
				KeysOnly: keysOnly,
			}
			got := []string{}
			err := s.Get(req, func(idx types.MessageIndex, d []byte) bool {
				if keysOnly {
					got = append(got, strconv.Itoa(int(idx)))
				} else {
					got = append(got, string(d))
				}
				return true
			})
			return got, err
		}

		put := func(path string, index int, d ...string) error {
			data := make([][]byte, len(d))
			for i, v := range d {
				data[i] = []byte(v)
			}

			return s.Put(storage.PutRequest{
				Project: project,
				Path:    types.StreamPath(path),
				Index:   types.MessageIndex(index),
				Values:  data,
			})
		}

		ekey := func(path string, v, c int64) string {
			return newRowKey(string(project), path, v, c).encode()
		}
		records := func(s ...string) []byte {
			buf := bytes.Buffer{}
			w := recordio.NewWriter(&buf)

			for _, v := range s {
				if _, err := w.Write([]byte(v)); err != nil {
					panic(err)
				}
				if err := w.Flush(); err != nil {
					panic(err)
				}
			}

			return buf.Bytes()
		}

		Convey(`With an artificial maximum BigTable row size of two records`, func() {
			// Artificially constrain row size. 4 = 2*{size/1, data/1} RecordIO
			// entries.
			s.maxRowSize = 4

			Convey(`Will split row data that overflows the table into multiple rows.`, func() {
				So(put("A", 0, "0", "1", "2", "3"), ShouldBeNil)

				So(bt.dataMap(), ShouldResemble, map[string][]byte{
					ekey("A", 1, 2): records("0", "1"),
					ekey("A", 3, 2): records("2", "3"),
				})
			})

			Convey(`Loading a single row data beyond the maximum row size will fail.`, func() {
				So(put("A", 0, "0123"), ShouldErrLike, "single row entry exceeds maximum size")
			})
		})

		Convey(`With row data: A{0, 1, 2, 3, 4}, B{10, 12, 13}`, func() {
			So(put("A", 0, "0", "1", "2"), ShouldBeNil)
			So(put("A", 3, "3", "4"), ShouldBeNil)
			So(put("B", 10, "10"), ShouldBeNil)
			So(put("B", 12, "12", "13"), ShouldBeNil)

			Convey(`Testing "Put"...`, func() {
				Convey(`Loads the row data.`, func() {
					So(bt.dataMap(), ShouldResemble, map[string][]byte{
						ekey("A", 2, 3):  records("0", "1", "2"),
						ekey("A", 4, 2):  records("3", "4"),
						ekey("B", 10, 1): records("10"),
						ekey("B", 13, 2): records("12", "13"),
					})
				})
			})

			Convey(`Testing "Get"...`, func() {
				Convey(`Can fetch the full row, "A".`, func() {
					got, err := get("A", 0, 0, false)
					So(err, ShouldBeNil)
					So(got, ShouldResemble, []string{"0", "1", "2", "3", "4"})
				})

				Convey(`Will fetch A{1, 2, 3, 4} with index=1.`, func() {
					got, err := get("A", 1, 0, false)
					So(err, ShouldBeNil)
					So(got, ShouldResemble, []string{"1", "2", "3", "4"})
				})

				Convey(`Will fetch A{1, 2} with index=1 and limit=2.`, func() {
					got, err := get("A", 1, 2, false)
					So(err, ShouldBeNil)
					So(got, ShouldResemble, []string{"1", "2"})
				})

				Convey(`Will fetch B{10, 12, 13} for B.`, func() {
					got, err := get("B", 0, 0, false)
					So(err, ShouldBeNil)
					So(got, ShouldResemble, []string{"10", "12", "13"})
				})

				Convey(`Will fetch B{12, 13} when index=11.`, func() {
					got, err := get("B", 11, 0, false)
					So(err, ShouldBeNil)
					So(got, ShouldResemble, []string{"12", "13"})
				})

				Convey(`Will fetch {} for INVALID.`, func() {
					got, err := get("INVALID", 0, 0, false)
					So(err, ShouldBeNil)
					So(got, ShouldResemble, []string{})
				})
			})

			Convey(`Testing "Get" (keys only)`, func() {
				Convey(`Can fetch the full row, "A".`, func() {
					got, err := get("A", 0, 0, true)
					So(err, ShouldBeNil)
					So(got, ShouldResemble, []string{"0", "1", "2", "3", "4"})
				})

				Convey(`Will fetch A{1, 2, 3, 4} with index=1.`, func() {
					got, err := get("A", 1, 0, true)
					So(err, ShouldBeNil)
					So(got, ShouldResemble, []string{"1", "2", "3", "4"})
				})

				Convey(`Will fetch A{1, 2} with index=1 and limit=2.`, func() {
					got, err := get("A", 1, 2, true)
					So(err, ShouldBeNil)
					So(got, ShouldResemble, []string{"1", "2"})
				})

				Convey(`Will fetch B{10, 12, 13} for B.`, func() {
					got, err := get("B", 0, 0, true)
					So(err, ShouldBeNil)
					So(got, ShouldResemble, []string{"10", "12", "13"})
				})

				Convey(`Will fetch B{12, 13} when index=11.`, func() {
					got, err := get("B", 11, 0, true)
					So(err, ShouldBeNil)
					So(got, ShouldResemble, []string{"12", "13"})
				})
			})

			Convey(`Testing "Tail"...`, func() {
				tail := func(path string) (string, error) {
					got, _, err := s.Tail(project, types.StreamPath(path))
					return string(got), err
				}

				Convey(`A tail request for "A" returns A{4}.`, func() {
					got, err := tail("A")
					So(err, ShouldBeNil)
					So(got, ShouldEqual, "4")
				})

				Convey(`A tail request for "B" returns B{13}.`, func() {
					got, err := tail("B")
					So(err, ShouldBeNil)
					So(got, ShouldEqual, "13")
				})

				Convey(`A tail request for "INVALID" errors NOT FOUND.`, func() {
					_, err := tail("INVALID")
					So(err, ShouldEqual, storage.ErrDoesNotExist)
				})
			})
		})
	})
}
