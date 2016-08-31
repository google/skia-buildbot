// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logs

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/luci/gae/filter/featureBreaker"
	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/logs/v1"
	"github.com/luci/luci-go/logdog/api/logpb"
	ct "github.com/luci/luci-go/logdog/appengine/coordinator/coordinatorTest"
	"github.com/luci/luci-go/logdog/common/types"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func shouldHaveLogPaths(actual interface{}, expected ...interface{}) string {
	resp := actual.(*logdog.QueryResponse)
	var paths []string
	if len(resp.Streams) > 0 {
		paths = make([]string, len(resp.Streams))
		for i, s := range resp.Streams {
			paths[i] = s.Path
		}
	}

	var exp []string
	for _, e := range expected {
		switch t := e.(type) {
		case []string:
			exp = append(exp, t...)

		case string:
			exp = append(exp, t)

		case types.StreamPath:
			exp = append(exp, string(t))

		default:
			panic(fmt.Errorf("unsupported expected type %T: %v", t, t))
		}
	}

	return ShouldResemble(paths, exp)
}

func TestQuery(t *testing.T) {
	t.Parallel()

	Convey(`With a testing configuration, a Query request`, t, func() {
		c, env := ct.Install()
		c, fb := featureBreaker.FilterRDS(c, nil)

		ds.Get(c).Testable().Consistent(true)

		var svrBase server
		svr := newService(&svrBase)

		const project = config.ProjectName("proj-foo")

		// Stock query request, will be modified by each test.
		req := logdog.QueryRequest{
			Project: string(project),
			Tags:    map[string]string{},
		}

		// Install a set of stock log streams to query against.
		var streamPaths []string
		var purgedStreamPaths []string
		streams := map[string]*ct.TestStream{}
		for i, v := range []types.StreamPath{
			"testing/+/foo",
			"testing/+/foo/bar",
			"other/+/foo/bar",
			"other/+/baz",

			"meta/terminated/+/foo",
			"meta/archived/+/foo",
			"meta/purged/+/foo",
			"meta/terminated/archived/purged/+/foo",
			"meta/datagram/+/foo",
			"meta/binary/+/foo",

			"testing/+/foo/bar/baz",
			"testing/+/baz",
		} {
			tls := ct.MakeStream(c, project, v)
			tls.Desc.ContentType = tls.Stream.Prefix
			tls.Desc.Tags = map[string]string{
				"prefix": tls.Stream.Prefix,
				"name":   tls.Stream.Name,
			}

			// Set an empty tag for each name segment.
			for _, p := range types.StreamName(tls.Stream.Name).Segments() {
				tls.Desc.Tags[p] = ""
			}

			now := env.Clock.Now().UTC()
			psegs := types.StreamName(tls.Stream.Prefix).Segments()
			if psegs[0] == "meta" {
				for _, p := range psegs[1:] {
					switch p {
					case "purged":
						tls.Stream.Purged = true

					case "archived":
						tls.State.ArchiveStreamURL = "http://example.com"
						tls.State.ArchivedTime = now
						So(tls.State.ArchivalState().Archived(), ShouldBeTrue)
						fallthrough // Archived streams are also terminated.

					case "terminated":
						tls.State.TerminalIndex = 1337
						tls.State.TerminatedTime = now
						So(tls.State.Terminated(), ShouldBeTrue)

					case "datagram":
						tls.Desc.StreamType = logpb.StreamType_DATAGRAM

					case "binary":
						tls.Desc.StreamType = logpb.StreamType_BINARY
					}
				}
			}

			tls.Reload(c)
			if err := tls.Put(c); err != nil {
				panic(fmt.Errorf("failed to put log stream %d: %v", i, err))
			}

			streams[string(v)] = tls
			if !tls.Stream.Purged {
				streamPaths = append(streamPaths, string(v))
			}
			purgedStreamPaths = append(purgedStreamPaths, string(v))
			env.Clock.Add(time.Second)
		}
		ds.Get(c).Testable().CatchupIndexes()

		// Invert streamPaths since we will return results in descending Created
		// order.
		invert := func(s []string) {
			for i := 0; i < len(s)/2; i++ {
				eidx := len(s) - i - 1
				s[i], s[eidx] = s[eidx], s[i]
			}
		}
		invert(streamPaths)
		invert(purgedStreamPaths)

		Convey(`An empty query will return all log streams.`, func() {
			resp, err := svr.Query(c, &req)
			So(err, ShouldBeRPCOK)
			So(resp, shouldHaveLogPaths, streamPaths)
		})

		Convey(`An empty query to a non-existent project fails with NotFound.`, func() {
			req.Project = "does-not-exist"

			_, err := svr.Query(c, &req)
			So(err, ShouldBeRPCNotFound)
		})

		Convey(`An empty query to a project without access fails with NotFound.`, func() {
			req.Project = "proj-exclusive"

			_, err := svr.Query(c, &req)
			So(err, ShouldBeRPCNotFound)
		})

		Convey(`An empty query will include purged streams if admin.`, func() {
			env.JoinGroup("admin")

			resp, err := svr.Query(c, &req)
			So(err, ShouldBeRPCOK)
			So(resp, shouldHaveLogPaths, purgedStreamPaths)
		})

		Convey(`A query with an invalid path will return BadRequest error.`, func() {
			req.Path = "***"

			_, err := svr.Query(c, &req)
			So(err, ShouldBeRPCInvalidArgument, "invalid query `path`")
		})

		Convey(`A query with an invalid Next cursor will return BadRequest error.`, func() {
			req.Next = "invalid"
			fb.BreakFeatures(errors.New("testing error"), "DecodeCursor")

			_, err := svr.Query(c, &req)
			So(err, ShouldBeRPCInvalidArgument, "invalid `next` value")
		})

		Convey(`A datastore query error will return InternalServer error.`, func() {
			fb.BreakFeatures(errors.New("testing error"), "Run")

			_, err := svr.Query(c, &req)
			So(err, ShouldBeRPCInternal)
		})

		Convey(`When querying for "testing/+/baz"`, func() {
			req.Path = "testing/+/baz"

			tls := streams["testing/+/baz"]
			Convey(`State is not returned.`, func() {
				resp, err := svr.Query(c, &req)
				So(err, ShouldBeRPCOK)
				So(resp, shouldHaveLogPaths, "testing/+/baz")

				So(resp.Streams, ShouldHaveLength, 1)
				So(resp.Streams[0].State, ShouldBeNil)
				So(resp.Streams[0].Desc, ShouldBeNil)
				So(resp.Streams[0].DescProto, ShouldBeNil)
			})

			Convey(`When requesting state`, func() {
				req.State = true

				Convey(`When not requesting protobufs, returns a descriptor structure.`, func() {
					resp, err := svr.Query(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogPaths, "testing/+/baz")

					So(resp.Streams, ShouldHaveLength, 1)
					So(resp.Streams[0].State, ShouldResemble, buildLogStreamState(tls.Stream, tls.State))
					So(resp.Streams[0].Desc, ShouldResemble, tls.Desc)
					So(resp.Streams[0].DescProto, ShouldBeNil)
				})

				Convey(`When not requesting protobufs, and with a corrupt descriptor, returns InternalServer error.`, func() {
					tls.Stream.SetDSValidate(false)
					tls.Stream.Descriptor = []byte{0x00} // Invalid protobuf, zero tag.
					if err := tls.Put(c); err != nil {
						panic(err)
					}
					ds.Get(c).Testable().CatchupIndexes()

					_, err := svr.Query(c, &req)
					So(err, ShouldBeRPCInternal)
				})

				Convey(`When requesting protobufs, returns the raw protobuf descriptor.`, func() {
					req.Proto = true

					resp, err := svr.Query(c, &req)
					So(err, ShouldBeNil)
					So(resp, shouldHaveLogPaths, "testing/+/baz")

					So(resp.Streams, ShouldHaveLength, 1)
					So(resp.Streams[0].State, ShouldResemble, buildLogStreamState(tls.Stream, tls.State))
					So(resp.Streams[0].Desc, ShouldResemble, tls.Desc)
				})
			})
		})

		Convey(`With a query limit of 3`, func() {
			svrBase.resultLimit = 3

			Convey(`Can iteratively query to retrieve all stream paths.`, func() {
				var seen []string

				next := ""
				for {
					req.Next = next

					resp, err := svr.Query(c, &req)
					So(err, ShouldBeRPCOK)

					for _, svr := range resp.Streams {
						seen = append(seen, svr.Path)
					}

					next = resp.Next
					if next == "" {
						break
					}
				}

				sort.Strings(seen)
				sort.Strings(streamPaths)
				So(seen, ShouldResemble, streamPaths)
			})
		})

		Convey(`When querying against timestamp constraints`, func() {
			req.Older = google.NewTimestamp(env.Clock.Now())

			// Invert our streamPaths, since we're going to receive results
			// newest-to-oldest and this makes it much easier to check/express.
			for i := 0; i < len(streamPaths)/2; i++ {
				eidx := len(streamPaths) - i - 1
				streamPaths[i], streamPaths[eidx] = streamPaths[eidx], streamPaths[i]
			}

			Convey(`Querying for entries created at or after 2 seconds ago (latest 2 entries).`, func() {
				req.Newer = google.NewTimestamp(env.Clock.Now().Add(-2*time.Second - time.Millisecond))

				resp, err := svr.Query(c, &req)
				So(err, ShouldBeRPCOK)
				So(resp, shouldHaveLogPaths, "testing/+/baz", "testing/+/foo/bar/baz")
			})

			Convey(`With a query limit of 3`, func() {
				svrBase.resultLimit = 3

				Convey(`A query request will return the newest 3 entries and have a Next cursor for the next 3.`, func() {
					resp, err := svr.Query(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogPaths, "testing/+/baz", "testing/+/foo/bar/baz", "meta/binary/+/foo")
					So(resp.Next, ShouldNotEqual, "")

					// Iterate.
					req.Next = resp.Next

					resp, err = svr.Query(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogPaths, "meta/datagram/+/foo", "meta/archived/+/foo", "meta/terminated/+/foo")
				})

				Convey(`A datastore query error will return InternalServer error.`, func() {
					fb.BreakFeatures(errors.New("testing error"), "Run")

					_, err := svr.Query(c, &req)
					So(err, ShouldBeRPCInternal)
				})
			})
		})

		Convey(`When querying for meta streams`, func() {
			req.Path = "meta/**/+/**"

			Convey(`When purged=yes, returns BadRequest error.`, func() {
				req.Purged = logdog.QueryRequest_YES

				_, err := svr.Query(c, &req)
				So(err, ShouldBeRPCInvalidArgument, "non-admin user cannot request purged log streams")
			})

			Convey(`When the user is an administrator`, func() {
				env.JoinGroup("admin")

				Convey(`When purged=yes, returns [terminated/archived/purged, purged]`, func() {
					req.Purged = logdog.QueryRequest_YES

					resp, err := svr.Query(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogPaths, "meta/terminated/archived/purged/+/foo", "meta/purged/+/foo")
				})

				Convey(`When purged=no, returns [binary, datagram, archived, terminated]`, func() {
					req.Purged = logdog.QueryRequest_NO

					resp, err := svr.Query(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogPaths, "meta/binary/+/foo", "meta/datagram/+/foo", "meta/archived/+/foo", "meta/terminated/+/foo")
				})
			})

			Convey(`When querying for text streams, returns [archived, terminated]`, func() {
				req.StreamType = &logdog.QueryRequest_StreamTypeFilter{Value: logpb.StreamType_TEXT}

				resp, err := svr.Query(c, &req)
				So(err, ShouldBeRPCOK)
				So(resp, shouldHaveLogPaths, "meta/archived/+/foo", "meta/terminated/+/foo")
			})

			Convey(`When querying for binary streams, returns [binary]`, func() {
				req.StreamType = &logdog.QueryRequest_StreamTypeFilter{Value: logpb.StreamType_BINARY}

				resp, err := svr.Query(c, &req)
				So(err, ShouldBeRPCOK)
				So(resp, shouldHaveLogPaths, "meta/binary/+/foo")
			})

			Convey(`When querying for datagram streams, returns [datagram]`, func() {
				req.StreamType = &logdog.QueryRequest_StreamTypeFilter{Value: logpb.StreamType_DATAGRAM}

				resp, err := svr.Query(c, &req)
				So(err, ShouldBeRPCOK)
				So(resp, shouldHaveLogPaths, "meta/datagram/+/foo")
			})

			Convey(`When querying for an invalid stream type, returns a BadRequest error.`, func() {
				req.StreamType = &logdog.QueryRequest_StreamTypeFilter{Value: -1}

				_, err := svr.Query(c, &req)
				So(err, ShouldBeRPCInvalidArgument)
			})
		})

		Convey(`When querying for content type "other", returns [other/+/baz, other/+/foo/bar].`, func() {
			req.ContentType = "other"

			resp, err := svr.Query(c, &req)
			So(err, ShouldBeRPCOK)
			So(resp, shouldHaveLogPaths, "other/+/baz", "other/+/foo/bar")
		})

		Convey(`When querying for proto version, the current version returns all non-purged streams.`, func() {
			req.ProtoVersion = logpb.Version

			resp, err := svr.Query(c, &req)
			So(err, ShouldBeRPCOK)
			So(resp, shouldHaveLogPaths, streamPaths)
		})

		Convey(`When querying for proto version, "invalid" returns nothing.`, func() {
			req.ProtoVersion = "invalid"

			resp, err := svr.Query(c, &req)
			So(err, ShouldBeRPCOK)
			So(resp, shouldHaveLogPaths)
		})

		Convey(`When querying for tags`, func() {
			Convey(`Tag "baz", returns [testing/+/baz, testing/+/foo/bar/baz, other/+/baz]`, func() {
				req.Tags["baz"] = ""

				resp, err := svr.Query(c, &req)
				So(err, ShouldBeRPCOK)
				So(resp, shouldHaveLogPaths, "testing/+/baz", "testing/+/foo/bar/baz", "other/+/baz")
			})

			Convey(`Tags "prefix=testing", "baz", returns [testing/+/baz, testing/+/foo/bar/baz]`, func() {
				req.Tags["baz"] = ""
				req.Tags["prefix"] = "testing"

				resp, err := svr.Query(c, &req)
				So(err, ShouldBeRPCOK)
				So(resp, shouldHaveLogPaths, "testing/+/baz", "testing/+/foo/bar/baz")
			})

			Convey(`When an invalid tag is specified, returns BadRequest error`, func() {
				req.Tags["+++not a valid tag+++"] = ""

				_, err := svr.Query(c, &req)
				So(err, ShouldBeRPCInvalidArgument, "invalid tag constraint")
			})
		})
	})
}
