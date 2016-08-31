// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cloudlogging

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/api/googleapi"
	cloudlog "google.golang.org/api/logging/v1beta3"
)

type testLogRequest struct {
	url         *url.URL
	contentType string
	userAgent   string
	request     cloudlog.WriteLogEntriesRequest
}

type testCloudLoggingServer struct {
	logC chan *testLogRequest
}

func (h *testCloudLoggingServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if req.Header.Get("content-type") != "application/json" {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	testReq := testLogRequest{
		url:         req.URL,
		contentType: req.Header.Get("content-type"),
		userAgent:   req.Header.Get("user-agent"),
	}

	if err := json.Unmarshal(body, &testReq.request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.logC <- &testReq

	// Write an empty response to the client.
	w.Write([]byte("{}"))
}

func shouldResembleLogs(actual interface{}, expected ...interface{}) string {
	errors := []string{}
	addError := func(f string, args ...interface{}) {
		errors = append(errors, fmt.Sprintf(f, args...))
	}

	actualEntries, ok := actual.([]*cloudlog.LogEntry)
	if !ok {
		return "Actual: Not a cloud log entry bundle."
	}

	for i, act := range actualEntries {
		if len(expected) == 0 {
			addError("Actual #%d: additional entry %#v", i, act)
			continue
		}

		exp, ok := expected[i].(*cloudlog.LogEntry)
		if !ok {
			addError("Expected #%d: not a *cloudlog.LogEntry.", i)
			continue
		}

		actClone := *act
		expClone := *exp
		actClone.Metadata = nil
		expClone.Metadata = nil
		if err := ShouldResemble(actClone, expClone); err != "" {
			addError("Entry #%d:\n  Actual:   %#v\n  Expected: %#v\n", i, &actClone, &expClone)
		}

		if err := ShouldResemble(act.Metadata, exp.Metadata); err != "" {
			addError("Entry #%d Metadata:\n  Actual:   %#v\n  Expected: %#v\n",
				i, act.Metadata, exp.Metadata)
		}
	}

	for i := len(actualEntries); i < len(expected); i++ {
		addError("Expected #%d: additional entry %#v", i, expected[i])
	}
	return strings.Join(errors, "\n")
}

func TestClient(t *testing.T) {
	t.Parallel()
	now := time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)

	Convey(`A client connected to a testing server`, t, func() {
		opts := ClientOptions{
			UserAgent:    "client.go test client",
			ProjectID:    "test-project",
			LogID:        "test-log-id",
			ResourceType: "test-resource",
			ResourceID:   "test-resource-id",
		}

		h := &testCloudLoggingServer{
			logC: make(chan *testLogRequest, 1),
		}
		srv := httptest.NewServer(h)
		defer srv.Close()

		tr := &http.Transport{
			DialTLS: func(network, addr string) (net.Conn, error) {
				u, err := url.Parse(srv.URL)
				if err != nil {
					return nil, err
				}
				return net.Dial(network, u.Host)
			},
		}
		client := &http.Client{
			Transport: tr,
		}

		c, err := NewClient(opts, client)
		So(err, ShouldBeNil)

		Convey(`Can send a log entry bundle.`, func() {
			err = c.PushEntries([]*Entry{
				{
					InsertID:    "test-log-0",
					Timestamp:   now,
					Severity:    Debug,
					TextPayload: "hi",
					Labels: Labels{
						"foo": "bar",
					},
				},
				{
					InsertID:    "test-log-1",
					Timestamp:   now,
					Severity:    Info,
					TextPayload: "sup",
					Labels: Labels{
						"baz": "qux",
					},
				},
			})
			So(err, ShouldBeNil)

			req := <-h.logC
			So(req.url.Path, ShouldEqual, "/v1beta3/projects/test-project/logs/test-log-id/entries:write")
			So(req.contentType, ShouldEqual, "application/json")
			So(req.userAgent, ShouldEqual, googleapi.UserAgent+" client.go test client")
			So(req.request.CommonLabels, ShouldResemble, map[string]string{
				"compute.googleapis.com/resource_type": "test-resource",
				"compute.googleapis.com/resource_id":   "test-resource-id",
			})
			So(req.request.Entries, shouldResembleLogs,
				&cloudlog.LogEntry{
					InsertId: "test-log-0",
					Metadata: &cloudlog.LogEntryMetadata{
						Labels: map[string]string{
							"foo": "bar",
						},
						ProjectId: "test-project",
						Severity:  "DEBUG",
						Timestamp: "2015-01-01T00:00:00Z",
					},
					TextPayload: "hi",
				},

				&cloudlog.LogEntry{
					InsertId: "test-log-1",
					Metadata: &cloudlog.LogEntryMetadata{
						Labels: map[string]string{
							"baz": "qux",
						},
						ProjectId: "test-project",
						Severity:  "INFO",
						Timestamp: "2015-01-01T00:00:00Z",
					},
					TextPayload: "sup",
				},
			)
		})
	})
}
