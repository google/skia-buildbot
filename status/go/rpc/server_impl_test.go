// Copyright 2020 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/status/go/capacity"
	status_mocks "go.skia.org/infra/status/go/mocks"
	ts_mocks "go.skia.org/infra/task_scheduler/go/mocks"
	"go.skia.org/infra/task_scheduler/go/types"
)

const testUser = "test_user@example.com"

type mocks struct {
	capacityClient *status_mocks.CapacityClient
	remoteDB       *ts_mocks.RemoteDB
}

func (m mocks) AssertExpectations(t *testing.T) {
	m.capacityClient.AssertExpectations(t)
	m.remoteDB.AssertExpectations(t)
}

func setupServerWithMockCapacityClient() (context.Context, mocks, *statusServerImpl) {
	mocks := mocks{capacityClient: &status_mocks.CapacityClient{}, remoteDB: &ts_mocks.RemoteDB{}}
	ctx := alogin.FakeStatus(context.Background(), &alogin.Status{
		EMail: testUser,
		Roles: roles.Roles{roles.Editor},
	})
	return ctx, mocks, newStatusServerImpl(
		mocks.remoteDB,
		mocks.capacityClient,
		func() *GetAutorollerStatusesResponse {
			return &GetAutorollerStatusesResponse{
				Rollers: []*AutorollerStatus{
					{
						Name:           "android",
						CurrentRollRev: "def",
						LastRollRev:    "abc",
						Mode:           "running",
						NumFailed:      0,
						NumBehind:      3,
						Url:            "https://example.com/skiatoandroid",
					},
					{
						Name:           "chrome",
						CurrentRollRev: "def",
						LastRollRev:    "123",
						Mode:           "paused",
						NumFailed:      3,
						NumBehind:      17,
						Url:            "https://example.com/skiatochrome",
					},
				},
			}
		},
		func(string) (string, string, error) { return "", "skia", nil },
		100,
		35,
		"mypod",
	)
}

func TestGetAutorollerStatuses_ValidResponse(t *testing.T) {
	ctx, _, server := setupServerWithMockCapacityClient()
	resp, err := server.GetAutorollerStatuses(ctx, &GetAutorollerStatusesRequest{})
	require.NoError(t, err)
	assert.Equal(t, &GetAutorollerStatusesResponse{
		Rollers: []*AutorollerStatus{
			{
				Name:           "android",
				CurrentRollRev: "def",
				LastRollRev:    "abc",
				Mode:           "running",
				NumFailed:      0,
				NumBehind:      3,
				Url:            "https://example.com/skiatoandroid",
			},
			{
				Name:           "chrome",
				CurrentRollRev: "def",
				LastRollRev:    "123",
				Mode:           "paused",
				NumFailed:      3,
				NumBehind:      17,
				Url:            "https://example.com/skiatochrome",
			},
		},
	}, resp)
}

func TestAddComment_TaskSpecComment_Added(t *testing.T) {
	ctx, mocks, server := setupServerWithMockCapacityClient()
	defer mocks.AssertExpectations(t)
	matchComment := mock.MatchedBy(func(c *types.TaskSpecComment) bool {
		assert.Equal(t, "skia", c.Repo)
		assert.Equal(t, "Build-A-Thing", c.Name)
		assert.Equal(t, true, c.IgnoreFailure)
		assert.Equal(t, "Adding a comment", c.Message)
		return true
	})
	mocks.remoteDB.On("PutTaskSpecComment", testutils.AnyContext, matchComment).Return(nil).Once()

	req := &AddCommentRequest{
		Repo:          "skia",
		Message:       "Adding a comment",
		Flaky:         false,
		IgnoreFailure: true,
		TaskSpec:      "Build-A-Thing",
	}
	_, err := server.AddComment(ctx, req)
	require.NoError(t, err)
}

func TestAddComment_NotLoggedIn_ErrorReturned(t *testing.T) {
	_, mocks, server := setupServerWithMockCapacityClient()
	defer mocks.AssertExpectations(t)
	ctx := alogin.FakeStatus(context.Background(), &alogin.Status{})
	req := &AddCommentRequest{
		Repo:          "skia",
		Message:       "Adding a comment",
		Flaky:         false,
		IgnoreFailure: true,
		TaskSpec:      "Build-A-Thing",
	}
	_, err := server.AddComment(ctx, req)
	require.Error(t, err)
}

func TestAddComment_TaskComment_Added(t *testing.T) {
	ctx, mocks, server := setupServerWithMockCapacityClient()
	defer mocks.AssertExpectations(t)
	mocks.remoteDB.
		On("GetTaskById", testutils.AnyContext, "abcdefg").Return(&types.Task{
		TaskKey: types.TaskKey{
			RepoState: types.RepoState{
				Repo:     "taskRepo",
				Revision: "deadbeef",
			},
			Name: "Test-Something",
		},
		Id: "abcdefg",
	}, nil).Once()
	matchComment := mock.MatchedBy(func(c *types.TaskComment) bool {
		// Note: values are taken from the returned task, not the request.
		assert.Equal(t, "abcdefg", c.TaskId)
		assert.Equal(t, "deadbeef", c.Revision)
		assert.Equal(t, "taskRepo", c.Repo)
		assert.Equal(t, "Test-Something", c.Name)
		assert.Equal(t, "Adding a comment", c.Message)
		return true
	})
	mocks.remoteDB.On("PutTaskComment", testutils.AnyContext, matchComment).Return(nil).Once()

	req := &AddCommentRequest{
		Repo:          "skia",
		Message:       "Adding a comment",
		Flaky:         false,
		IgnoreFailure: true,
		TaskId:        "abcdefg",
	}
	_, err := server.AddComment(ctx, req)
	require.NoError(t, err)
}

func TestAddComment_CommitComment_Added(t *testing.T) {
	ctx, mocks, server := setupServerWithMockCapacityClient()
	defer mocks.AssertExpectations(t)
	matchComment := mock.MatchedBy(func(c *types.CommitComment) bool {
		// Note: values are taken from the returned task, not the request.
		assert.Equal(t, "abc", c.Revision)
		assert.Equal(t, "skia", c.Repo)
		assert.Equal(t, true, c.IgnoreFailure)
		assert.Equal(t, "Adding a comment", c.Message)
		return true
	})
	mocks.remoteDB.On("PutCommitComment", testutils.AnyContext, matchComment).Return(nil).Once()

	req := &AddCommentRequest{
		Repo:          "skia",
		Message:       "Adding a comment",
		Flaky:         false,
		IgnoreFailure: true,
		Commit:        "abc",
	}
	_, err := server.AddComment(ctx, req)
	require.NoError(t, err)
}

func TestDeleteComment_TaskSpecComment_Deleted(t *testing.T) {
	ctx, mocks, server := setupServerWithMockCapacityClient()
	defer mocks.AssertExpectations(t)
	ts := time.Now().UTC()
	mocks.remoteDB.On("DeleteTaskSpecComment", testutils.AnyContext, &types.TaskSpecComment{
		Repo:      "skia",
		Name:      "Build-A-Thing",
		Timestamp: ts,
	}).Return(nil).Once()

	req := &DeleteCommentRequest{
		Repo:      "skia",
		Timestamp: timestamppb.New(ts),
		TaskSpec:  "Build-A-Thing",
	}
	_, err := server.DeleteComment(ctx, req)
	require.NoError(t, err)
}

func TestDeleteComment_TaskComment_Deleted(t *testing.T) {
	ctx, mocks, server := setupServerWithMockCapacityClient()
	defer mocks.AssertExpectations(t)
	ts := time.Now().UTC()
	mocks.remoteDB.On("GetTaskById", testutils.AnyContext, "abcdefg").Return(&types.Task{
		TaskKey: types.TaskKey{
			RepoState: types.RepoState{
				Repo:     "taskRepo",
				Revision: "deadbeef",
			},
			Name: "Test-Something",
		},
		Id: "abcdefg",
	}, nil).Once()
	mocks.remoteDB.On("DeleteTaskComment", testutils.AnyContext, &types.TaskComment{
		// Note: values are taken from the returned task, not the request.
		Repo:      "taskRepo",
		Name:      "Test-Something",
		Revision:  "deadbeef",
		TaskId:    "abcdefg",
		Timestamp: ts,
	}).Return(nil).Once()

	req := &DeleteCommentRequest{
		Repo:      "skia",
		Timestamp: timestamppb.New(ts),
		TaskId:    "abcdefg",
	}
	_, err := server.DeleteComment(ctx, req)
	require.NoError(t, err)
}

func TestDeleteComment_CommitComment_Deleted(t *testing.T) {
	ctx, mocks, server := setupServerWithMockCapacityClient()
	defer mocks.AssertExpectations(t)
	ts := time.Now().UTC()
	mocks.remoteDB.On("DeleteCommitComment", testutils.AnyContext, &types.CommitComment{
		Repo:      "skia",
		Revision:  "abc",
		Timestamp: ts,
	}).Return(nil).Once()

	req := &DeleteCommentRequest{
		Repo:      "skia",
		Timestamp: timestamppb.New(ts),
		Commit:    "abc",
	}
	_, err := server.DeleteComment(ctx, req)
	require.NoError(t, err)
}

func TestGetBotUsage_MultipleDimensionSets_ValidResponse(t *testing.T) {
	ctx, mocks, server := setupServerWithMockCapacityClient()
	defer mocks.AssertExpectations(t)
	mocks.capacityClient.On("CapacityMetrics").Return(map[string]capacity.BotConfig{
		"keyIgnored0": {
			Dimensions: []string{
				// Dimensions can include colons.
				"gpu:8086:3ea5-26.20.100.7463",
				"os:Windows-10-18363",
				"pool:Skia",
			},
			TaskAverageDurations: []capacity.TaskDuration{
				{
					AverageDuration: 5 * time.Minute,
					OnCQ:            true,
				},
				{
					AverageDuration: 13 * time.Minute,
					OnCQ:            false,
				},
			},
			Bots: map[string]bool{"task1": true, "task2": true, "task3": true},
		},
		"keyIgnored1": {
			Dimensions: []string{
				"cpu:widget5",
				"os:Android",
				"device:Pixel2",
				"pool:Skia",
			},
			TaskAverageDurations: []capacity.TaskDuration{
				{
					AverageDuration: 5 * time.Minute,
					OnCQ:            false,
				},
			},
			Bots: map[string]bool{"task4": true},
		},
	}).Once()
	resp, err := server.GetBotUsage(ctx, &GetBotUsageRequest{})
	require.NoError(t, err)
	assert.ElementsMatch(t, []*BotSet{
		{
			Dimensions: map[string]string{
				"gpu":  "8086:3ea5-26.20.100.7463",
				"os":   "Windows-10-18363",
				"pool": "Skia",
			},
			BotCount:    3,
			CqTasks:     1,
			MsPerCq:     300000,
			TotalTasks:  2,
			MsPerCommit: 1080000,
		}, {
			Dimensions: map[string]string{
				"cpu":    "widget5",
				"device": "Pixel2",
				"os":     "Android",
				"pool":   "Skia",
			},
			BotCount:    1,
			CqTasks:     0,
			MsPerCq:     0,
			TotalTasks:  1,
			MsPerCommit: 300000,
		},
	}, resp.BotSets)
}
