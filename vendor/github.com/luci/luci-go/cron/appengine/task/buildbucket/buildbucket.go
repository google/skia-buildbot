// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package buildbucket implements cron task that runs Buildbucket jobs.
package buildbucket

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/api/pubsub/v1"

	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/common/api/buildbucket/buildbucket/v1"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/cron/appengine/messages"
	"github.com/luci/luci-go/cron/appengine/task"
	"github.com/luci/luci-go/cron/appengine/task/utils"
)

// TaskManager implements task.Manager interface for tasks defined with
// BuildbucketTask proto message.
type TaskManager struct {
}

// Name is part of Manager interface.
func (m TaskManager) Name() string {
	return "buildbucket"
}

// ProtoMessageType is part of Manager interface.
func (m TaskManager) ProtoMessageType() proto.Message {
	return (*messages.BuildbucketTask)(nil)
}

// ValidateProtoMessage is part of Manager interface.
func (m TaskManager) ValidateProtoMessage(msg proto.Message) error {
	cfg, ok := msg.(*messages.BuildbucketTask)
	if !ok {
		return fmt.Errorf("wrong type %T, expecting *messages.BuildbucketTask", msg)
	}

	// Validate 'server' field.
	server := cfg.GetServer()
	if server == "" {
		return fmt.Errorf("field 'server' is required")
	}
	u, err := url.Parse(server)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %s", server, err)
	}
	if !u.IsAbs() {
		return fmt.Errorf("not an absolute url: %q", server)
	}
	if u.Path != "" {
		return fmt.Errorf("not a host root url: %q", server)
	}

	// Bucket and builder fields are required.
	if cfg.GetBucket() == "" {
		return fmt.Errorf("'bucket' field is required")
	}
	if cfg.GetBuilder() == "" {
		return fmt.Errorf("'builder' field is required")
	}

	// Validate 'properties' and 'tags'.
	if err = utils.ValidateKVList("property", cfg.GetProperties(), ':'); err != nil {
		return err
	}
	if err = utils.ValidateKVList("tag", cfg.GetTags(), ':'); err != nil {
		return err
	}

	// Default tags can not be overridden.
	defTags := defaultTags(nil, nil, nil)
	for _, kv := range utils.UnpackKVList(cfg.GetTags(), ':') {
		if _, ok := defTags[kv.Key]; ok {
			return fmt.Errorf("tag %q is reserved", kv.Key)
		}
	}

	return nil
}

// defaultTags returns map with default set of tags.
//
// If context is nil, only keys are set.
func defaultTags(c context.Context, ctl task.Controller, cfg *messages.BuildbucketTask) map[string]string {
	if c != nil {
		return map[string]string{
			"builder":            cfg.GetBuilder(),
			"cron_invocation_id": fmt.Sprintf("%d", ctl.InvocationID()),
			"cron_job_id":        ctl.JobID(),
			"user_agent":         info.Get(c).AppID(),
		}
	}
	return map[string]string{
		"builder":            "",
		"cron_invocation_id": "",
		"cron_job_id":        "",
		"user_agent":         "",
	}
}

// taskData is saved in Invocation.TaskData field.
type taskData struct {
	BuildID int64 `json:"build_id,omitempty,string"`
}

// LaunchTask is part of Manager interface.
func (m TaskManager) LaunchTask(c context.Context, ctl task.Controller) error {
	// At this point config is already validated by ValidateProtoMessage.
	cfg := ctl.Task().(*messages.BuildbucketTask)

	// Default set of tags.
	tags := utils.KVListFromMap(defaultTags(c, ctl, cfg)).Pack(':')
	tags = append(tags, cfg.Tags...)

	// Prepare parameters blob.
	var params struct {
		BuilderName string            `json:"builder_name"`
		Properties  map[string]string `json:"properties"`
	}
	params.BuilderName = cfg.GetBuilder()
	params.Properties = make(map[string]string, len(cfg.Properties))
	for _, kv := range utils.UnpackKVList(cfg.Properties, ':') {
		params.Properties[kv.Key] = kv.Value
	}
	paramsJSON, err := json.Marshal(&params)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters JSON - %s", err)
	}

	// Make sure Buildbucket can publish PubSub messages, grab token that would
	// identify this invocation when receiving PubSub notifications.
	ctl.DebugLog("Preparing PubSub topic for %q", *cfg.Server)
	topic, authToken, err := ctl.PrepareTopic(*cfg.Server)
	if err != nil {
		ctl.DebugLog("Failed to prepare PubSub topic - %s", err)
		return err
	}
	ctl.DebugLog("PubSub topic is %q", topic)

	// Prepare the request.
	request := buildbucket.ApiPutRequestMessage{
		Bucket:            cfg.GetBucket(),
		ClientOperationId: fmt.Sprintf("%d", ctl.InvocationNonce()),
		ParametersJson:    string(paramsJSON),
		Tags:              tags,
		PubsubCallback: &buildbucket.ApiPubSubCallbackMessage{
			AuthToken: "...", // set a bit later, after printing this struct
			Topic:     topic,
		},
	}

	// Serialize for debug log without auth token.
	blob, err := json.MarshalIndent(&request, "", "  ")
	if err != nil {
		return err
	}
	ctl.DebugLog("Buildbucket request:\n%s", string(blob))
	request.PubsubCallback.AuthToken = authToken // can put the token now

	// Send the request.
	service, err := m.createBuildbucketService(c, ctl)
	if err != nil {
		return err
	}
	resp, err := service.Put(&request).Do()
	if err != nil {
		ctl.DebugLog("Failed to add buildbucket build - %s", err)
		return utils.WrapAPIError(err)
	}

	// Dump response in full to the debug log. It doesn't contain any secrets.
	blob, err = json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	ctl.DebugLog("Buildbucket response:\n%s", string(blob))

	if resp.Error != nil {
		ctl.DebugLog("Buildbucket returned error %q: %s", resp.Error.Reason, resp.Error.Message)
		return fmt.Errorf("buildbucket error %q - %s", resp.Error.Reason, resp.Error.Message)
	}
	if resp.Build == nil {
		ctl.DebugLog("Buildbucket response is not valid, missing 'build'")
		return fmt.Errorf("bad buildbucket response, no 'build' field")
	}

	// Save build id in invocation, will be used later when handling PubSub
	// notifications
	ctl.State().TaskData, err = json.Marshal(&taskData{BuildID: resp.Build.Id})
	if err != nil {
		return err
	}

	// Successfully launched.
	ctl.State().Status = task.StatusRunning
	ctl.State().ViewURL = resp.Build.Url
	ctl.DebugLog("Task URL: %s", ctl.State().ViewURL)

	// Maybe finished already? It can happen if we are retrying a call with same
	// ClientOperationId as a finished one.
	if resp.Build.Status == "COMPLETED" {
		m.handleBuildResult(c, ctl, resp.Build)
	}

	return nil
}

// AbortTask is part of Manager interface.
func (m TaskManager) AbortTask(c context.Context, ctl task.Controller) error {
	// TODO(vadimsh): Send the abort signal to Buildbucket.
	return nil
}

// HandleNotification is part of Manager interface.
func (m TaskManager) HandleNotification(c context.Context, ctl task.Controller, msg *pubsub.PubsubMessage) error {
	switch status := ctl.State().Status; {
	// This can happen if Buildbucket manages to send PubSub message before
	// LaunchTask finishes. Do not touch State or DebugLog to avoid collision with
	// still running LaunchTask when saving the invocation, it will only make the
	// matters worse.
	case status == task.StatusStarting:
		return errors.WrapTransient(errors.New("invocation is still starting, try again later"))
	case status != task.StatusRunning:
		return fmt.Errorf("unexpected invocation status %q, expecting %q", status, task.StatusRunning)
	}

	// Grab build ID from the blob generated in LaunchTask.
	taskData := taskData{}
	if err := json.Unmarshal(ctl.State().TaskData, &taskData); err != nil {
		ctl.State().Status = task.StatusFailed
		return fmt.Errorf("could not parse TaskData - %s", err)
	}

	// Fetch build result from Buildbucket.
	ctl.DebugLog("Received PubSub notification, asking Buildbucket for the task status")
	service, err := m.createBuildbucketService(c, ctl)
	if err != nil {
		return err
	}
	resp, err := service.Get(taskData.BuildID).Do()
	if err != nil {
		ctl.DebugLog("Failed to fetch buildbucket build - %s", err)
		err = utils.WrapAPIError(err)
		if !errors.IsTransient(err) {
			ctl.State().Status = task.StatusFailed
		}
		return err
	}

	// Dump response in full to the debug log. It doesn't contain any secrets.
	blob, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	ctl.DebugLog("Buildbucket response:\n%s", string(blob))

	// Give up (fail invocation) on unexpected fatal errors.
	if resp.Error != nil {
		ctl.DebugLog("Buildbucket returned error %q: %s", resp.Error.Reason, resp.Error.Message)
		ctl.State().Status = task.StatusFailed
		return fmt.Errorf("buildbucket error %q - %s", resp.Error.Reason, resp.Error.Message)
	}
	if resp.Build == nil {
		ctl.DebugLog("Buildbucket response is not valid, missing 'build'")
		ctl.State().Status = task.StatusFailed
		return fmt.Errorf("bad buildbucket response, no 'build' field")
	}

	m.handleBuildResult(c, ctl, resp.Build)
	return nil
}

// createBuildbucketService makes a configured Buildbucket API client.
func (m TaskManager) createBuildbucketService(c context.Context, ctl task.Controller) (*buildbucket.Service, error) {
	client, err := ctl.GetClient(time.Minute)
	if err != nil {
		return nil, err
	}
	service, err := buildbucket.New(client)
	if err != nil {
		return nil, err
	}
	cfg := ctl.Task().(*messages.BuildbucketTask)
	service.BasePath = *cfg.Server + "/_ah/api/buildbucket/v1/"
	return service, nil
}

// handleBuildResult processes buildbucket results message updating the state
// of the invocation.
func (m TaskManager) handleBuildResult(c context.Context, ctl task.Controller, r *buildbucket.ApiBuildMessage) {
	ctl.DebugLog(
		"Build %d: status %q, result %q, failure_reason %q, cancelation_reason %q",
		r.Id, r.Status, r.Result, r.FailureReason, r.CancelationReason)
	switch {
	case r.Status == "SCHEDULED" || r.Status == "STARTED":
		return // do nothing
	case r.Status == "COMPLETED" && r.Result == "SUCCESS":
		ctl.State().Status = task.StatusSucceeded
	default:
		ctl.State().Status = task.StatusFailed
	}
}
