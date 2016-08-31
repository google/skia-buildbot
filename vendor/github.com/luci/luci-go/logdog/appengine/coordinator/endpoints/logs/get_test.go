// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logs

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/luci/gae/filter/featureBreaker"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/data/recordio"
	"github.com/luci/luci-go/common/iotools"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/logs/v1"
	"github.com/luci/luci-go/logdog/api/logpb"
	ct "github.com/luci/luci-go/logdog/appengine/coordinator/coordinatorTest"
	"github.com/luci/luci-go/logdog/common/archive"
	"github.com/luci/luci-go/logdog/common/storage"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

type staticArchiveSource []*logpb.LogEntry

func (s *staticArchiveSource) NextLogEntry() (le *logpb.LogEntry, err error) {
	if len(*s) == 0 {
		err = archive.ErrEndOfStream
	} else {
		le, *s = (*s)[0], (*s)[1:]
	}
	return
}

func shouldHaveLogs(actual interface{}, expected ...interface{}) string {
	resp := actual.(*logdog.GetResponse)

	respLogs := make([]int, len(resp.Logs))
	for i, le := range resp.Logs {
		respLogs[i] = int(le.StreamIndex)
	}

	expLogs := make([]int, len(expected))
	for i, exp := range expected {
		expLogs[i] = exp.(int)
	}

	return ShouldResemble(respLogs, expLogs)
}

// zeroRecords reads a recordio stream and clears all of the record data,
// preserving size data.
func zeroRecords(d []byte) {
	r := bytes.NewReader(d)
	cr := iotools.CountingReader{Reader: r}
	rio := recordio.NewReader(&cr, 4096)
	trash := bytes.Buffer{}

	for {
		s, r, err := rio.ReadFrame()
		if err != nil {
			break
		}

		pos := int(cr.Count())
		for i := int64(0); i < s; i++ {
			d[pos+int(i)] = 0x00
		}

		// Read the (now-zeroed) data.
		trash.Reset()
		trash.ReadFrom(r)
	}
}

func testGetImpl(t *testing.T, archived bool) {
	Convey(fmt.Sprintf(`With a testing configuration, a Get request (archived=%v)`, archived), t, func() {
		c, env := ct.Install()

		svr := New()

		// di is a datastore bound to the test project namespace.
		const project = config.ProjectName("proj-foo")

		// Generate our test stream.
		tls := ct.MakeStream(c, "proj-foo", "testing/+/foo/bar")

		putLogStream := func(c context.Context) {
			if err := tls.Put(c); err != nil {
				panic(err)
			}
		}
		putLogStream(c)

		env.Clock.Add(time.Second)
		var entries []*logpb.LogEntry
		protobufs := map[uint64][]byte{}
		for _, v := range []int{0, 1, 2, 4, 5, 7} {
			le := tls.LogEntry(c, v)
			le.GetText().Lines = append(le.GetText().Lines, &logpb.Text_Line{
				Value: "another line of text",
			})
			entries = append(entries, le)

			switch v {
			case 4:
				le.Content = &logpb.LogEntry_Binary{
					&logpb.Binary{
						Data: []byte{0x00, 0x01, 0x02, 0x03},
					},
				}

			case 5:
				le.Content = &logpb.LogEntry_Datagram{
					&logpb.Datagram{
						Data: []byte{0x00, 0x01, 0x02, 0x03},
						Partial: &logpb.Datagram_Partial{
							Index: 2,
							Size:  1024,
							Last:  false,
						},
					},
				}
			}

			d, err := proto.Marshal(le)
			if err != nil {
				panic(err)
			}
			protobufs[uint64(v)] = d
		}

		// frameSize returns the full RecordIO frame size for the named log protobuf
		// indices.
		frameSize := func(indices ...uint64) int32 {
			var size int
			for _, idx := range indices {
				pb := protobufs[idx]
				size += recordio.FrameHeaderSize(int64(len(pb))) + len(pb)
			}
			if size > math.MaxInt32 {
				panic(size)
			}
			return int32(size)
		}

		Convey(`Testing Get requests (no logs)`, func() {
			req := logdog.GetRequest{
				Project: string(project),
				Path:    string(tls.Path),
			}

			Convey(`Will succeed with no logs.`, func() {
				resp, err := svr.Get(c, &req)

				So(err, ShouldBeRPCOK)
				So(resp, shouldHaveLogs)
			})

			Convey(`Will fail if the Path is not a stream path or a hash.`, func() {
				req.Path = "not/a/full/stream/path"
				_, err := svr.Get(c, &req)
				So(err, ShouldErrLike, "invalid path value")
			})

			Convey(`Will fail with Internal if the datastore Get() doesn't work.`, func() {
				c, fb := featureBreaker.FilterRDS(c, nil)
				fb.BreakFeatures(errors.New("testing error"), "GetMulti")

				_, err := svr.Get(c, &req)
				So(err, ShouldBeRPCInternal)
			})

			Convey(`Will fail with InvalidArgument if the project name is invalid.`, func() {
				req.Project = "!!! invalid project name !!!"
				_, err := svr.Get(c, &req)
				So(err, ShouldBeRPCInvalidArgument)
			})

			Convey(`Will fail with PermissionDenied if the project does not exist.`, func() {
				req.Project = "does-not-exist"
				_, err := svr.Get(c, &req)
				So(err, ShouldBeRPCNotFound)
			})

			Convey(`Will fail with PermissionDenied if the user can't access the project.`, func() {
				req.Project = "proj-exclusive"
				_, err := svr.Get(c, &req)
				So(err, ShouldBeRPCNotFound)
			})

			Convey(`Will fail with NotFound if the log path does not exist (different path).`, func() {
				req.Path = "testing/+/does/not/exist"
				_, err := svr.Get(c, &req)
				So(err, ShouldBeRPCNotFound)
			})
		})

		Convey(`Testing Tail requests (no logs)`, func() {
			req := logdog.TailRequest{
				Project: string(project),
				Path:    string(tls.Path),
			}

			Convey(`Will succeed with no logs.`, func() {
				resp, err := svr.Tail(c, &req)

				So(err, ShouldBeRPCOK)
				So(resp, shouldHaveLogs)
			})

			Convey(`Will fail with PermissionDenied if the project does not exist.`, func() {
				req.Project = "does-not-exist"
				_, err := svr.Tail(c, &req)
				So(err, ShouldBeRPCNotFound)
			})

			Convey(`Will fail with PermissionDenied if the user can't access the project.`, func() {
				req.Project = "proj-exclusive"
				_, err := svr.Tail(c, &req)
				So(err, ShouldBeRPCNotFound)
			})

			Convey(`Will fail with NotFound if the log path does not exist (different path).`, func() {
				req.Path = "testing/+/does/not/exist"
				_, err := svr.Tail(c, &req)
				So(err, ShouldBeRPCNotFound)
			})
		})

		Convey(`When testing log data is added`, func() {
			if !archived {
				// Add the logs to the in-memory temporary storage.
				for _, le := range entries {
					err := env.IntermediateStorage.Put(storage.PutRequest{
						Project: project,
						Path:    tls.Path,
						Index:   types.MessageIndex(le.StreamIndex),
						Values:  [][]byte{protobufs[le.StreamIndex]},
					})
					if err != nil {
						panic(fmt.Errorf("failed to Put() LogEntry: %v", err))
					}
				}
			} else {
				// Archive this log stream. We will generate one index entry for every
				// 2 log entries.
				src := staticArchiveSource(entries)
				var lbuf, ibuf bytes.Buffer
				m := archive.Manifest{
					Desc:             tls.Desc,
					Source:           &src,
					LogWriter:        &lbuf,
					IndexWriter:      &ibuf,
					StreamIndexRange: 2,
				}
				if err := archive.Archive(m); err != nil {
					panic(err)
				}

				now := env.Clock.Now().UTC()

				env.GSClient.Put("gs://testbucket/stream", lbuf.Bytes())
				env.GSClient.Put("gs://testbucket/index", ibuf.Bytes())
				tls.State.TerminatedTime = now
				tls.State.ArchivedTime = now
				tls.State.ArchiveStreamURL = "gs://testbucket/stream"
				tls.State.ArchiveIndexURL = "gs://testbucket/index"

				So(tls.State.ArchivalState().Archived(), ShouldBeTrue)
			}
			putLogStream(c)

			Convey(`Testing Get requests`, func() {
				req := logdog.GetRequest{
					Project: string(project),
					Path:    string(tls.Path),
				}

				Convey(`When the log stream is purged`, func() {
					tls.Stream.Purged = true
					putLogStream(c)

					Convey(`Will return NotFound if the user is not an administrator.`, func() {
						_, err := svr.Get(c, &req)
						So(err, ShouldBeRPCNotFound)
					})

					Convey(`Will process the request if the user is an administrator.`, func() {
						env.JoinGroup("admin")

						resp, err := svr.Get(c, &req)
						So(err, ShouldBeRPCOK)
						So(resp, shouldHaveLogs, 0, 1, 2)
					})
				})

				Convey(`Will return empty if no records were requested.`, func() {
					req.LogCount = -1
					req.State = false

					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp.Logs, ShouldHaveLength, 0)
				})

				Convey(`Will successfully retrieve a stream path.`, func() {
					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 0, 1, 2)
				})

				Convey(`Will successfully retrieve a stream path offset at 4.`, func() {
					req.Index = 4

					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 4, 5)
				})

				Convey(`Will retrieve no logs for contiguous offset 6.`, func() {
					req.Index = 6

					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(len(resp.Logs), ShouldEqual, 0)
				})

				Convey(`Will retrieve log 7 for non-contiguous offset 6.`, func() {
					req.NonContiguous = true
					req.Index = 6

					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 7)
				})

				Convey(`With a byte limit of 1, will still return at least one log entry.`, func() {
					req.ByteCount = 1

					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 0)
				})

				Convey(`With a byte limit of sizeof(0), will return log entry 0.`, func() {
					req.ByteCount = frameSize(0)

					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 0)
				})

				Convey(`With a byte limit of sizeof(0)+1, will return log entry 0.`, func() {
					req.ByteCount = frameSize(0) + 1

					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 0)
				})

				Convey(`With a byte limit of sizeof({0, 1}), will return log entries {0, 1}.`, func() {
					req.ByteCount = frameSize(0, 1)

					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 0, 1)
				})

				Convey(`With a byte limit of sizeof({0, 1, 2}), will return log entries {0, 1, 2}.`, func() {
					req.ByteCount = frameSize(0, 1, 2)

					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 0, 1, 2)
				})

				Convey(`With a byte limit of sizeof({0, 1, 2})+1, will return log entries {0, 1, 2}.`, func() {
					req.ByteCount = frameSize(0, 1, 2) + 1

					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 0, 1, 2)
				})

				Convey(`When requesting state`, func() {
					req.State = true
					req.LogCount = -1

					Convey(`Will successfully retrieve stream state.`, func() {
						resp, err := svr.Get(c, &req)
						So(err, ShouldBeRPCOK)
						So(resp.State, ShouldResemble, buildLogStreamState(tls.Stream, tls.State))
						So(len(resp.Logs), ShouldEqual, 0)
					})

					Convey(`Will return Internal if the protobuf descriptor data is corrupt.`, func() {
						tls.Stream.SetDSValidate(false)
						tls.Stream.Descriptor = []byte{0x00} // Invalid protobuf, zero tag.
						putLogStream(c)

						_, err := svr.Get(c, &req)
						So(err, ShouldBeRPCInternal)
					})
				})

				Convey(`Will return Internal if the protobuf log entry data is corrupt.`, func() {
					if archived {
						// Corrupt the archive datastream.
						stream := env.GSClient.Get("gs://testbucket/stream")
						zeroRecords(stream)
					} else {
						// Add corrupted entry to Storage. Create a new entry here, since
						// the storage will reject a duplicate/overwrite.
						err := env.IntermediateStorage.Put(storage.PutRequest{
							Project: project,
							Path:    types.StreamPath(req.Path),
							Index:   666,
							Values:  [][]byte{{0x00}}, // Invalid protobuf, zero tag.
						})
						if err != nil {
							panic(err)
						}
						req.Index = 666
					}

					_, err := svr.Get(c, &req)
					So(err, ShouldBeRPCInternal)
				})

				Convey(`Will successfully retrieve both logs and stream state.`, func() {
					req.State = true

					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp.State, ShouldResemble, buildLogStreamState(tls.Stream, tls.State))
					So(resp, shouldHaveLogs, 0, 1, 2)
				})

				Convey(`Will return Internal if the Storage is not working.`, func() {
					if archived {
						env.GSClient["error"] = []byte("test error")
					} else {
						env.IntermediateStorage.Close()
					}

					_, err := svr.Get(c, &req)
					So(err, ShouldBeRPCInternal)
				})

				Convey(`Will enforce a maximum count of 2.`, func() {
					req.LogCount = 2
					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 0, 1)
				})

				Convey(`When requesting protobufs`, func() {
					req.State = true

					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 0, 1, 2)

					// Confirm that this has protobufs.
					So(len(resp.Logs), ShouldEqual, 3)
					So(resp.Logs[0], ShouldNotBeNil)

					// Confirm that there is a descriptor protobuf.
					So(resp.Desc, ShouldResemble, tls.Desc)

					// Confirm that the state was returned.
					So(resp.State, ShouldNotBeNil)
				})

				Convey(`Will successfully retrieve all records if non-contiguous is allowed.`, func() {
					req.NonContiguous = true
					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 0, 1, 2, 4, 5, 7)
				})

				Convey(`When newlines are not requested, does not include delimiters.`, func() {
					req.LogCount = 1

					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 0)

					So(resp.Logs[0].GetText(), ShouldResemble, &logpb.Text{
						Lines: []*logpb.Text_Line{
							{"log entry #0", "\n"},
							{"another line of text", ""},
						},
					})
				})

				Convey(`Will get a Binary LogEntry`, func() {
					req.Index = 4
					req.LogCount = 1
					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 4)
					So(resp.Logs[0].GetBinary(), ShouldResemble, &logpb.Binary{
						Data: []byte{0x00, 0x01, 0x02, 0x03},
					})
				})

				Convey(`Will get a Datagram LogEntry`, func() {
					req.Index = 5
					req.LogCount = 1
					resp, err := svr.Get(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 5)
					So(resp.Logs[0].GetDatagram(), ShouldResemble, &logpb.Datagram{
						Data: []byte{0x00, 0x01, 0x02, 0x03},
						Partial: &logpb.Datagram_Partial{
							Index: 2,
							Size:  1024,
							Last:  false,
						},
					})
				})
			})

			Convey(`Testing tail requests`, func() {
				req := logdog.TailRequest{
					Project: string(project),
					Path:    string(tls.Path),
					State:   true,
				}

				Convey(`Will successfully retrieve a stream path.`, func() {
					resp, err := svr.Tail(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, shouldHaveLogs, 7)
					So(resp.State, ShouldResemble, buildLogStreamState(tls.Stream, tls.State))
				})
			})
		})
	})
}

func TestGetIntermediate(t *testing.T) {
	t.Parallel()

	testGetImpl(t, false)
}

func TestGetArchived(t *testing.T) {
	t.Parallel()

	testGetImpl(t, false)
}
