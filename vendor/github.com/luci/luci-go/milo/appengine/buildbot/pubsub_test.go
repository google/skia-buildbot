// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package buildbot

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/server/router"
	//log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/gologger"
	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

var (
	fakeTime = time.Date(2001, time.February, 3, 4, 5, 6, 7, time.UTC)
)

func buildbotTimesFinished(start, end float64) []*float64 {
	return []*float64{&start, &end}
}

func buildbotTimesPending(start float64) []*float64 {
	return []*float64{&start, nil}
}

func newCombinedPsBody(bs []buildbotBuild, m *buildbotMaster, internal bool) io.ReadCloser {
	bmsg := buildMasterMsg{
		Master: m,
		Builds: bs,
	}
	bm, _ := json.Marshal(bmsg)
	var b bytes.Buffer
	zw := zlib.NewWriter(&b)
	zw.Write(bm)
	zw.Close()
	sub := "projects/luci-milo/subscriptions/buildbot-public"
	if internal {
		sub = "projects/luci-milo/subscriptions/buildbot-private"
	}
	msg := pubSubSubscription{
		Subscription: sub,
		Message: pubSubMessage{
			Data: base64.StdEncoding.EncodeToString(b.Bytes()),
		},
	}
	jmsg, _ := json.Marshal(msg)
	return ioutil.NopCloser(bytes.NewReader(jmsg))
}

func TestPubSub(t *testing.T) {
	Convey(`A test Environment`, t, func() {
		c := memory.Use(context.Background())
		c = gologger.StdConfig.Use(c)
		c, _ = testclock.UseTime(c, fakeTime)
		ds := datastore.Get(c)

		Convey("Save build entry", func() {
			build := &buildbotBuild{
				Master:      "Fake Master",
				Buildername: "Fake buildername",
				Number:      1234,
				Currentstep: "this is a string",
				Finished:    true,
			}
			err := ds.Put(build)
			ds.Testable().CatchupIndexes()

			So(err, ShouldBeNil)
			Convey("Load build entry", func() {
				loadB := &buildbotBuild{
					Master:      "Fake Master",
					Buildername: "Fake buildername",
					Number:      1234,
				}
				err = ds.Get(loadB)
				So(err, ShouldBeNil)
				So(loadB.Master, ShouldEqual, "Fake Master")
				So(loadB.Internal, ShouldEqual, false)
				So(loadB.Currentstep.(string), ShouldEqual, "this is a string")
				So(loadB.Finished, ShouldEqual, true)
			})

			Convey("Query build entry", func() {
				q := datastore.NewQuery("buildbotBuild")
				buildbots := []*buildbotBuild{}
				err = ds.GetAll(q, &buildbots)
				So(err, ShouldBeNil)

				So(len(buildbots), ShouldEqual, 1)
				So(buildbots[0].Currentstep.(string), ShouldEqual, "this is a string")
				Convey("Query for finished entries should be 1", func() {
					q = q.Eq("finished", true)
					buildbots = []*buildbotBuild{}
					err = ds.GetAll(q, &buildbots)
					So(err, ShouldBeNil)
					So(len(buildbots), ShouldEqual, 1)
				})
				Convey("Query for unfinished entries should be 0", func() {
					q = q.Eq("finished", false)
					buildbots = []*buildbotBuild{}
					err = ds.GetAll(q, &buildbots)
					So(err, ShouldBeNil)
					So(len(buildbots), ShouldEqual, 0)
				})
			})

			Convey("Save a few more entries", func() {
				ds.Testable().Consistent(true)
				ds.Testable().AutoIndex(true)
				for i := 1235; i < 1240; i++ {
					build := &buildbotBuild{
						Master:      "Fake Master",
						Buildername: "Fake buildername",
						Number:      i,
						Currentstep: "this is a string",
						Finished:    i%2 == 0,
					}
					err := ds.Put(build)
					So(err, ShouldBeNil)
				}
				q := datastore.NewQuery("buildbotBuild")
				q = q.Eq("finished", true)
				q = q.Eq("master", "Fake Master")
				q = q.Eq("builder", "Fake buildername")
				q = q.Order("-number")
				buildbots := []*buildbotBuild{}
				err = ds.GetAll(q, &buildbots)
				So(err, ShouldBeNil)
				So(len(buildbots), ShouldEqual, 3) // 1235, 1237, 1239
			})
		})

		Convey("Build panics on invalid query", func() {
			build := &buildbotBuild{
				Master: "Fake Master",
			}
			So(func() { ds.Put(build) }, ShouldPanicLike, "No Master or Builder found")
		})

		b := buildbotBuild{
			Master:      "Fake Master",
			Buildername: "Fake buildername",
			Number:      1234,
			Currentstep: "this is a string",
			Times:       buildbotTimesPending(123.0),
		}

		Convey("Basic master + build pusbsub subscription", func() {
			h := httptest.NewRecorder()
			slaves := map[string]*buildbotSlave{}
			ft := 1234.0
			slaves["testslave"] = &buildbotSlave{
				Name:      "testslave",
				Connected: true,
				Runningbuilds: []buildbotBuild{
					{
						Master:      "Fake Master",
						Buildername: "Fake buildername",
						Number:      2222,
						Times:       []*float64{&ft, nil},
					},
				},
			}
			ms := buildbotMaster{
				Name:    "fakename",
				Project: buildbotProject{Title: "some title"},
				Slaves:  slaves,
			}
			r := &http.Request{
				Body: newCombinedPsBody([]buildbotBuild{b}, &ms, false),
			}
			p := httprouter.Params{}
			PubSubHandler(&router.Context{
				Context: c,
				Writer:  h,
				Request: r,
				Params:  p,
			})
			So(h.Code, ShouldEqual, 200)
			Convey("And stores correctly", func() {
				loadB := &buildbotBuild{
					Master:      "Fake Master",
					Buildername: "Fake buildername",
					Number:      1234,
				}
				err := ds.Get(loadB)
				So(err, ShouldBeNil)
				So(loadB.Master, ShouldEqual, "Fake Master")
				So(loadB.Currentstep.(string), ShouldEqual, "this is a string")
				m, internal, t, err := getMasterJSON(c, "fakename")
				So(err, ShouldBeNil)
				So(internal, ShouldEqual, false)
				So(t.Unix(), ShouldEqual, 981173106)
				So(m.Name, ShouldEqual, "fakename")
				So(m.Project.Title, ShouldEqual, "some title")
				So(m.Slaves["testslave"].Name, ShouldEqual, "testslave")
				So(len(m.Slaves["testslave"].Runningbuilds), ShouldEqual, 0)
				So(len(m.Slaves["testslave"].RunningbuildsMap), ShouldEqual, 1)
				So(m.Slaves["testslave"].RunningbuildsMap["Fake buildername"][0],
					ShouldEqual, 2222)
			})

			Convey("And a new master overwrites", func() {
				c, _ = testclock.UseTime(c, fakeTime.Add(time.Duration(1*time.Second)))
				ms.Project.Title = "some other title"
				h = httptest.NewRecorder()
				r := &http.Request{
					Body: newCombinedPsBody([]buildbotBuild{b}, &ms, false)}
				p = httprouter.Params{}
				PubSubHandler(&router.Context{
					Context: c,
					Writer:  h,
					Request: r,
					Params:  p,
				})
				So(h.Code, ShouldEqual, 200)
				m, internal, t, err := getMasterJSON(c, "fakename")
				So(err, ShouldBeNil)
				So(internal, ShouldEqual, false)
				So(m.Project.Title, ShouldEqual, "some other title")
				So(t.Unix(), ShouldEqual, 981173107)
				So(m.Name, ShouldEqual, "fakename")
			})
			Convey("And a new build overwrites", func() {
				b.Times = buildbotTimesFinished(123.0, 124.0)
				h = httptest.NewRecorder()
				r = &http.Request{
					Body: newCombinedPsBody([]buildbotBuild{b}, &ms, false),
				}
				p = httprouter.Params{}
				PubSubHandler(&router.Context{
					Context: c,
					Writer:  h,
					Request: r,
					Params:  p,
				})
				So(h.Code, ShouldEqual, 200)
				loadB := &buildbotBuild{
					Master:      "Fake Master",
					Buildername: "Fake buildername",
					Number:      1234,
				}
				err := ds.Get(loadB)
				So(err, ShouldBeNil)
				So(*loadB.Times[0], ShouldEqual, 123.0)
				So(*loadB.Times[1], ShouldEqual, 124.0)
				Convey("And another pending build is rejected", func() {
					b.Times = buildbotTimesPending(123.0)
					h = httptest.NewRecorder()
					r = &http.Request{
						Body: newCombinedPsBody([]buildbotBuild{b}, &ms, false),
					}
					p = httprouter.Params{}
					PubSubHandler(&router.Context{
						Context: c,
						Writer:  h,
						Request: r,
						Params:  p,
					})
					So(h.Code, ShouldEqual, 200)
					loadB := &buildbotBuild{
						Master:      "Fake Master",
						Buildername: "Fake buildername",
						Number:      1234,
					}
					err := ds.Get(loadB)
					So(err, ShouldBeNil)
					So(*loadB.Times[0], ShouldEqual, 123.0)
					So(*loadB.Times[1], ShouldEqual, 124.0)
				})
			})
		})

		Convey("Empty pubsub message", func() {
			h := httptest.NewRecorder()
			r := &http.Request{Body: ioutil.NopCloser(bytes.NewReader([]byte{}))}
			p := httprouter.Params{}
			PubSubHandler(&router.Context{
				Context: c,
				Writer:  h,
				Request: r,
				Params:  p,
			})
			So(h.Code, ShouldEqual, 200)
		})

		Convey("Internal master + build pusbsub subscription", func() {
			h := httptest.NewRecorder()
			slaves := map[string]*buildbotSlave{}
			ft := 1234.0
			slaves["testslave"] = &buildbotSlave{
				Name:      "testslave",
				Connected: true,
				Runningbuilds: []buildbotBuild{
					{
						Master:      "Fake Master",
						Buildername: "Fake buildername",
						Number:      2222,
						Times:       []*float64{&ft, nil},
					},
				},
			}
			ms := buildbotMaster{
				Name:    "fakename",
				Project: buildbotProject{Title: "some title"},
				Slaves:  slaves,
			}
			r := &http.Request{
				Body: newCombinedPsBody([]buildbotBuild{b}, &ms, true),
			}
			p := httprouter.Params{}
			PubSubHandler(&router.Context{
				Context: c,
				Writer:  h,
				Request: r,
				Params:  p,
			})
			So(h.Code, ShouldEqual, 200)
			Convey("And stores correctly", func() {
				loadB := &buildbotBuild{
					Master:      "Fake Master",
					Buildername: "Fake buildername",
					Number:      1234,
				}
				err := ds.Get(loadB)
				So(err, ShouldBeNil)
				So(loadB.Master, ShouldEqual, "Fake Master")
				So(loadB.Internal, ShouldEqual, true)
				So(loadB.Currentstep.(string), ShouldEqual, "this is a string")
				m, internal, t, err := getMasterJSON(c, "fakename")
				So(err, ShouldBeNil)
				So(internal, ShouldEqual, true)
				So(t.Unix(), ShouldEqual, 981173106)
				So(m.Name, ShouldEqual, "fakename")
				So(m.Project.Title, ShouldEqual, "some title")
				So(m.Slaves["testslave"].Name, ShouldEqual, "testslave")
				So(len(m.Slaves["testslave"].Runningbuilds), ShouldEqual, 0)
				So(len(m.Slaves["testslave"].RunningbuildsMap), ShouldEqual, 1)
				So(m.Slaves["testslave"].RunningbuildsMap["Fake buildername"][0],
					ShouldEqual, 2222)
			})
		})
	})
}
