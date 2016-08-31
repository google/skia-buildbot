// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/server/auth/signing"
	"github.com/luci/luci-go/server/router"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCertificatesHandler(t *testing.T) {
	call := func(s signing.Signer) (*signing.PublicCertificates, error) {
		r := router.New()
		InstallHandlers(r, router.NewMiddlewareChain(
			func(c *router.Context, next router.Handler) {
				c.Context = SetConfig(context.Background(), Config{Signer: s})
				next(c)
			},
		))
		ts := httptest.NewServer(r)
		ctx := SetConfig(context.Background(), Config{
			AnonymousTransport: func(context.Context) http.RoundTripper {
				return http.DefaultTransport
			},
		})
		return signing.FetchCertificates(ctx, ts.URL+"/auth/api/v1/server/certificates")
	}

	Convey("Works", t, func() {
		certs, err := call(&phonySigner{})
		So(err, ShouldBeNil)
		So(len(certs.Certificates), ShouldEqual, 1)
	})

	Convey("No signer", t, func() {
		_, err := call(nil)
		So(err, ShouldErrLike, "HTTP code (404)")
	})

	Convey("Error getting certs", t, func() {
		_, err := call(&phonySigner{errors.New("fail")})
		So(err, ShouldErrLike, "HTTP code (500)")
	})
}

func TestServiceInfoHandler(t *testing.T) {
	Convey("Works", t, func() {
		r := router.New()
		signer := &phonySigner{}

		InstallHandlers(r, router.NewMiddlewareChain(
			func(ctx *router.Context, next router.Handler) {
				ctx.Context = SetConfig(context.Background(), Config{Signer: signer})
				next(ctx)
			},
		))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/auth/api/v1/server/info", nil)
		r.ServeHTTP(w, req)
		So(w.Code, ShouldEqual, 200)
		So(w.Body.String(), ShouldResemble,
			`{"app_id":"phony-app","app_runtime":"go",`+
				`"app_runtime_version":"go1.5.1",`+
				`"app_version":"1234-abcdef","service_account_name":`+
				`"phony-app-account@example.com"}`+"\n")

		signer.err = errors.New("fail")

		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/auth/api/v1/server/info", nil)
		r.ServeHTTP(w, req)
		So(w.Code, ShouldEqual, 500)
		So(w.Body.String(), ShouldResemble, "{\"error\":\"Can't grab service info - fail\"}\n")
	})
}

///

type phonySigner struct {
	err error
}

func (s *phonySigner) SignBytes(c context.Context, blob []byte) (string, []byte, error) {
	if s.err != nil {
		return "", nil, s.err
	}
	return "phonyKey", []byte("signature"), nil
}

func (s *phonySigner) Certificates(c context.Context) (*signing.PublicCertificates, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &signing.PublicCertificates{
		Certificates: []signing.Certificate{
			{
				KeyName:            "phonyKey",
				X509CertificatePEM: "phonyPEM",
			},
		},
	}, nil
}

func (s *phonySigner) ServiceInfo(c context.Context) (*signing.ServiceInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &signing.ServiceInfo{
		AppID:              "phony-app",
		AppRuntime:         "go",
		AppRuntimeVersion:  "go1.5.1",
		AppVersion:         "1234-abcdef",
		ServiceAccountName: "phony-app-account@example.com",
	}, nil
}
