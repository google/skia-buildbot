// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package httpmitm

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type record struct {
	o Origin
	d string
	e error
}

// shouldRecord tests whether a captured record (actual) matches the expected result:
// 0) Origin
// 1) bool (are we expecting an error?)
// 2) string (optional). If present, the regexp pattern that must match the data.
func shouldRecord(actual interface{}, expected ...interface{}) string {
	r := actual.(*record)
	o := expected[0].(Origin)
	e := expected[1].(bool)

	var re string
	if len(expected) == 3 {
		re = expected[2].(string)
	}

	if err := ShouldEqual(r.o, o); err != "" {
		return err
	}
	if !e {
		// No error.
		if err := ShouldBeNil(r.e); err != "" {
			return err
		}
	} else {
		// Error expected.
		if err := ShouldNotBeNil(r.e); err != "" {
			return err
		}
	}
	if re != "" {
		m, e := regexp.MatchString(re, r.d)
		if err := ShouldEqual(e, nil); err != "" {
			return err
		}
		if err := ShouldBeTrue(m); err != "" {
			return err
		}
	}
	return ""
}

func TestTransport(t *testing.T) {
	t.Parallel()

	Convey(`A debug HTTP client`, t, func() {
		// Generic callback-based server. Each test will set its callback.
		var callback func() (string, error)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := errors.New("No Callback.")
			if callback != nil {
				var content string
				content, err = callback()
				if err == nil {
					_, err = w.Write([]byte(content))
				}
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}))
		defer ts.Close()

		var records []*record
		client := http.Client{
			Transport: &Transport{
				Callback: func(o Origin, data []byte, err error) {
					records = append(records, &record{o, string(data), err})
				},
			},
		}

		Convey(`Successfully fetches content.`, func() {
			callback = func() (string, error) {
				return "Hello, client!", nil
			}
			resp, err := client.Post(ts.URL, "test", bytes.NewBufferString("DATA"))
			So(err, ShouldBeNil)
			defer resp.Body.Close()

			So(len(records), ShouldEqual, 2)
			So(records[0], shouldRecord, Request, false,
				"(?s:POST / HTTP/1.1\r\n(.+:.+\r\n)*\r\nDATA)")
			So(records[1], shouldRecord, Response, false,
				"(?s:HTTP/1.1 200 OK\r\n(.+:.+\r\n)*\r\nHello, client!)")

			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(string(body), ShouldEqual, "Hello, client!")
			So(resp.StatusCode, ShouldEqual, http.StatusOK)
		})

		Convey(`Handles HTTP error.`, func() {
			callback = func() (string, error) {
				return "", errors.New("Failure!")
			}
			resp, err := client.Post(ts.URL, "test", bytes.NewBufferString("DATA"))
			So(err, ShouldBeNil)
			defer resp.Body.Close()

			So(len(records), ShouldEqual, 2)
			So(records[0], shouldRecord, Request, false,
				"(?s:POST / HTTP/1.1\r\n(.+:.+\r\n)*\r\nDATA)")
			So(records[1], shouldRecord, Response, false,
				"(?s:HTTP/1.1 500 Internal Server Error\r\n(.+:.+\r\n)*\r\nFailure!)")

			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(string(body), ShouldEqual, "Failure!\n")
			So(resp.StatusCode, ShouldEqual, http.StatusInternalServerError)
		})

		Convey(`Handles connection error.`, func() {
			_, err := client.Get("http+testingfakeprotocol://")
			So(err, ShouldNotBeNil)

			So(len(records), ShouldEqual, 2)
			So(records[0], shouldRecord, Request, false,
				"(?s:GET / HTTP/1.1\r\n(.+:.+\r\n)*)")
			So(records[1], shouldRecord, Response, true)
		})
	})
}
