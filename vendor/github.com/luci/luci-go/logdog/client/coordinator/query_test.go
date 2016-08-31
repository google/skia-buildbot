// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"errors"
	"fmt"
	"testing"

	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/common/testing/prpctest"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/logs/v1"
	"github.com/luci/luci-go/logdog/api/logpb"
	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

type testQueryLogsService struct {
	testLogsServiceBase

	LR logdog.QueryRequest
	H  func(*logdog.QueryRequest) (*logdog.QueryResponse, error)
}

func (s *testQueryLogsService) Query(c context.Context, req *logdog.QueryRequest) (*logdog.QueryResponse, error) {
	s.LR = *req
	if h := s.H; h != nil {
		return s.H(req)
	}
	return nil, errors.New("not implemented")
}

func gen(name string, state *logdog.LogStreamState) *logdog.QueryResponse_Stream {
	return &logdog.QueryResponse_Stream{
		Path:  fmt.Sprintf("test/+/%s", name),
		State: state,
		Desc: &logpb.LogStreamDescriptor{
			Prefix: "test",
			Name:   name,
		},
	}
}

func shouldHaveLogStreams(actual interface{}, expected ...interface{}) string {
	a := actual.([]*LogStream)

	aList := make([]string, len(a))
	for i, ls := range a {
		aList[i] = string(ls.Path)
	}

	eList := make([]string, len(expected))
	for i, exp := range expected {
		eList[i] = exp.(string)
	}

	return ShouldResemble(aList, eList)
}

func TestClientQuery(t *testing.T) {
	t.Parallel()

	Convey(`A testing Client`, t, func() {
		now := testclock.TestTimeLocal
		c := context.Background()

		ts := prpctest.Server{}
		svc := testQueryLogsService{}
		logdog.RegisterLogsServer(&ts, &svc)

		// Create a testing server and client.
		ts.Start(c)
		defer ts.Close()

		prpcClient, err := ts.NewClient()
		if err != nil {
			panic(err)
		}
		client := Client{
			C: logdog.NewLogsPRPCClient(prpcClient),
		}

		Convey(`When making a query request`, func() {
			const project = config.ProjectName("myproj")
			const path = "**/+/**"
			q := QueryOptions{
				Tags: map[string]string{
					"foo": "bar",
					"baz": "qux",
				},
				ContentType: "application/text",
				Before:      now,
				After:       now,
				Purged:      Both,
				State:       true,
			}

			var results []*LogStream
			accumulate := func(s *LogStream) bool {
				results = append(results, s)
				return true
			}

			Convey(`Can accumulate results across queries.`, func() {
				// This handler will return a single query per request, as well as a
				// non-empty Next pointer for the next query element. It progresses
				// "a" => "b" => "final" => "".
				svc.H = func(req *logdog.QueryRequest) (*logdog.QueryResponse, error) {
					r := logdog.QueryResponse{
						Project: string(project),
					}
					switch req.Next {
					case "":
						r.Streams = append(r.Streams, gen("a", nil))
						r.Next = "b"
					case "b":
						r.Streams = append(r.Streams, gen("b", nil))
						r.Next = "final"
					case "final":
						r.Streams = append(r.Streams, gen("final", nil))
					default:
						return nil, errors.New("invalid cursor")
					}
					return &r, nil
				}

				So(client.Query(c, project, path, q, accumulate), ShouldBeNil)
				So(results, shouldHaveLogStreams, "test/+/a", "test/+/b", "test/+/final")
			})

			Convey(`Will stop invoking the callback if it returns false.`, func() {
				// This handler will return three query results, "a", "b", and "c".
				svc.H = func(*logdog.QueryRequest) (*logdog.QueryResponse, error) {
					return &logdog.QueryResponse{
						Streams: []*logdog.QueryResponse_Stream{
							gen("a", nil),
							gen("b", nil),
							gen("c", nil),
						},
						Next: "infiniteloop",
					}, nil
				}

				accumulate = func(s *LogStream) bool {
					results = append(results, s)
					return len(results) < 3
				}
				So(client.Query(c, project, path, q, accumulate), ShouldBeNil)
				So(results, shouldHaveLogStreams, "test/+/a", "test/+/b", "test/+/c")
			})

			Convey(`Will properly handle state and protobuf deserialization.`, func() {
				svc.H = func(*logdog.QueryRequest) (*logdog.QueryResponse, error) {
					return &logdog.QueryResponse{
						Streams: []*logdog.QueryResponse_Stream{
							gen("a", &logdog.LogStreamState{
								Created: google.NewTimestamp(now),
							}),
						},
					}, nil
				}

				So(client.Query(c, project, path, q, accumulate), ShouldBeNil)
				So(results, shouldHaveLogStreams, "test/+/a")
				So(results[0], ShouldResemble, &LogStream{
					Path: "test/+/a",
					Desc: &logpb.LogStreamDescriptor{Prefix: "test", Name: "a"},
					State: &StreamState{
						Created: now.UTC(),
					},
				})
			})

			Convey(`Can query for stream types`, func() {
				svc.H = func(*logdog.QueryRequest) (*logdog.QueryResponse, error) {
					return &logdog.QueryResponse{}, nil
				}

				Convey(`Text`, func() {
					q.StreamType = Text
					So(client.Query(c, project, path, q, accumulate), ShouldBeNil)
					So(svc.LR.StreamType, ShouldResemble, &logdog.QueryRequest_StreamTypeFilter{Value: logpb.StreamType_TEXT})
				})

				Convey(`Binary`, func() {
					q.StreamType = Binary
					So(client.Query(c, project, path, q, accumulate), ShouldBeNil)
					So(svc.LR.StreamType, ShouldResemble, &logdog.QueryRequest_StreamTypeFilter{Value: logpb.StreamType_BINARY})
				})

				Convey(`Datagram`, func() {
					q.StreamType = Datagram
					So(client.Query(c, project, path, q, accumulate), ShouldBeNil)
					So(svc.LR.StreamType, ShouldResemble, &logdog.QueryRequest_StreamTypeFilter{Value: logpb.StreamType_DATAGRAM})
				})
			})

			Convey(`Will return ErrNoAccess if unauthenticated.`, func() {
				svc.H = func(*logdog.QueryRequest) (*logdog.QueryResponse, error) {
					return nil, grpcutil.Unauthenticated
				}

				So(client.Query(c, project, path, q, accumulate), ShouldEqual, ErrNoAccess)
			})

			Convey(`Will return ErrNoAccess if permission denied.`, func() {
				svc.H = func(*logdog.QueryRequest) (*logdog.QueryResponse, error) {
					return nil, grpcutil.Unauthenticated
				}

				So(client.Query(c, project, path, q, accumulate), ShouldEqual, ErrNoAccess)
			})
		})
	})
}
