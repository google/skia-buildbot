// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package buildbucket

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/context"
	"google.golang.org/api/pubsub/v1"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/luci-go/cron/appengine/messages"
	"github.com/luci/luci-go/cron/appengine/task"
	"github.com/luci/luci-go/cron/appengine/task/utils/tasktest"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestValidateProtoMessage(t *testing.T) {
	tm := TaskManager{}

	Convey("ValidateProtoMessage passes good msg", t, func() {
		So(tm.ValidateProtoMessage(&messages.BuildbucketTask{
			Server:     strPtr("https://blah.com"),
			Bucket:     strPtr("bucket"),
			Builder:    strPtr("builder"),
			Tags:       []string{"a:b", "c:d"},
			Properties: []string{"a:b", "c:d"},
		}), ShouldBeNil)
	})

	Convey("ValidateProtoMessage passes good minimal msg", t, func() {
		So(tm.ValidateProtoMessage(&messages.BuildbucketTask{
			Server:  strPtr("https://blah.com"),
			Bucket:  strPtr("bucket"),
			Builder: strPtr("builder"),
		}), ShouldBeNil)
	})

	Convey("ValidateProtoMessage wrong type", t, func() {
		So(tm.ValidateProtoMessage(&messages.NoopTask{}), ShouldErrLike, "wrong type")
	})

	Convey("ValidateProtoMessage empty", t, func() {
		So(tm.ValidateProtoMessage(tm.ProtoMessageType()), ShouldErrLike, "field 'server' is required")
	})

	Convey("ValidateProtoMessage validates URL", t, func() {
		call := func(url string) error {
			return tm.ValidateProtoMessage(&messages.BuildbucketTask{
				Server:  &url,
				Bucket:  strPtr("bucket"),
				Builder: strPtr("builder"),
			})
		}
		So(call(""), ShouldErrLike, "field 'server' is required")
		So(call("%%%%"), ShouldErrLike, "invalid URL")
		So(call("/abc"), ShouldErrLike, "not an absolute url")
		So(call("https://host/not-root"), ShouldErrLike, "not a host root url")
	})

	Convey("ValidateProtoMessage needs bucket", t, func() {
		So(tm.ValidateProtoMessage(&messages.BuildbucketTask{
			Server:  strPtr("https://blah.com"),
			Builder: strPtr("builder"),
		}), ShouldErrLike, "'bucket' field is required")
	})

	Convey("ValidateProtoMessage needs builder", t, func() {
		So(tm.ValidateProtoMessage(&messages.BuildbucketTask{
			Server: strPtr("https://blah.com"),
			Bucket: strPtr("bucket"),
		}), ShouldErrLike, "'builder' field is required")
	})

	Convey("ValidateProtoMessage validates properties", t, func() {
		So(tm.ValidateProtoMessage(&messages.BuildbucketTask{
			Server:     strPtr("https://blah.com"),
			Bucket:     strPtr("bucket"),
			Builder:    strPtr("builder"),
			Properties: []string{"not_kv_pair"},
		}), ShouldErrLike, "bad property, not a 'key:value' pair")
	})

	Convey("ValidateProtoMessage validates tags", t, func() {
		So(tm.ValidateProtoMessage(&messages.BuildbucketTask{
			Server:  strPtr("https://blah.com"),
			Bucket:  strPtr("bucket"),
			Builder: strPtr("builder"),
			Tags:    []string{"not_kv_pair"},
		}), ShouldErrLike, "bad tag, not a 'key:value' pair")
	})

	Convey("ValidateProtoMessage forbids default tags overwrite", t, func() {
		So(tm.ValidateProtoMessage(&messages.BuildbucketTask{
			Server:  strPtr("https://blah.com"),
			Bucket:  strPtr("bucket"),
			Builder: strPtr("builder"),
			Tags:    []string{"cron_job_id:blah"},
		}), ShouldErrLike, "tag \"cron_job_id\" is reserved")
	})
}

func TestFullFlow(t *testing.T) {
	Convey("LaunchTask and HandleNotification work", t, func(ctx C) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			resp := ""
			switch {
			case r.Method == "PUT" && r.URL.Path == "/_ah/api/buildbucket/v1/builds":
				// There's more stuff in actual response that we don't use.
				resp = `{
					"build": {
						"id": "9025781602559305888",
						"status": "STARTED",
						"url": "https://chromium-swarm-dev.appspot.com/user/task/2bdfb7404d18ac10"
					}
				}`
			case r.Method == "GET" && r.URL.Path == "/_ah/api/buildbucket/v1/builds/9025781602559305888":
				resp = `{
					"build": {
						"id": "9025781602559305888",
						"status": "COMPLETED",
						"result": "SUCCESS"
					}
				}`
			default:
				ctx.Printf("Unknown URL fetch - %s %s\n", r.Method, r.URL.Path)
				w.WriteHeader(400)
				return
			}
			w.WriteHeader(200)
			w.Write([]byte(resp))
		}))
		defer ts.Close()

		c := memory.Use(context.Background())
		mgr := TaskManager{}
		ctl := &tasktest.TestController{
			TaskMessage: &messages.BuildbucketTask{
				Server:  strPtr(ts.URL),
				Bucket:  strPtr("test-bucket"),
				Builder: strPtr("builder"),
				Tags:    []string{"a:b", "c:d"},
			},
			Client:       http.DefaultClient,
			SaveCallback: func() error { return nil },
			PrepareTopicCallback: func(publisher string) (string, string, error) {
				So(publisher, ShouldEqual, ts.URL)
				return "topic", "auth_token", nil
			},
		}

		// Launch.
		So(mgr.LaunchTask(c, ctl), ShouldBeNil)
		So(ctl.TaskState, ShouldResemble, task.State{
			Status:   task.StatusRunning,
			TaskData: []byte(`{"build_id":"9025781602559305888"}`),
			ViewURL:  "https://chromium-swarm-dev.appspot.com/user/task/2bdfb7404d18ac10",
		})

		// Process finish notification.
		So(mgr.HandleNotification(c, ctl, &pubsub.PubsubMessage{}), ShouldBeNil)
		So(ctl.TaskState.Status, ShouldEqual, task.StatusSucceeded)
	})
}

//////////////

func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int32 {
	j := int32(i)
	return &j
}
