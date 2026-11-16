// Copyright 2020 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rpc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/status/go/capacity"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Generate Go structs and Typescript classes from protobuf definitions.
//go:generate bazelisk run --config=mayberemote //:protoc -- --go_opt=paths=source_relative --twirp_out=. --go_out=. status.proto
//go:generate mv ./go.skia.org/infra/status/go/rpc/status.twirp.go ./status.twirp.go
//go:generate rm -rf ./go.skia.org
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w status.pb.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w status.twirp.go
//go:generate bazelisk run --config=mayberemote //:protoc -- --twirp_typescript_out=../../modules/rpc status.proto

type statusServerImpl struct {
	taskDb                db.RemoteDB
	capacityClient        capacity.CapacityClient
	getAutorollerStatuses func() *GetAutorollerStatusesResponse
	getRepo               func(string) (string, string, error)
	maxCommitsToLoad      int
	defaultCommitsToLoad  int
	podID                 string
}

func (s *statusServerImpl) AddComment(ctx context.Context, req *AddCommentRequest) (*AddCommentResponse, error) {
	defer metrics2.FuncTimer().Stop()
	status := alogin.GetStatus(ctx)
	if !status.Roles.Has(roles.Editor) {
		return nil, twirpErrorFromIntermediary(http.StatusForbidden, "You are not logged in as an editor", "")
	}

	_, repoURL, err := s.getRepo(req.Repo)
	if err != nil {
		return nil, err
	}
	message := req.Message
	now := time.Now().UTC()

	if req.GetTaskId() != "" {
		task, err := s.taskDb.GetTaskById(ctx, req.GetTaskId())
		if err != nil {
			return nil, fmt.Errorf("failed to obtain task details: %v", err)
		}
		c := types.TaskComment{
			Repo:      task.Repo,
			Revision:  task.Revision,
			Name:      task.Name,
			Timestamp: now,
			TaskId:    task.Id,
			User:      status.EMail.String(),
			Message:   message,
		}
		if err := s.taskDb.PutTaskComment(ctx, &c); err != nil {
			return nil, fmt.Errorf("failed to add task comment: %v", err)
		}
	} else if req.GetTaskSpec() != "" {
		c := types.TaskSpecComment{
			Repo:          repoURL,
			Name:          req.GetTaskSpec(),
			Timestamp:     now,
			User:          status.EMail.String(),
			Flaky:         req.Flaky,
			IgnoreFailure: req.IgnoreFailure,
			Message:       req.Message,
		}
		if err := s.taskDb.PutTaskSpecComment(ctx, &c); err != nil {
			return nil, fmt.Errorf("failed to add task spec  comment: %v", err)
		}
	} else if req.GetCommit() != "" {
		c := types.CommitComment{
			Repo:          repoURL,
			Revision:      req.GetCommit(),
			Timestamp:     now,
			User:          status.EMail.String(),
			IgnoreFailure: req.IgnoreFailure,
			Message:       req.Message,
		}
		if err := s.taskDb.PutCommitComment(ctx, &c); err != nil {
			return nil, fmt.Errorf("failed to add commit comment: %v", err)
		}
	} else {
		return nil, fmt.Errorf("no Task ID, Task Spec, or Commit given")
	}
	return &AddCommentResponse{Timestamp: timestamppb.New(now)}, nil
}

func (s *statusServerImpl) DeleteComment(ctx context.Context, req *DeleteCommentRequest) (*DeleteCommentResponse, error) {
	defer metrics2.FuncTimer().Stop()
	status := alogin.GetStatus(ctx)
	if !status.Roles.Has(roles.Editor) {
		return nil, twirpErrorFromIntermediary(http.StatusForbidden, "You are not logged in as an editor", "")
	}

	_, repoURL, err := s.getRepo(req.Repo)
	if err != nil {
		return nil, err
	}
	timestamp := req.Timestamp.AsTime()
	if timestamp.IsZero() {
		return nil, fmt.Errorf("no timestamp (comment ID) given")
	}
	commit := req.Commit
	taskSpec := req.TaskSpec
	taskID := req.TaskId

	if taskID != "" {
		// This references a comment on an individual task.
		task, err := s.taskDb.GetTaskById(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain task details: %v", err)
		}
		c := &types.TaskComment{
			Repo:      task.Repo,
			Revision:  task.Revision,
			Name:      task.Name,
			Timestamp: timestamp,
			TaskId:    task.Id,
		}

		if err := s.taskDb.DeleteTaskComment(ctx, c); err != nil {
			return nil, fmt.Errorf("failed to delete comment: %v", err)
		}
	} else if taskSpec != "" {
		// This references a comment on a Task Spec.
		c := types.TaskSpecComment{
			Repo:      repoURL,
			Name:      taskSpec,
			Timestamp: timestamp,
		}
		if err := s.taskDb.DeleteTaskSpecComment(ctx, &c); err != nil {
			return nil, fmt.Errorf("failed to delete comment: %v", err)
		}
	} else if commit != "" {
		// This references a comment on a commit.
		c := types.CommitComment{
			Repo:      repoURL,
			Revision:  commit,
			Timestamp: timestamp,
		}
		if err := s.taskDb.DeleteCommitComment(ctx, &c); err != nil {
			return nil, fmt.Errorf("failed to delete comment: %v", err)
		}

	} else {
		return nil, fmt.Errorf("no Task ID, Task Spec, or Commit given")
	}
	return &DeleteCommentResponse{}, nil
}

func (s *statusServerImpl) GetAutorollerStatuses(ctx context.Context, req *GetAutorollerStatusesRequest) (*GetAutorollerStatusesResponse, error) {
	return s.getAutorollerStatuses(), nil
}

func (s *statusServerImpl) GetBotUsage(ctx context.Context, req *GetBotUsageRequest) (*GetBotUsageResponse, error) {
	rv := GetBotUsageResponse{}
	for _, botconfig := range s.capacityClient.CapacityMetrics() {
		dims := make(map[string]string, len(botconfig.Dimensions))
		for _, dim := range botconfig.Dimensions {
			split := strings.SplitN(dim, ":", 2)
			if len(split) > 0 {
				// Handles empty dimensions.
				dims[split[0]] = dim[len(split[0])+1:]
			}
		}
		var totalTasks, cqTasks int32
		var taskTimeMs, cqTimeMs int64
		for _, task := range botconfig.TaskAverageDurations {
			timeSpent := task.AverageDuration.Milliseconds()
			taskTimeMs += timeSpent
			totalTasks++
			if task.OnCQ {
				cqTimeMs += timeSpent
				cqTasks++
			}
		}
		numBots := len(botconfig.Bots)
		rv.BotSets = append(rv.BotSets, &BotSet{
			Dimensions:  dims,
			CqTasks:     cqTasks,
			MsPerCq:     cqTimeMs,
			TotalTasks:  totalTasks,
			MsPerCommit: taskTimeMs,
			BotCount:    int32(numBots),
		})
	}
	return &rv, nil
}

// newStatusServerImpl creates and returns a statusServerImpl instance.
func newStatusServerImpl(taskDb db.RemoteDB, capacityClient capacity.CapacityClient, getAutorollerStatuses func() *GetAutorollerStatusesResponse, getRepo func(string) (string, string, error), maxCommitsToLoad, defaultCommitsToLoad int, podID string) *statusServerImpl {
	return &statusServerImpl{
		taskDb:                taskDb,
		capacityClient:        capacityClient,
		getAutorollerStatuses: getAutorollerStatuses,
		getRepo:               getRepo,
		maxCommitsToLoad:      maxCommitsToLoad,
		defaultCommitsToLoad:  defaultCommitsToLoad,
		podID:                 podID}
}

// NewStatusServer creates and returns a Twirp HTTP Server.
func NewStatusServer(
	taskDb db.RemoteDB,
	capacityClient capacity.CapacityClient,
	getAutorollerStatuses func() *GetAutorollerStatusesResponse,
	getRepo func(string) (string, string, error),
	maxCommitsToLoad int,
	defaultCommitsToLoad int,
	podID string) http.Handler {
	return NewStatusServiceServer(newStatusServerImpl(
		taskDb,
		capacityClient,
		getAutorollerStatuses,
		getRepo,
		maxCommitsToLoad,
		defaultCommitsToLoad,
		podID), nil)
}
