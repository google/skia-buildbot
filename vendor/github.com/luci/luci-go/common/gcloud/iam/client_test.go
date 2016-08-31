// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package iam

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func TestClient(t *testing.T) {
	Convey("ModifyIAMPolicy works", t, func(c C) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				panic(err)
			}

			switch r.URL.Path {
			case "/v1/project/1/resource/2:getIamPolicy":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				w.Write([]byte(`{"etag":"blah"}`))

			case "/v1/project/1/resource/2:setIamPolicy":
				c.So(string(body), ShouldEqual,
					`{"policy":{"bindings":[{"role":"role","members":["principal"]}],"etag":"blah"}}`)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				w.Write([]byte(`{"bindings":[{"role":"role","members":["principal"]}],"etag":"blah"}}`))

			default:
				c.Printf("Unknown URL: %q\n", r.URL.Path)
				w.WriteHeader(404)
			}
		}))
		defer ts.Close()

		cl := Client{
			Client:   http.DefaultClient,
			BasePath: ts.URL,
		}

		err := cl.ModifyIAMPolicy(context.Background(), "project/1/resource/2", func(p *Policy) error {
			p.GrantRole("role", "principal")
			return nil
		})
		So(err, ShouldBeNil)
	})
}
