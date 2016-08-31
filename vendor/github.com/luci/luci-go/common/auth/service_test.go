// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGroupsService(t *testing.T) {
	Convey("Mocking time and HTTP", t, func() {
		ctx, tc := testclock.UseTime(context.Background(), testclock.TestTimeUTC)
		tc.SetTimerCallback(func(d time.Duration, t clock.Timer) {
			tc.Add(d)
		})

		service := NewGroupsService("https://example.com", &http.Client{})

		requests, callback := mockDoRequest()
		service.doRequest = callback

		Convey("doGet works", func() {
			requests.expect("https://example.com/some/path", 200, `{"key":"val"}`)
			var response struct {
				Key string `json:"key"`
			}
			err := service.doGet(ctx, "/some/path", &response)
			So(err, ShouldBeNil)
			So(response.Key, ShouldEqual, "val")
		})

		Convey("doGet retry works", func() {
			requests.expect("https://example.com/some/path", 500, "")
			requests.expect("https://example.com/some/path", 503, "")
			requests.expect("https://example.com/some/path", 200, `{"key":"val"}`)
			var response struct {
				Key string `json:"key"`
			}
			err := service.doGet(ctx, "/some/path", &response)
			So(err, ShouldBeNil)
			So(response.Key, ShouldEqual, "val")
		})

		Convey("doGet fatal error", func() {
			requests.expect("https://example.com/some/path", 403, "error")
			var response struct{}
			err := service.doGet(ctx, "/some/path", &response)
			So(err, ShouldNotBeNil)
		})

		Convey("doGet max retry", func() {
			for i := 0; i < 17; i++ {
				requests.expect("https://example.com/some/path", 500, "")
			}
			var response struct{}
			err := service.doGet(ctx, "/some/path", &response)
			So(err, ShouldNotBeNil)
		})

		Convey("FetchCallerIdentity works", func() {
			requests.expect("https://example.com/auth/api/v1/accounts/self", 200, `{
				"identity": "user:abc@example.com"
			}`)
			ident, err := service.FetchCallerIdentity(ctx)
			So(err, ShouldBeNil)
			So(ident, ShouldResemble, Identity{
				Kind: IdentityKindUser,
				Name: "abc@example.com",
			})
		})

		Convey("FetchGroup works", func() {
			requests.expect("https://example.com/auth/api/v1/groups/abc/name", 200, `{
				"group": {
					"name": "abc/name",
					"description": "Some description",
					"members": [
						"anonymous:anonymous",
						"bot:127.0.0.1",
						"service:some-gae-app",
						"user:abc@example.com",
						"broken id",
						"unknown_type:abc"
					],
					"globs": ["user:*@example.com"],
					"nested": ["another-group"]
				}
			}`)
			group, err := service.FetchGroup(ctx, "abc/name")
			So(err, ShouldBeNil)
			So(group, ShouldResemble, Group{
				Name:        "abc/name",
				Description: "Some description",
				Members: []Identity{
					{Kind: IdentityKindAnonymous, Name: "anonymous"},
					{Kind: IdentityKindBot, Name: "127.0.0.1"},
					{Kind: IdentityKindService, Name: "some-gae-app"},
					{Kind: IdentityKindUser, Name: "abc@example.com"},
				},
				Globs:  []string{"user:*@example.com"},
				Nested: []string{"another-group"},
			})
		})
	})
}

type mockedRequest struct {
	URL        string
	StatusCode int
	Response   string
}

type mockedRequests struct {
	Expected []mockedRequest
}

type reqCb func(context.Context, *http.Client, *http.Request) (*http.Response, error)

func mockDoRequest() (*mockedRequests, reqCb) {
	requests := &mockedRequests{}
	doRequest := func(ctx context.Context, c *http.Client, req *http.Request) (*http.Response, error) {
		So(len(requests.Expected), ShouldNotEqual, 0)
		next := requests.Expected[0]
		requests.Expected = requests.Expected[1:]
		So(next.URL, ShouldEqual, req.URL.String())
		return &http.Response{
			StatusCode: next.StatusCode,
			Body:       ioutil.NopCloser(strings.NewReader(next.Response)),
		}, nil
	}
	return requests, doRequest
}

func (r *mockedRequests) expect(url string, status int, response string) {
	r.Expected = append(r.Expected, mockedRequest{
		URL:        url,
		StatusCode: status,
		Response:   response,
	})
}

func (r *mockedRequests) assertExecuted() {
	So(r.Expected, ShouldBeEmpty)
}
