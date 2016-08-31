// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package prpc

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"

	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/memlogger"

	. "github.com/smartystreets/goconvey/convey"
)

func TestResponse(t *testing.T) {
	t.Parallel()

	Convey("response", t, func() {
		c := context.Background()
		c = memlogger.Use(c)
		log := logging.Get(c).(*memlogger.MemLogger)

		rec := httptest.NewRecorder()

		Convey("ok", func() {
			r := response{
				body: []byte("hi"),
				header: http.Header{
					headerContentType: []string{"text/html"},
				},
			}
			r.write(c, rec)
			So(rec.Code, ShouldEqual, http.StatusOK)
			So(rec.Header().Get(HeaderGRPCCode), ShouldEqual, "0")
			So(rec.Header().Get(headerContentType), ShouldEqual, "text/html")
			So(rec.Body.String(), ShouldEqual, "hi")
		})

		Convey("client error", func() {
			r := errResponse(codes.NotFound, 0, "not found")
			r.write(c, rec)
			So(rec.Code, ShouldEqual, http.StatusNotFound)
			So(rec.Header().Get(HeaderGRPCCode), ShouldEqual, "5")
			So(rec.Header().Get(headerContentType), ShouldEqual, "text/plain")
			So(rec.Body.String(), ShouldEqual, "not found\n")
		})

		Convey("internal error", func() {
			r := errResponse(codes.Internal, 0, "errmsg")
			r.write(c, rec)
			So(rec.Code, ShouldEqual, http.StatusInternalServerError)
			So(rec.Header().Get(HeaderGRPCCode), ShouldEqual, "13")
			So(rec.Header().Get(headerContentType), ShouldEqual, "text/plain")
			So(rec.Body.String(), ShouldEqual, "Internal Server Error\n")
			So(log, memlogger.ShouldHaveLog, logging.Error, "errmsg", map[string]interface{}{
				"code": codes.Internal,
			})
		})

		Convey("unknown error", func() {
			r := errResponse(codes.Unknown, 0, "errmsg")
			r.write(c, rec)
			So(rec.Code, ShouldEqual, http.StatusInternalServerError)
			So(rec.Header().Get(HeaderGRPCCode), ShouldEqual, "2")
			So(rec.Header().Get(headerContentType), ShouldEqual, "text/plain")
			So(rec.Body.String(), ShouldEqual, "Internal Server Error\n")
			So(log, memlogger.ShouldHaveLog, logging.Error, "errmsg", map[string]interface{}{
				"code": codes.Unknown,
			})
		})
	})
}
