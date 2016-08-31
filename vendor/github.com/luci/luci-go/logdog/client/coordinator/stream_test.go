// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"errors"
	"testing"

	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/common/testing/prpctest"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/logs/v1"
	"github.com/luci/luci-go/logdog/api/logpb"
	"golang.org/x/net/context"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func genLog(idx int64, id string) *logpb.LogEntry {
	return &logpb.LogEntry{
		StreamIndex: uint64(idx),
		Content: &logpb.LogEntry_Text{
			Text: &logpb.Text{
				Lines: []*logpb.Text_Line{
					{Value: id},
				},
			},
		},
	}
}

// testStreamLogsService implements just the Get and Tail endpoints,
// instrumented for testing.
type testStreamLogsService struct {
	testLogsServiceBase

	// Get
	GR logdog.GetRequest
	GH func(*logdog.GetRequest) (*logdog.GetResponse, error)

	// Tail
	TR logdog.TailRequest
	TH func(*logdog.TailRequest) (*logdog.GetResponse, error)
}

func (s *testStreamLogsService) Get(c context.Context, req *logdog.GetRequest) (*logdog.GetResponse, error) {
	s.GR = *req
	if h := s.GH; h != nil {
		return s.GH(req)
	}
	return nil, errors.New("not implemented")
}

func (s *testStreamLogsService) Tail(c context.Context, req *logdog.TailRequest) (*logdog.GetResponse, error) {
	s.TR = *req
	if h := s.TH; h != nil {
		return s.TH(req)
	}
	return nil, errors.New("not implemented")
}

func TestStreamGet(t *testing.T) {
	t.Parallel()

	Convey(`A testing Client`, t, func() {
		c := context.Background()
		now := testclock.TestTimeUTC

		ts := prpctest.Server{}
		svc := testStreamLogsService{}
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

		Convey(`Can bind a Stream`, func() {
			s := client.Stream("myproj", "test/+/a")

			Convey(`Test Get`, func() {
				p := NewGetParams()

				Convey(`A default Get query will return logs and no state.`, func() {
					svc.GH = func(*logdog.GetRequest) (*logdog.GetResponse, error) {
						return &logdog.GetResponse{
							Logs: []*logpb.LogEntry{
								genLog(1337, "ohai"),
								genLog(1338, "kthxbye"),
							},
						}, nil
					}

					l, err := s.Get(c, nil)
					So(err, ShouldBeNil)
					So(l, ShouldResemble, []*logpb.LogEntry{genLog(1337, "ohai"), genLog(1338, "kthxbye")})

					// Validate the correct parameters were sent.
					So(svc.GR, ShouldResemble, logdog.GetRequest{
						Project: "myproj",
						Path:    "test/+/a",
					})
				})

				Convey(`Will form a proper Get logs query.`, func() {
					p = p.NonContiguous().Index(1)

					svc.GH = func(*logdog.GetRequest) (*logdog.GetResponse, error) {
						return &logdog.GetResponse{}, nil
					}

					l, err := s.Get(c, p)
					So(err, ShouldBeNil)
					So(l, ShouldBeNil)

					// Validate the correct parameters were sent.
					So(svc.GR, ShouldResemble, logdog.GetRequest{
						Project:       "myproj",
						Path:          "test/+/a",
						NonContiguous: true,
						Index:         1,
					})
				})

				Convey(`Will request a specific number of logs if a constraint is supplied.`, func() {
					p = p.Limit(32, 64)

					svc.GH = func(*logdog.GetRequest) (*logdog.GetResponse, error) {
						return &logdog.GetResponse{
							Logs: []*logpb.LogEntry{
								genLog(1337, "ohai"),
							},
						}, nil
					}

					l, err := s.Get(c, p)
					So(err, ShouldBeNil)
					So(l, ShouldResemble, []*logpb.LogEntry{genLog(1337, "ohai")})

					// Validate the HTTP request that we made.
					So(svc.GR, ShouldResemble, logdog.GetRequest{
						Project:   "myproj",
						Path:      "test/+/a",
						LogCount:  64,
						ByteCount: 32,
					})
				})

				Convey(`Can decode a full protobuf and state.`, func() {
					var ls LogStream
					p = p.State(&ls)

					svc.GH = func(*logdog.GetRequest) (*logdog.GetResponse, error) {
						return &logdog.GetResponse{
							Logs: []*logpb.LogEntry{
								genLog(1337, "kthxbye"),
							},
							State: &logdog.LogStreamState{
								Created: google.NewTimestamp(now),
								Archive: &logdog.LogStreamState_ArchiveInfo{
									IndexUrl:  "index",
									StreamUrl: "stream",
									DataUrl:   "data",
								},
							},
							Desc: &logpb.LogStreamDescriptor{
								Prefix:     "test",
								Name:       "a",
								StreamType: logpb.StreamType_TEXT,
							},
						}, nil
					}

					l, err := s.Get(c, p)
					So(err, ShouldBeNil)
					So(l, ShouldResemble, []*logpb.LogEntry{genLog(1337, "kthxbye")})
					So(ls, ShouldResemble, LogStream{
						Path: "test/+/a",
						Desc: &logpb.LogStreamDescriptor{
							Prefix:     "test",
							Name:       "a",
							StreamType: logpb.StreamType_TEXT,
						},
						State: &StreamState{
							Created:          now,
							Archived:         true,
							ArchiveIndexURL:  "index",
							ArchiveStreamURL: "stream",
							ArchiveDataURL:   "data",
						},
					})
				})

				Convey(`Will return ErrNoSuchStream if the stream is not found.`, func() {
					svc.GH = func(*logdog.GetRequest) (*logdog.GetResponse, error) {
						return nil, grpcutil.NotFound
					}

					_, err := s.Get(c, p)
					So(err, ShouldEqual, ErrNoSuchStream)
				})

				Convey(`Will return ErrNoAccess if unauthenticated.`, func() {
					svc.GH = func(*logdog.GetRequest) (*logdog.GetResponse, error) {
						return nil, grpcutil.Unauthenticated
					}

					_, err := s.Get(c, p)
					So(err, ShouldEqual, ErrNoAccess)
				})

				Convey(`Will return ErrNoAccess if permission is denied.`, func() {
					svc.GH = func(*logdog.GetRequest) (*logdog.GetResponse, error) {
						return nil, grpcutil.PermissionDenied
					}

					_, err := s.Get(c, p)
					So(err, ShouldEqual, ErrNoAccess)
				})
			})

			Convey(`Test State`, func() {
				Convey(`Will request just the state if asked.`, func() {
					svc.GH = func(*logdog.GetRequest) (*logdog.GetResponse, error) {
						return &logdog.GetResponse{
							Project: "myproj",
							State: &logdog.LogStreamState{
								Created: google.NewTimestamp(now),
							},
						}, nil
					}

					l, err := s.State(c)
					So(err, ShouldBeNil)
					So(l, ShouldResemble, &LogStream{
						Project: "myproj",
						Path:    "test/+/a",
						State: &StreamState{
							Created: now.UTC(),
						},
					})

					// Validate the HTTP request that we made.
					So(svc.GR, ShouldResemble, logdog.GetRequest{
						Project:  "myproj",
						Path:     "test/+/a",
						LogCount: -1,
						State:    true,
					})
				})

				Convey(`Will return ErrNoSuchStream if the stream is not found.`, func() {
					svc.GH = func(*logdog.GetRequest) (*logdog.GetResponse, error) {
						return nil, grpcutil.NotFound
					}

					_, err := s.State(c)
					So(err, ShouldEqual, ErrNoSuchStream)
				})

				Convey(`Will return ErrNoAccess if unauthenticated.`, func() {
					svc.GH = func(*logdog.GetRequest) (*logdog.GetResponse, error) {
						return nil, grpcutil.Unauthenticated
					}

					_, err := s.State(c)
					So(err, ShouldEqual, ErrNoAccess)
				})

				Convey(`Will return ErrNoAccess if permission is denied.`, func() {
					svc.GH = func(*logdog.GetRequest) (*logdog.GetResponse, error) {
						return nil, grpcutil.PermissionDenied
					}

					_, err := s.State(c)
					So(err, ShouldEqual, ErrNoAccess)
				})
			})

			Convey(`Test Tail`, func() {
				Convey(`Will form a proper Tail query.`, func() {
					svc.TH = func(*logdog.TailRequest) (*logdog.GetResponse, error) {
						return &logdog.GetResponse{
							Project: "myproj",
							State: &logdog.LogStreamState{
								Created: google.NewTimestamp(now),
							},
							Desc: &logpb.LogStreamDescriptor{
								Prefix:     "test",
								Name:       "a",
								StreamType: logpb.StreamType_TEXT,
							},
							Logs: []*logpb.LogEntry{
								genLog(1337, "kthxbye"),
							},
						}, nil
					}

					var ls LogStream
					l, err := s.Tail(c, &ls)
					So(err, ShouldBeNil)

					// Validate the HTTP request that we made.
					So(svc.TR, ShouldResemble, logdog.TailRequest{
						Project: "myproj",
						Path:    "test/+/a",
						State:   true,
					})

					// Validate that the log and state were returned.
					So(l, ShouldResemble, genLog(1337, "kthxbye"))
					So(ls, ShouldResemble, LogStream{
						Project: "myproj",
						Path:    "test/+/a",
						Desc: &logpb.LogStreamDescriptor{
							Prefix:     "test",
							Name:       "a",
							StreamType: logpb.StreamType_TEXT,
						},
						State: &StreamState{
							Created: now,
						},
					})
				})

				Convey(`Will return nil with state if no logs are returned from the endpoint.`, func() {
					svc.TH = func(*logdog.TailRequest) (*logdog.GetResponse, error) {
						return &logdog.GetResponse{
							Project: "myproj",
							State: &logdog.LogStreamState{
								Created: google.NewTimestamp(now),
							},
							Desc: &logpb.LogStreamDescriptor{
								Prefix:     "test",
								Name:       "a",
								StreamType: logpb.StreamType_TEXT,
							},
						}, nil
					}

					var ls LogStream
					l, err := s.Tail(c, &ls)
					So(err, ShouldBeNil)
					So(l, ShouldBeNil)
					So(ls, ShouldResemble, LogStream{
						Project: "myproj",
						Path:    "test/+/a",
						Desc: &logpb.LogStreamDescriptor{
							Prefix:     "test",
							Name:       "a",
							StreamType: logpb.StreamType_TEXT,
						},
						State: &StreamState{
							Created: now,
						},
					})
				})

				Convey(`Will error if multiple logs are returned from the endpoint.`, func() {
					svc.TH = func(*logdog.TailRequest) (*logdog.GetResponse, error) {
						return &logdog.GetResponse{
							State: &logdog.LogStreamState{
								Created: google.NewTimestamp(now),
							},
							Logs: []*logpb.LogEntry{
								genLog(1337, "ohai"),
								genLog(1338, "kthxbye"),
							},
						}, nil
					}

					_, err := s.Tail(c, nil)
					So(err, ShouldErrLike, "tail call returned 2 logs")
				})

				Convey(`Will return ErrNoSuchStream if the stream is not found.`, func() {
					svc.TH = func(*logdog.TailRequest) (*logdog.GetResponse, error) {
						return nil, grpcutil.NotFound
					}

					_, err := s.Tail(c, nil)
					So(err, ShouldEqual, ErrNoSuchStream)
				})

				Convey(`Will return ErrNoAccess if unauthenticated.`, func() {
					svc.TH = func(*logdog.TailRequest) (*logdog.GetResponse, error) {
						return nil, grpcutil.Unauthenticated
					}

					_, err := s.Tail(c, nil)
					So(err, ShouldEqual, ErrNoAccess)
				})

				Convey(`Will return ErrNoAccess if permission is denied.`, func() {
					svc.TH = func(*logdog.TailRequest) (*logdog.GetResponse, error) {
						return nil, grpcutil.PermissionDenied
					}

					_, err := s.Tail(c, nil)
					So(err, ShouldEqual, ErrNoAccess)
				})
			})
		})
	})
}
