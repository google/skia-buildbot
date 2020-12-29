package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/status/go/capacity"
	"go.skia.org/infra/status/go/incremental"
	status_mocks "go.skia.org/infra/status/go/mocks"
	ts_mocks "go.skia.org/infra/task_scheduler/go/mocks"
	"go.skia.org/infra/task_scheduler/go/types"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type mocks struct {
	capacityClient   *status_mocks.CapacityClient
	incrementalCache *status_mocks.IncrementalCache
	remoteDB         *ts_mocks.RemoteDB
}

func setupServerWithMockCapacityClient() (context.Context, mocks, *statusServerImpl) {
	mocks := mocks{capacityClient: &status_mocks.CapacityClient{}, incrementalCache: &status_mocks.IncrementalCache{}, remoteDB: &ts_mocks.RemoteDB{}}
	return context.Background(), mocks, newStatusServerImpl(
		mocks.incrementalCache,
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

func incrementalUpdate(ts time.Time) *incremental.Update {
	ret := &incremental.Update{}
	ret.Timestamp = ts

	// Make sure nontrivial data is transferred.
	// Branches.
	ret.BranchHeads = []*git.Branch{
		{Name: "b1", Head: "abc123"},
		{Name: "b2", Head: "def456"},
	}
	// Commits.
	ret.Commits = []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "abc123",
				Author:  "example@google.com",
				Subject: "a change",
			},
			Parents:   []string{"def456"},
			Timestamp: ts,
		},
	}
	// Tasks
	ret.Tasks = []*incremental.Task{
		{
			Commits:        []string{"abc123", "def456"},
			Name:           "Test_Android27_Metal",
			Id:             "999",
			Revision:       "abc123",
			Status:         "SUCCESS",
			SwarmingTaskId: "555",
		},
	}

	newTrue := func() *bool { b := true; return &b }
	// Comments
	ret.CommitComments = map[string][]*incremental.CommitComment{
		"abc123": {
			{
				CommitComment: types.CommitComment{
					Repo:          "skia",
					Revision:      "abc123",
					Timestamp:     ts,
					User:          "example@google.com",
					IgnoreFailure: true,
					Message:       "Commenting",
					Deleted:       newTrue(),
				},
				Id: "7",
			},
		},
	}
	ret.TaskSpecComments = map[string][]*incremental.TaskSpecComment{
		"My_Task_Spec": {
			{
				TaskSpecComment: types.TaskSpecComment{
					Repo:          "skia",
					Name:          "My_Task_Spec",
					Timestamp:     ts,
					User:          "example@google.com",
					IgnoreFailure: true,
					Message:       "Commenting",
					Deleted:       newTrue(),
					Flaky:         true,
				},
				Id: "8",
			},
		},
	}
	ret.TaskComments = map[string]map[string][]*incremental.TaskComment{
		"abc123": {
			"My_Task_Spec": {
				{
					TaskComment: types.TaskComment{
						Repo:      "skia",
						Revision:  "abc123",
						Name:      "My_Task_Spec",
						Timestamp: ts,
						User:      "example@google.com",
						Message:   "Commenting",
						Deleted:   newTrue(),
						TaskId:    "7777",
					},
					Id: "9",
				},
			},
		},
	}

	// StartOver, Urls
	ret.StartOver = newTrue()
	return ret
}

func TestGetIncrementalCommits_FreshLoad_ValidResponse(t *testing.T) {
	unittest.SmallTest(t)
	ctx, mocks, server := setupServerWithMockCapacityClient()
	// Use same times everywhere for ease of testing.
	ts := time.Now()
	tspb := timestamppb.New(ts)
	mocks.incrementalCache.On("GetAll", "skia", 10).Return(incrementalUpdate(ts), nil).Once()
	req := &GetIncrementalCommitsRequest{
		N:        10,
		Pod:      "mypod",
		RepoPath: "skia",
	}
	resp, err := server.GetIncrementalCommits(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, &GetIncrementalCommitsResponse{
		Metadata: &ResponseMetadata{
			StartOver: true,
			Pod:       "mypod",
			Timestamp: tspb,
		},
		Update: &IncrementalUpdate{
			Commits: []*LongCommit{
				{
					Hash:    "abc123",
					Author:  "example@google.com",
					Subject: "a change",
					Parents: []string{
						"def456",
					},
					Body:      "",
					Timestamp: tspb,
				},
			},
			BranchHeads: []*Branch{
				{
					Name: "b1",
					Head: "abc123",
				},
				{
					Name: "b2",
					Head: "def456",
				},
			},
			Tasks: []*Task{
				{
					Commits: []string{
						"abc123",
						"def456",
					},
					Name:           "Test_Android27_Metal",
					Id:             "999",
					Revision:       "abc123",
					Status:         "SUCCESS",
					SwarmingTaskId: "555",
				},
			},
			Comments: []*Comment{
				{
					Id:            "7",
					Repo:          "skia",
					Timestamp:     tspb,
					User:          "example@google.com",
					Message:       "Commenting",
					Deleted:       true,
					IgnoreFailure: true,
					Flaky:         false,
					TaskSpecName:  "",
					TaskId:        "",
					Commit:        "abc123",
				},
				{
					Id:            "8",
					Repo:          "skia",
					Timestamp:     tspb,
					User:          "example@google.com",
					Message:       "Commenting",
					Deleted:       true,
					IgnoreFailure: true,
					Flaky:         true,
					TaskSpecName:  "My_Task_Spec",
					TaskId:        "",
					Commit:        "",
				},
				{
					Id:            "9",
					Repo:          "skia",
					Timestamp:     tspb,
					User:          "example@google.com",
					Message:       "Commenting",
					Deleted:       true,
					IgnoreFailure: false,
					Flaky:         false,
					TaskSpecName:  "My_Task_Spec",
					TaskId:        "7777",
					Commit:        "abc123",
				},
			},
		},
	}, resp)
}

func TestGetIncrementalCommits_IncrementalCall_UsesCacheCorrectly(t *testing.T) {
	unittest.SmallTest(t)
	ctx, mocks, server := setupServerWithMockCapacityClient()
	// Without explicitly setting UTC, equal times expressed in different timezones (UTC and EST) are treated as not equal by mockery.
	ts := time.Now().UTC()
	mocks.incrementalCache.On("Get", "skia", ts, 10).Return(incrementalUpdate(ts), nil).Once()
	req := &GetIncrementalCommitsRequest{
		N:        10,
		Pod:      "mypod",
		RepoPath: "skia",
		From:     timestamppb.New(ts),
	}
	_, err := server.GetIncrementalCommits(ctx, req)
	require.NoError(t, err)
	// Here we're only testing that incrementalCache is correctly used (via the
	// mock call) the conversion logic etc is already tested in other methods.
}

func TestGetIncrementalCommits_RangeCall_UsesCacheCorrectly(t *testing.T) {
	unittest.SmallTest(t)
	ctx, mocks, server := setupServerWithMockCapacityClient()
	// Without explicitly setting UTC, equal times expressed in different timezones (UTC and EST) are treated as not equal by mockery.
	ts := time.Now().UTC()
	from := ts.Add(-60 * time.Minute)
	mocks.incrementalCache.On("GetRange", "skia", from, ts, 10).Return(incrementalUpdate(ts), nil).Once()
	req := &GetIncrementalCommitsRequest{
		N:        10,
		Pod:      "mypod",
		RepoPath: "skia",
		To:       timestamppb.New(ts),
		From:     timestamppb.New(from),
	}
	_, err := server.GetIncrementalCommits(ctx, req)
	require.NoError(t, err)
	// Here we're only testing that incrementalCache is correctly used (via the
	// mock call) the conversion logic etc is already tested in other methods.
}

func TestGetAutorollerStatuses_ValidResponse(t *testing.T) {
	unittest.SmallTest(t)
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
	unittest.SmallTest(t)
	ctx, mocks, server := setupServerWithMockCapacityClient()
	req := &AddCommentRequest{
		Repo:          "skia",
		Message:       "Adding a comment",
		Flaky:         false,
		IgnoreFailure: true,
		TaskSpec:      "Build-A-Thing",
	}
	mocks.remoteDB.On("PutTaskSpecComment", mock.MatchedBy(func(c *types.TaskSpecComment) bool {
		return c.Repo == "skia" && c.Name == "Build-A-Thing" && c.IgnoreFailure == true && c.Message == "Adding a comment"
	})).Return(nil).Once()
	mocks.incrementalCache.On("Update", testutils.AnyContext, false).Return(nil).Once()
	_, err := server.AddComment(ctx, req)
	require.NoError(t, err)
}

func TestAddComment_TaskComment_Added(t *testing.T) {
	unittest.SmallTest(t)
	ctx, mocks, server := setupServerWithMockCapacityClient()
	req := &AddCommentRequest{
		Repo:          "skia",
		Message:       "Adding a comment",
		Flaky:         false,
		IgnoreFailure: true,
		TaskId:        "abcdefg",
	}
	mocks.remoteDB.
		On("GetTaskById", "abcdefg").Return(&types.Task{
		TaskKey: types.TaskKey{
			RepoState: types.RepoState{
				Repo:     "taskRepo",
				Revision: "deadbeef",
			},
			Name: "Test-Something",
		},
		Id: "abcdefg",
	}, nil).Once().
		On("PutTaskComment", mock.MatchedBy(func(c *types.TaskComment) bool {
			// Note: values are taken from the returned task, not the request.
			return c.TaskId == "abcdefg" && c.Repo == "taskRepo" && c.Revision == "deadbeef" && c.Name == "Test-Something" && c.Message == "Adding a comment"
		})).Return(nil).Once()
	mocks.incrementalCache.On("Update", testutils.AnyContext, false).Return(nil).Once()
	_, err := server.AddComment(ctx, req)
	require.NoError(t, err)
}

func TestAddComment_CommitComment_Added(t *testing.T) {
	unittest.SmallTest(t)
	ctx, mocks, server := setupServerWithMockCapacityClient()
	req := &AddCommentRequest{
		Repo:          "skia",
		Message:       "Adding a comment",
		Flaky:         false,
		IgnoreFailure: true,
		Commit:        "abc",
	}
	mocks.remoteDB.On("PutCommitComment", mock.MatchedBy(func(c *types.CommitComment) bool {
		return c.Repo == "skia" && c.Revision == "abc" && c.IgnoreFailure == true && c.Message == "Adding a comment"
	})).Return(nil).Once()
	mocks.incrementalCache.On("Update", testutils.AnyContext, false).Return(nil).Once()
	_, err := server.AddComment(ctx, req)
	require.NoError(t, err)
}

func TestDeleteComment_TaskSpecComment_Deleted(t *testing.T) {
	unittest.SmallTest(t)
	ctx, mocks, server := setupServerWithMockCapacityClient()
	ts := time.Now().UTC()
	req := &DeleteCommentRequest{
		Repo:      "skia",
		Timestamp: timestamppb.New(ts),
		TaskSpec:  "Build-A-Thing",
	}
	mocks.remoteDB.On("DeleteTaskSpecComment", &types.TaskSpecComment{
		Repo:      "skia",
		Name:      "Build-A-Thing",
		Timestamp: ts,
	}).Return(nil).Once()
	mocks.incrementalCache.On("Update", testutils.AnyContext, false).Return(nil).Once()
	_, err := server.DeleteComment(ctx, req)
	require.NoError(t, err)
}

func TestDeleteComment_TaskComment_Deleted(t *testing.T) {
	unittest.SmallTest(t)
	ctx, mocks, server := setupServerWithMockCapacityClient()
	ts := time.Now().UTC()
	req := &DeleteCommentRequest{
		Repo:      "skia",
		Timestamp: timestamppb.New(ts),
		TaskId:    "abcdefg",
	}
	mocks.remoteDB.
		On("GetTaskById", "abcdefg").Return(&types.Task{
		TaskKey: types.TaskKey{
			RepoState: types.RepoState{
				Repo:     "taskRepo",
				Revision: "deadbeef",
			},
			Name: "Test-Something",
		},
		Id: "abcdefg",
	}, nil).Once().
		On("DeleteTaskComment", &types.TaskComment{
			// Note: values are taken from the returned task, not the request.
			Repo:      "taskRepo",
			Name:      "Test-Something",
			Revision:  "deadbeef",
			TaskId:    "abcdefg",
			Timestamp: ts,
		}).Return(nil).Once()
	mocks.incrementalCache.On("Update", testutils.AnyContext, false).Return(nil).Once()
	_, err := server.DeleteComment(ctx, req)
	require.NoError(t, err)
}

func TestDeleteComment_CommitComment_Deleted(t *testing.T) {
	unittest.SmallTest(t)
	ctx, mocks, server := setupServerWithMockCapacityClient()
	ts := time.Now().UTC()
	req := &DeleteCommentRequest{
		Repo:      "skia",
		Timestamp: timestamppb.New(ts),
		Commit:  "abc",
	}
	mocks.remoteDB.On("DeleteCommitComment", &types.CommitComment{
		Repo:      "skia",
		Revision:      "abc",
		Timestamp: ts,
	}).Return(nil).Once()
	mocks.incrementalCache.On("Update", testutils.AnyContext, false).Return(nil).Once()
	_, err := server.DeleteComment(ctx, req)
	require.NoError(t, err)
}

func TestConvertUpdate(t *testing.T) {
	unittest.SmallTest(t)
	ts := time.Now()
	require.Nil(t, nil)
	// Minimal update.
	src := &incremental.Update{Timestamp: ts}

	dest := ConvertUpdate(src, "mypod")
	expected := &GetIncrementalCommitsResponse{
		Metadata: &ResponseMetadata{
			Pod:       "mypod",
			Timestamp: timestamppb.New(ts),
		},
		Update: &IncrementalUpdate{}}
	require.Equal(t, expected, dest)

	// Make sure nontrivial data is transferred.
	// Branches.
	expected.Update.BranchHeads = []*Branch{
		{Name: "b1", Head: "abc123"},
		{Name: "b2", Head: "def456"},
	}
	// Commits.
	expected.Update.Commits = []*LongCommit{
		{
			Hash:      "abc123",
			Author:    "example@google.com",
			Subject:   "a change",
			Parents:   []string{"def456"},
			Timestamp: timestamppb.New(ts),
		},
	}
	// Tasks
	expected.Update.Tasks = []*Task{
		{
			Commits:        []string{"abc123", "def456"},
			Name:           "Test_Android27_Metal",
			Id:             "999",
			Revision:       "abc123",
			Status:         "SUCCESS",
			SwarmingTaskId: "555",
		},
	}
	// Comments are flattened in the resulting IncrementalUpdate
	expected.Update.Comments = []*Comment{
		{
			Repo:      "skia",
			Timestamp: timestamppb.New(ts),
			User:      "example@google.com",
			Message:   "Commenting",
			Deleted:   true,
			// Differentiated from other comments.
			Id:            "7",
			Commit:        "abc123",
			Flaky:         false,
			IgnoreFailure: true,
			TaskSpecName:  "",
			TaskId:        "",
		},
		{
			Repo:      "skia",
			Timestamp: timestamppb.New(ts),
			User:      "example@google.com",
			Message:   "Commenting",
			Deleted:   true,
			// Differentiated from other comments.
			Id:            "8",
			Commit:        "",
			Flaky:         true,
			IgnoreFailure: true,
			TaskSpecName:  "My_Task_Spec",
			TaskId:        "",
		},
		{
			Repo:      "skia",
			Timestamp: timestamppb.New(ts),
			User:      "example@google.com",
			Message:   "Commenting",
			Deleted:   true,
			// Differentiated from other comments.
			Id:            "9",
			Commit:        "abc123",
			Flaky:         false,
			IgnoreFailure: false,
			TaskSpecName:  "My_Task_Spec",
			TaskId:        "7777",
		},
	}

	// StartOver, Urls
	expected.Metadata.StartOver = true
	src = incrementalUpdate(ts)
	dest = ConvertUpdate(src, "mypod")
	require.Equal(t, expected, dest)
}

func TestGetBotUsage_MultipleDimensionSets_ValidResponse(t *testing.T) {
	unittest.SmallTest(t)
	ctx, mocks, server := setupServerWithMockCapacityClient()
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

// TODO(weston): Add tests for the remainder of status after adding testing helpers for iCache etc.
