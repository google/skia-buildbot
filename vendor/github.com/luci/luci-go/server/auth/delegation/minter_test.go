// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package delegation

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/logging/memlogger"
	"github.com/luci/luci-go/server/auth/identity"
	"github.com/luci/luci-go/server/auth/internal"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCreateToken(t *testing.T) {
	ctx := memlogger.Use(context.Background())
	ctx, _ = testclock.UseTime(ctx, testclock.TestRecentTimeUTC)

	goodReq := TokenRequest{
		AuthServiceURL:   "example.com",
		Audience:         []identity.Identity{"user:a@example.com"},
		AudienceGroups:   []string{"group"},
		TargetServices:   []identity.Identity{"service:abc"},
		Impersonate:      "user:b@example.com",
		ValidityDuration: time.Hour,
		Intent:           "intent",
	}

	ctx, tr := withTestTransport(ctx, `{
		"delegation_token": "tok",
		"validity_duration": 3600,
		"subtoken_id": "123"
	}`)

	Convey("Works", t, func() {
		tok, err := CreateToken(ctx, goodReq)
		So(err, ShouldBeNil)
		So(tok, ShouldResemble, &Token{
			Token:      "tok",
			SubtokenID: "123",
			Expiry:     testclock.TestRecentTimeUTC.Add(time.Hour),
		})
		So(tr.request, ShouldEqual,
			`{"audience":["user:a@example.com","group:group"],`+
				`"services":["service:abc"],"validity_duration":3600,`+
				`"impersonate":"user:b@example.com","intent":"intent"}`)
	})

	Convey("Audience check works", t, func() {
		req := goodReq
		req.Audience = nil
		req.AudienceGroups = nil
		_, err := CreateToken(ctx, req)
		So(err, ShouldErrLike, "either Audience/AudienceGroups or UnlimitedAudience=true are required")

		req = goodReq
		req.UnlimitedAudience = true
		_, err = CreateToken(ctx, req)
		So(err, ShouldErrLike, "can't specify audience for UnlimitedAudience=true token")

		req = goodReq
		req.Audience = nil
		req.AudienceGroups = nil
		req.UnlimitedAudience = true
		_, err = CreateToken(ctx, req)
		So(err, ShouldBeNil)
	})

	Convey("Services check works", t, func() {
		req := goodReq
		req.TargetServices = nil
		_, err := CreateToken(ctx, req)
		So(err, ShouldErrLike, "either TargetServices or Untargeted=true are required")

		req = goodReq
		req.Untargeted = true
		_, err = CreateToken(ctx, req)
		So(err, ShouldErrLike, "can't specify TargetServices for Untargeted=true token")

		req = goodReq
		req.TargetServices = nil
		req.Untargeted = true
		_, err = CreateToken(ctx, req)
		So(err, ShouldBeNil)
	})
}

var testTransportKey = "key for testTransport"

func withTestTransport(c context.Context, response string) (context.Context, *testTransport) {
	t := &testTransport{response: response}
	return context.WithValue(c, &testTransportKey, t), t
}

type testTransport struct {
	response string
	request  string
}

func (f *testTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	body, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, err
	}
	f.request = string(body)
	return &http.Response{
		StatusCode: 200,
		Status:     "OK",
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(f.response))),
	}, nil
}

func init() {
	internal.RegisterClientFactory(func(c context.Context, scopes []string) (*http.Client, error) {
		return &http.Client{Transport: c.Value(&testTransportKey).(*testTransport)}, nil
	})
}
