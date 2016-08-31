// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package swarming

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
		So(tm.ValidateProtoMessage(&messages.SwarmingTask{
			Server:     strPtr("https://blah.com"),
			Command:    []string{"echo", "Hi!"},
			Env:        []string{"A=B", "C=D"},
			Dimensions: []string{"OS:Linux"},
			Tags:       []string{"a:b", "c:d"},
			Priority:   intPtr(50),
		}), ShouldBeNil)
	})

	Convey("ValidateProtoMessage passes good minimal msg", t, func() {
		So(tm.ValidateProtoMessage(&messages.SwarmingTask{
			Server:  strPtr("https://blah.com"),
			Command: []string{"echo", "Hi!"},
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
			return tm.ValidateProtoMessage(&messages.SwarmingTask{
				Server:  &url,
				Command: []string{"echo", "Hi!"},
			})
		}
		So(call(""), ShouldErrLike, "field 'server' is required")
		So(call("%%%%"), ShouldErrLike, "invalid URL")
		So(call("/abc"), ShouldErrLike, "not an absolute url")
		So(call("https://host/not-root"), ShouldErrLike, "not a host root url")
	})

	Convey("ValidateProtoMessage validates environ", t, func() {
		So(tm.ValidateProtoMessage(&messages.SwarmingTask{
			Server:  strPtr("https://blah.com"),
			Command: []string{"echo", "Hi!"},
			Env:     []string{"not_kv_pair"},
		}), ShouldErrLike, "bad environment variable, not a 'key=value' pair")
	})

	Convey("ValidateProtoMessage validates dimensions", t, func() {
		So(tm.ValidateProtoMessage(&messages.SwarmingTask{
			Server:     strPtr("https://blah.com"),
			Command:    []string{"echo", "Hi!"},
			Dimensions: []string{"not_kv_pair"},
		}), ShouldErrLike, "bad dimension, not a 'key:value' pair")
	})

	Convey("ValidateProtoMessage validates tags", t, func() {
		So(tm.ValidateProtoMessage(&messages.SwarmingTask{
			Server:  strPtr("https://blah.com"),
			Command: []string{"echo", "Hi!"},
			Tags:    []string{"not_kv_pair"},
		}), ShouldErrLike, "bad tag, not a 'key:value' pair")
	})

	Convey("ValidateProtoMessage forbids default tags overwrite", t, func() {
		So(tm.ValidateProtoMessage(&messages.SwarmingTask{
			Server:  strPtr("https://blah.com"),
			Command: []string{"echo", "Hi!"},
			Tags:    []string{"cron_job_id:blah"},
		}), ShouldErrLike, "tag \"cron_job_id\" is reserved")
	})

	Convey("ValidateProtoMessage validates priority", t, func() {
		call := func(priority int32) error {
			return tm.ValidateProtoMessage(&messages.SwarmingTask{
				Server:   strPtr("https://blah.com"),
				Command:  []string{"echo", "Hi!"},
				Priority: &priority,
			})
		}
		So(call(-1), ShouldErrLike, "bad priority")
		So(call(256), ShouldErrLike, "bad priority")
	})

	Convey("ValidateProtoMessage accepts input_ref or command, not both", t, func() {
		So(tm.ValidateProtoMessage(&messages.SwarmingTask{
			Server: strPtr("https://blah.com"),
		}), ShouldErrLike, "one of 'command' or 'isolated_ref' is required")

		So(tm.ValidateProtoMessage(&messages.SwarmingTask{
			Server:      strPtr("https://blah.com"),
			Command:     []string{"echo", "Hi!"},
			IsolatedRef: &messages.SwarmingTask_IsolatedRef{},
		}), ShouldErrLike, "only one of 'command' or 'isolated_ref' must be specified")
	})
}

func TestFullFlow(t *testing.T) {
	Convey("LaunchTask and HandleNotification work", t, func(ctx C) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			resp := ""
			switch r.URL.Path {
			case "/_ah/api/swarming/v1/tasks/new":
				resp = `{"task_id": "task_id"}`
			case "/_ah/api/swarming/v1/task/task_id/result":
				resp = `{"state":"COMPLETED"}`
			default:
				ctx.Printf("Unknown URL fetch - %s\n", r.URL.Path)
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
			TaskMessage: &messages.SwarmingTask{
				Server: strPtr(ts.URL),
				IsolatedRef: &messages.SwarmingTask_IsolatedRef{
					Isolated:       strPtr("abcdef"),
					IsolatedServer: strPtr("https://isolated-server"),
					Namespace:      strPtr("default-gzip"),
				},
				Env:        []string{"A=B", "C=D"},
				Dimensions: []string{"OS:Linux"},
				Tags:       []string{"a:b", "c:d"},
				Priority:   intPtr(50),
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
			TaskData: []byte(`{"swarming_task_id":"task_id"}`),
			ViewURL:  ts.URL + "/user/task/task_id",
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
