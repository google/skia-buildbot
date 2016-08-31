// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package memory

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/logdog/common/storage"
	"github.com/luci/luci-go/logdog/common/types"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func numRec(v types.MessageIndex) *rec {
	buf := bytes.Buffer{}
	binary.Write(&buf, binary.BigEndian, v)
	return &rec{
		index: v,
		data:  buf.Bytes(),
	}
}

// index builds an index of stream message number to array offset.
func index(recs []*rec) map[types.MessageIndex]int {
	index := map[types.MessageIndex]int{}
	for i, r := range recs {
		index[r.index] = i
	}
	return index
}

func TestBigTable(t *testing.T) {
	t.Parallel()

	Convey(`A memory Storage instance.`, t, func() {
		st := Storage{}
		defer st.Close()

		project := config.ProjectName("test-project")
		path := types.StreamPath("testing/+/foo/bar")

		Convey(`Can Put() log stream records {0..5, 7, 8, 10}.`, func() {
			var indices []types.MessageIndex

			putRange := func(start types.MessageIndex, count int) error {
				req := storage.PutRequest{
					Project: project,
					Path:    path,
					Index:   start,
				}
				for i := 0; i < count; i++ {
					index := start + types.MessageIndex(i)
					req.Values = append(req.Values, numRec(index).data)
					indices = append(indices, index)
				}
				return st.Put(req)
			}

			So(putRange(0, 6), ShouldBeNil)
			So(putRange(7, 2), ShouldBeNil)
			So(putRange(10, 1), ShouldBeNil)

			// Forward-indexed records.
			recs := make([]*rec, len(indices))
			for i, idx := range indices {
				recs[i] = numRec(idx)
			}

			var getRecs []*rec
			getAllCB := func(idx types.MessageIndex, data []byte) bool {
				getRecs = append(getRecs, &rec{
					index: idx,
					data:  data,
				})
				return true
			}

			Convey(`Put()`, func() {
				req := storage.PutRequest{
					Project: project,
					Path:    path,
				}

				Convey(`Will return ErrExists when putting an existing entry.`, func() {
					req.Values = [][]byte{[]byte("ohai")}

					So(st.Put(req), ShouldEqual, storage.ErrExists)
				})

				Convey(`Will return an error if one is set.`, func() {
					st.SetErr(errors.New("test error"))

					req.Index = 1337
					So(st.Put(req), ShouldErrLike, "test error")
				})
			})

			Convey(`Get()`, func() {
				req := storage.GetRequest{
					Project: project,
					Path:    path,
				}

				Convey(`Can retrieve all of the records correctly.`, func() {
					So(st.Get(req, getAllCB), ShouldBeNil)
					So(getRecs, ShouldResemble, recs)
				})

				Convey(`Will adhere to GetRequest limit.`, func() {
					req.Limit = 4

					So(st.Get(req, getAllCB), ShouldBeNil)
					So(getRecs, ShouldResemble, recs[:4])
				})

				Convey(`Will adhere to hard limit.`, func() {
					st.MaxGetCount = 3
					req.Limit = 4

					So(st.Get(req, getAllCB), ShouldBeNil)
					So(getRecs, ShouldResemble, recs[:3])
				})

				Convey(`Will stop iterating if callback returns false.`, func() {
					count := 0
					err := st.Get(req, func(types.MessageIndex, []byte) bool {
						count++
						return false
					})
					So(err, ShouldBeNil)
					So(count, ShouldEqual, 1)
				})

				Convey(`Will fail to retrieve records if the project doesn't exist.`, func() {
					req.Project = "project-does-not-exist"

					So(st.Get(req, getAllCB), ShouldEqual, storage.ErrDoesNotExist)
				})

				Convey(`Will fail to retrieve records if the path doesn't exist.`, func() {
					req.Path = "testing/+/does/not/exist"

					So(st.Get(req, getAllCB), ShouldEqual, storage.ErrDoesNotExist)
				})

				Convey(`Will return an error if one is set.`, func() {
					st.SetErr(errors.New("test error"))

					So(st.Get(req, nil), ShouldErrLike, "test error")
				})
			})

			Convey(`Tail()`, func() {
				Convey(`Can retrieve the tail record, 10.`, func() {
					d, idx, err := st.Tail(project, path)
					So(err, ShouldBeNil)
					So(d, ShouldResemble, numRec(10).data)
					So(idx, ShouldEqual, 10)
				})

				Convey(`Will fail to retrieve records if the project doesn't exist.`, func() {
					_, _, err := st.Tail("project-does-not-exist", path)
					So(err, ShouldEqual, storage.ErrDoesNotExist)
				})

				Convey(`Will fail to retrieve records if the path doesn't exist.`, func() {
					_, _, err := st.Tail(project, "testing/+/does/not/exist")
					So(err, ShouldEqual, storage.ErrDoesNotExist)
				})

				Convey(`Will return an error if one is set.`, func() {
					st.SetErr(errors.New("test error"))
					_, _, err := st.Tail("", "")
					So(err, ShouldErrLike, "test error")
				})
			})

			Convey(`Config()`, func() {
				cfg := storage.Config{
					MaxLogAge: time.Hour,
				}

				Convey(`Can update the configuration.`, func() {
					So(st.Config(cfg), ShouldBeNil)
					So(st.MaxLogAge, ShouldEqual, cfg.MaxLogAge)
				})

				Convey(`Will return an error if one is set.`, func() {
					st.SetErr(errors.New("test error"))
					So(st.Config(storage.Config{}), ShouldErrLike, "test error")
				})
			})

			Convey(`Errors can be set, cleared, and set again.`, func() {
				So(st.Config(storage.Config{}), ShouldBeNil)

				st.SetErr(errors.New("test error"))
				So(st.Config(storage.Config{}), ShouldErrLike, "test error")

				st.SetErr(nil)
				So(st.Config(storage.Config{}), ShouldBeNil)

				st.SetErr(errors.New("test error"))
				So(st.Config(storage.Config{}), ShouldErrLike, "test error")
			})
		})
	})
}
