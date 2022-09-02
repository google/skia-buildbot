package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/status/go/capacity"
	"go.skia.org/infra/status/go/incremental"
	status_mocks "go.skia.org/infra/status/go/mocks"
	ts_mocks "go.skia.org/infra/task_scheduler/go/mocks"
	"go.skia.org/infra/task_scheduler/go/types"
)

const testUser = "test_user@example.com"

type mocks struct {
	capacityClient   *status_mocks.CapacityClient
	incrementalCache *status_mocks.IncrementalCache
	remoteDB         *ts_mocks.RemoteDB
}

func (m mocks) AssertExpectations(t *testing.T) {
	m.capacityClient.AssertExpectations(t)
	m.incrementalCache.AssertExpectations(t)
	m.remoteDB.AssertExpectations(t)
}

func setupServerWithMockCapacityClient() (context.Context, mocks, *statusServerImpl) {
	mocks := mocks{capacityClient: &status_mocks.CapacityClient{}, incrementalCache: &status_mocks.IncrementalCache{}, remoteDB: &ts_mocks.RemoteDB{}}
	ctx := login.FakeLoggedInAs(context.Background(), testUser)
	allow := allowed.NewAllowedFromList([]string{testUser})
	login.FakeAllows(allow, allow, allow)
	return ctx, mocks, newStatusServerImpl(
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

// incrementalUpdate returns a filled incremental.Update. The update contains a single commit
// 'abc123', and a task that covers it and its parent 'def456'. There is a branch label pointing to
// each of those commits, and one each of task comment, commit comment, and taskspec comment.
func incrementalUpdate(ts time.Time, startover bool) *incremental.Update {
	newTrue := func() *bool { b := true; return &b }
	ret := &incremental.Update{
		Timestamp: ts,
		// Make sure nontrivial data is transferred.
		// Branches.
		BranchHeads: []*git.Branch{
			{Name: "b1", Head: "abc123"},
			{Name: "b2", Head: "def456"},
		},
		// Commits.
		Commits: []*vcsinfo.LongCommit{
			{
				ShortCommit: &vcsinfo.ShortCommit{
					Hash:    "abc123",
					Author:  "example@google.com",
					Subject: "a change at head",
				},
				Parents:   []string{"def456"},
				Timestamp: ts,
			},
		},
		// Tasks
		Tasks: []*incremental.Task{
			{
				Commits:        []string{"abc123", "def456"},
				Name:           "Test_Android27_Metal",
				Id:             "999",
				Revision:       "abc123",
				Status:         "SUCCESS",
				SwarmingTaskId: "555",
			},
		},
		// Comments
		CommitComments: map[string][]*incremental.CommitComment{
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
		},
		TaskSpecComments: map[string][]*incremental.TaskSpecComment{
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
		},
		TaskComments: map[string]map[string][]*incremental.TaskComment{
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
		},

		// StartOver, Urls
		StartOver: newTrue(),
	}
	if !startover {
		f := false
		ret.StartOver = &f
	}
	return ret
}

// matchTime produces a matcher function to be used by mock.MatchedBy() that matches a time.Time,
// adjusting for timezone.
func matchTime(t *testing.T, ts time.Time) interface{} {
	return mock.MatchedBy(func(arg time.Time) bool {
		assert.True(t, ts.Equal(arg))
		return true
	})
}

func TestGetIncrementalCommits_FreshLoad_ValidResponse(t *testing.T) {
	ctx, mocks, server := setupServerWithMockCapacityClient()
	defer mocks.AssertExpectations(t)
	// Use same times everywhere for ease of testing.
	ts := time.Now()
	tspb := timestamppb.New(ts)
	mocks.incrementalCache.On("GetAll", "skia", 10).Return(incrementalUpdate(ts, true), nil).Once()
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
					Subject: "a change at head",
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
	ctx, mocks, server := setupServerWithMockCapacityClient()
	defer mocks.AssertExpectations(t)
	ts := time.Now()
	mocks.incrementalCache.On("Get", "skia", matchTime(t, ts), 10).Return(incrementalUpdate(ts, false), nil).Once()
	req := &GetIncrementalCommitsRequest{
		N:        10,
		Pod:      "mypod",
		RepoPath: "skia",
		From:     timestamppb.New(ts),
	}
	result, err := server.GetIncrementalCommits(ctx, req)
	require.NoError(t, err)
	// Here we're only testing that incrementalCache is correctly used, the conversion logic is
	// already tested in other methods, so we just do a spot check here.
	assert.Equal(t, 2, len(result.Update.BranchHeads))
}

func TestGetIncrementalCommits_RangeCall_UsesCacheCorrectly(t *testing.T) {
	ctx, mocks, server := setupServerWithMockCapacityClient()
	defer mocks.AssertExpectations(t)
	ts := time.Now()
	from := ts.Add(-60 * time.Minute)
	matchTo := matchTime(t, ts)
	matchFrom := matchTime(t, from)
	mocks.incrementalCache.On("GetRange", "skia", matchFrom, matchTo, 10).Return(incrementalUpdate(ts, false), nil).Once()
	req := &GetIncrementalCommitsRequest{
		N:        10,
		Pod:      "mypod",
		RepoPath: "skia",
		To:       timestamppb.New(ts),
		From:     timestamppb.New(from),
	}
	result, err := server.GetIncrementalCommits(ctx, req)
	require.NoError(t, err)
	// Here we're only testing that incrementalCache is correctly used, the conversion logic is
	// already tested in other methods, so we just do a spot check here.
	assert.Equal(t, 2, len(result.Update.BranchHeads))
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
	mocks.incrementalCache.On("Update", testutils.AnyContext, false).Return(nil).Once()

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
	ctx := login.FakeLoggedInAs(context.Background(), "not_on_the_list@example.com")
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
	mocks.incrementalCache.On("Update", testutils.AnyContext, false).Return(nil).Once()

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
	mocks.incrementalCache.On("Update", testutils.AnyContext, false).Return(nil).Once()

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
	mocks.incrementalCache.On("Update", testutils.AnyContext, false).Return(nil).Once()

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
	mocks.incrementalCache.On("Update", testutils.AnyContext, false).Return(nil).Once()

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
	mocks.incrementalCache.On("Update", testutils.AnyContext, false).Return(nil).Once()

	req := &DeleteCommentRequest{
		Repo:      "skia",
		Timestamp: timestamppb.New(ts),
		Commit:    "abc",
	}
	_, err := server.DeleteComment(ctx, req)
	require.NoError(t, err)
}

func TestConvertUpdate_NontrivialUpdate_Success(t *testing.T) {
	ts := time.Now()
	// Minimal update.
	src := &incremental.Update{Timestamp: ts}
	result := ConvertUpdate(src, "mypod")
	require.Equal(t, &GetIncrementalCommitsResponse{
		Metadata: &ResponseMetadata{
			Pod:       "mypod",
			Timestamp: timestamppb.New(ts),
			StartOver: false,
		},
		Update: &IncrementalUpdate{},
	}, result)

	// Make sure nontrivial data is converted.
	src = incrementalUpdate(ts, true)
	result = ConvertUpdate(src, "mypod")
	assert.Equal(t, &GetIncrementalCommitsResponse{
		Metadata: &ResponseMetadata{
			Pod:       "mypod",
			Timestamp: timestamppb.New(ts),
			StartOver: true,
		},
		Update: &IncrementalUpdate{
			BranchHeads: []*Branch{
				{Name: "b1", Head: "abc123"},
				{Name: "b2", Head: "def456"},
			},
			Commits: []*LongCommit{
				{
					Hash:      "abc123",
					Author:    "example@google.com",
					Subject:   "a change at head",
					Parents:   []string{"def456"},
					Timestamp: timestamppb.New(ts),
				},
			},
			Tasks: []*Task{
				{
					Commits:        []string{"abc123", "def456"},
					Name:           "Test_Android27_Metal",
					Id:             "999",
					Revision:       "abc123",
					Status:         "SUCCESS",
					SwarmingTaskId: "555",
				},
			},
			// Comments are flattened in the resulting IncrementalUpdate
			Comments: []*Comment{
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
			},
		},
	}, result)
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
