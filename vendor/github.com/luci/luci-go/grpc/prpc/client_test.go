// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package prpc

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/memlogger"
	"github.com/luci/luci-go/common/retry"

	. "github.com/smartystreets/goconvey/convey"
)

func sayHello(c C) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.So(r.Method, ShouldEqual, "POST")
		c.So(r.URL.Path, ShouldEqual, "/prpc/prpc.Greeter/SayHello")
		c.So(r.Header.Get("Accept"), ShouldEqual, "application/prpc; encoding=binary")
		c.So(r.Header.Get("Content-Type"), ShouldEqual, "application/prpc; encoding=binary")
		c.So(r.Header.Get("User-Agent"), ShouldEqual, "prpc-test")

		if timeout := r.Header.Get(HeaderTimeout); timeout != "" {
			c.So(timeout, ShouldEqual, "10000000u")
		}

		reqBody, err := ioutil.ReadAll(r.Body)
		c.So(err, ShouldBeNil)

		var req HelloRequest
		err = proto.Unmarshal(reqBody, &req)
		c.So(err, ShouldBeNil)

		if req.Name == "TOO BIG" {
			w.Header().Set("Content-Length", "999999999999")
		}
		w.Header().Set("X-Lower-Case-Header", "CamelCaseValueStays")

		res := HelloReply{"Hello " + req.Name}
		buf, err := proto.Marshal(&res)
		c.So(err, ShouldBeNil)

		w.Header().Set(HeaderGRPCCode, strconv.Itoa(int(codes.OK)))
		w.Header().Set("Content-Type", ContentTypePRPC)
		_, err = w.Write(buf)
		c.So(err, ShouldBeNil)
	}
}

func doPanicHandler(w http.ResponseWriter, r *http.Request) {
	panic("test panic")
}

func transientErrors(count int, then http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if count > 0 {
			count--
			w.Header().Set(HeaderGRPCCode, strconv.Itoa(int(codes.Internal)))
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "Server misbehaved")
			return
		}
		then.ServeHTTP(w, r)
	}
}

func shouldHaveMessagesLike(actual interface{}, expected ...interface{}) string {
	log := actual.(*memlogger.MemLogger)
	msgs := log.Messages()

	So(msgs, ShouldHaveLength, len(expected))
	for i, actual := range msgs {
		expected := expected[i].(memlogger.LogEntry)
		So(actual.Level, ShouldEqual, expected.Level)
		So(actual.Msg, ShouldContainSubstring, expected.Msg)
	}
	return ""
}

func TestClient(t *testing.T) {
	t.Parallel()

	Convey("Client", t, func() {
		setUp := func(h http.HandlerFunc) (*Client, *httptest.Server) {
			server := httptest.NewServer(h)
			client := &Client{
				Host: strings.TrimPrefix(server.URL, "http://"),
				Options: &Options{
					Retry: func() retry.Iterator {
						return &retry.Limited{
							Retries: 3,
							Delay:   0,
						}
					},
					Insecure:  true,
					UserAgent: "prpc-test",
				},
			}
			return client, server
		}

		ctx, _ := testclock.UseTime(context.Background(), testclock.TestTimeLocal)
		ctx = memlogger.Use(ctx)
		log := logging.Get(ctx).(*memlogger.MemLogger)
		expectedCallLogEntry := func(c *Client) memlogger.LogEntry {
			return memlogger.LogEntry{
				Level: logging.Debug,
				Msg:   fmt.Sprintf("RPC %s/prpc.Greeter.SayHello", c.Host),
			}
		}

		req := &HelloRequest{"John"}
		res := &HelloReply{}

		Convey("Call", func() {
			Convey("Works", func(c C) {
				client, server := setUp(sayHello(c))
				defer server.Close()

				var hd metadata.MD
				err := client.Call(ctx, "prpc.Greeter", "SayHello", req, res, Header(&hd))
				So(err, ShouldBeNil)
				So(res.Message, ShouldEqual, "Hello John")
				So(hd["x-lower-case-header"], ShouldResemble, []string{"CamelCaseValueStays"})

				So(log, shouldHaveMessagesLike, expectedCallLogEntry(client))
			})

			Convey("With a deadline <= now, does not execute.", func(c C) {
				client, server := setUp(doPanicHandler)
				defer server.Close()

				ctx, _ = context.WithDeadline(ctx, clock.Now(ctx))
				err := client.Call(ctx, "prpc.Greeter", "SayHello", req, res)
				So(err, ShouldEqual, context.DeadlineExceeded)
			})

			Convey("With a deadline in the future, sets the deadline header.", func(c C) {
				client, server := setUp(sayHello(c))
				defer server.Close()

				ctx, _ = clock.WithDeadline(ctx, clock.Now(ctx).Add(10*time.Second))
				err := client.Call(ctx, "prpc.Greeter", "SayHello", req, res)
				So(err, ShouldBeNil)
				So(res.Message, ShouldEqual, "Hello John")

				So(log, shouldHaveMessagesLike, expectedCallLogEntry(client))
			})

			Convey(`With a maximum content length smaller than the response, returns "ErrResponseTooBig".`, func(c C) {
				client, server := setUp(sayHello(c))
				defer server.Close()

				client.MaxContentLength = 8
				err := client.Call(ctx, "prpc.Greeter", "SayHello", req, res)
				So(err, ShouldEqual, ErrResponseTooBig)
			})

			Convey(`When the response returns a huge Content Length, returns "ErrResponseTooBig".`, func(c C) {
				client, server := setUp(sayHello(c))
				defer server.Close()

				req.Name = "TOO BIG"
				err := client.Call(ctx, "prpc.Greeter", "SayHello", req, res)
				So(err, ShouldEqual, ErrResponseTooBig)
			})

			Convey("HTTP 500 x2", func(c C) {
				client, server := setUp(transientErrors(2, sayHello(c)))
				defer server.Close()

				err := client.Call(ctx, "prpc.Greeter", "SayHello", req, res)
				So(err, ShouldBeNil)
				So(res.Message, ShouldEqual, "Hello John")

				So(log, shouldHaveMessagesLike,
					expectedCallLogEntry(client),
					memlogger.LogEntry{Level: logging.Warning, Msg: "RPC failed transiently. Will retry in 0"},

					expectedCallLogEntry(client),
					memlogger.LogEntry{Level: logging.Warning, Msg: "RPC failed transiently. Will retry in 0"},

					expectedCallLogEntry(client),
				)
			})

			Convey("HTTP 500 many", func(c C) {
				client, server := setUp(transientErrors(10, sayHello(c)))
				defer server.Close()

				err := client.Call(ctx, "prpc.Greeter", "SayHello", req, res)
				So(grpc.Code(err), ShouldEqual, codes.Internal)
				So(grpc.ErrorDesc(err), ShouldEqual, "Server misbehaved")

				So(log, shouldHaveMessagesLike,
					expectedCallLogEntry(client),
					memlogger.LogEntry{Level: logging.Warning, Msg: "RPC failed transiently. Will retry in 0"},

					expectedCallLogEntry(client),
					memlogger.LogEntry{Level: logging.Warning, Msg: "RPC failed transiently. Will retry in 0"},

					expectedCallLogEntry(client),
					memlogger.LogEntry{Level: logging.Warning, Msg: "RPC failed transiently. Will retry in 0"},

					expectedCallLogEntry(client),
					memlogger.LogEntry{Level: logging.Warning, Msg: "RPC failed permanently"},
				)
			})

			Convey("Forbidden", func(c C) {
				client, server := setUp(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set(HeaderGRPCCode, strconv.Itoa(int(codes.PermissionDenied)))
					w.WriteHeader(http.StatusForbidden)
					fmt.Fprintln(w, "Access denied")
				})
				defer server.Close()

				err := client.Call(ctx, "prpc.Greeter", "SayHello", req, res)
				So(grpc.Code(err), ShouldEqual, codes.PermissionDenied)
				So(grpc.ErrorDesc(err), ShouldEqual, "Access denied")

				So(log, shouldHaveMessagesLike,
					expectedCallLogEntry(client),
					memlogger.LogEntry{Level: logging.Warning, Msg: "RPC failed permanently"},
				)
			})

			Convey(HeaderGRPCCode, func(c C) {
				client, server := setUp(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set(HeaderGRPCCode, strconv.Itoa(int(codes.Canceled)))
					w.WriteHeader(http.StatusBadRequest)
				})
				defer server.Close()

				err := client.Call(ctx, "prpc.Greeter", "SayHello", req, res)
				So(grpc.Code(err), ShouldEqual, codes.Canceled)
			})
		})
	})
}
