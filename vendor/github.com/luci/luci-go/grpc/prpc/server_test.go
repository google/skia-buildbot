// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package prpc

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/router"

	. "github.com/smartystreets/goconvey/convey"
)

type greeterService struct{}

func (s *greeterService) SayHello(c context.Context, req *HelloRequest) (*HelloReply, error) {
	if req.Name == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, "Name unspecified")
	}

	return &HelloReply{
		Message: "Hello " + req.Name,
	}, nil
}

type calcService struct{}

func (s *calcService) Multiply(c context.Context, req *MultiplyRequest) (*MultiplyResponse, error) {
	return &MultiplyResponse{
		Z: req.X & req.Y,
	}, nil
}

func TestServer(t *testing.T) {
	t.Parallel()

	Convey("Greeter service", t, func() {
		var server Server

		// auth.Authenticator.Authenticate is not designed to be called in tests.
		server.Authenticator = auth.Authenticator{}

		RegisterGreeterServer(&server, &greeterService{})

		Convey("Register Calc service", func() {
			RegisterCalcServer(&server, &calcService{})
			So(server.ServiceNames(), ShouldResemble, []string{
				"prpc.Calc",
				"prpc.Greeter",
			})
		})

		Convey("Handlers", func() {
			c := context.Background()
			r := router.New()
			server.InstallHandlers(r, router.NewMiddlewareChain(
				func(ctx *router.Context, next router.Handler) {
					ctx.Context = c
					next(ctx)
				},
			))
			res := httptest.NewRecorder()
			hiMsg := bytes.NewBufferString(`name: "Lucy"`)
			req, err := http.NewRequest("POST", "/prpc/prpc.Greeter/SayHello", hiMsg)
			So(err, ShouldBeNil)
			req.Header.Set("Content-Type", mtPRPCText)

			invalidArgument := strconv.Itoa(int(codes.InvalidArgument))
			unimplemented := strconv.Itoa(int(codes.Unimplemented))

			Convey("Works", func() {
				req.Header.Set("Accept", mtPRPCText)
				r.ServeHTTP(res, req)
				So(res.Code, ShouldEqual, http.StatusOK)
				So(res.Header().Get(HeaderGRPCCode), ShouldEqual, "0")
				So(res.Body.String(), ShouldEqual, "message: \"Hello Lucy\"\n")
			})

			Convey("Invalid Accept header", func() {
				req.Header.Set("Accept", "blah")
				r.ServeHTTP(res, req)
				So(res.Code, ShouldEqual, http.StatusNotAcceptable)
				So(res.Header().Get(HeaderGRPCCode), ShouldEqual, invalidArgument)
			})

			Convey("Invalid header", func() {
				req.Header.Set("X-Bin", "zzz")
				r.ServeHTTP(res, req)
				So(res.Code, ShouldEqual, http.StatusBadRequest)
				So(res.Header().Get(HeaderGRPCCode), ShouldEqual, invalidArgument)
			})

			Convey("Malformed request message", func() {
				hiMsg.WriteString("\nblah")
				r.ServeHTTP(res, req)
				So(res.Code, ShouldEqual, http.StatusBadRequest)
				So(res.Header().Get(HeaderGRPCCode), ShouldEqual, invalidArgument)
			})

			Convey("Invalid request message", func() {
				hiMsg.Reset()
				r.ServeHTTP(res, req)
				So(res.Code, ShouldEqual, http.StatusBadRequest)
				So(res.Header().Get(HeaderGRPCCode), ShouldEqual, invalidArgument)
				So(res.Body.String(), ShouldEqual, "Name unspecified\n")
			})

			Convey("no such service", func() {
				req.URL.Path = "/prpc/xxx/SayHello"
				r.ServeHTTP(res, req)
				So(res.Code, ShouldEqual, http.StatusNotImplemented)
				So(res.Header().Get(HeaderGRPCCode), ShouldEqual, unimplemented)
			})
			Convey("no such method", func() {
				req.URL.Path = "/prpc/Greeter/xxx"
				r.ServeHTTP(res, req)
				So(res.Code, ShouldEqual, http.StatusNotImplemented)
				So(res.Header().Get(HeaderGRPCCode), ShouldEqual, unimplemented)
			})

			Convey(`When access control is enabled for "http://example.com"`, func() {
				server.AccessControl = func(c context.Context, origin string) bool {
					return origin == "http://example.com"
				}

				Convey(`When sending an OPTIONS request`, func() {
					req.Method = "OPTIONS"

					Convey(`Will supply Access-* headers to "http://example.com"`, func() {
						req.Header.Add("Origin", "http://example.com")

						r.ServeHTTP(res, req)
						So(res.Code, ShouldEqual, http.StatusOK)
						So(res.Header().Get("Access-Control-Allow-Origin"), ShouldEqual, "http://example.com")
						So(res.Header().Get("Access-Control-Allow-Credentials"), ShouldEqual, "true")
						So(res.Header().Get("Access-Control-Allow-Headers"), ShouldEqual, "Origin, Content-Type, Accept, Authorization")
						So(res.Header().Get("Access-Control-Allow-Methods"), ShouldEqual, "OPTIONS, POST")
						So(res.Header().Get("Access-Control-Max-Age"), ShouldEqual, "600")
					})

					Convey(`Will not supply access-* headers to "http://foo.bar"`, func() {
						req.Header.Add("Origin", "http://foo.bar")

						r.ServeHTTP(res, req)
						So(res.Code, ShouldEqual, http.StatusOK)
						So(res.Header().Get("Access-Control-Allow-Origin"), ShouldEqual, "")
					})
				})

				Convey(`When sending a POST request`, func() {
					Convey(`Will supply Access-* headers to "http://example.com"`, func() {
						req.Header.Add("Origin", "http://example.com")

						r.ServeHTTP(res, req)
						So(res.Code, ShouldEqual, http.StatusOK)
						So(res.Header().Get(HeaderGRPCCode), ShouldEqual, "0")
						So(res.Header().Get("Access-Control-Allow-Origin"), ShouldEqual, "http://example.com")
						So(res.Header().Get("Access-Control-Allow-Credentials"), ShouldEqual, "true")
						So(res.Header().Get("Access-Control-Expose-Headers"), ShouldEqual, HeaderGRPCCode)
					})

					Convey(`Will not supply access-* headers to "http://foo.bar"`, func() {
						req.Header.Add("Origin", "http://foo.bar")

						r.ServeHTTP(res, req)
						So(res.Code, ShouldEqual, http.StatusOK)
						So(res.Header().Get(HeaderGRPCCode), ShouldEqual, "0")
						So(res.Header().Get("Access-Control-Allow-Origin"), ShouldEqual, "")
					})
				})
			})
		})
	})
}
