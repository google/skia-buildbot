package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/status/go/capacity"
	"go.skia.org/infra/status/go/incremental"
	status_mocks "go.skia.org/infra/status/go/mocks"
	ts_mocks "go.skia.org/infra/task_scheduler/go/mocks"
	"go.skia.org/infra/task_scheduler/go/types"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func setupServerWithMockCapacityClient() (context.Context, *status_mocks.CapacityClient, *statusServerImpl) {
	mcc := &status_mocks.CapacityClient{}
	return context.Background(), mcc, newStatusServerImpl(
		&incremental.IncrementalCache{},
		&ts_mocks.RemoteDB{},
		mcc,
		func() *GetAutorollerStatusesResponse { return nil },
		func(string) (string, string, error) { return "", "", nil },
		0,
		0,
		"",
	)

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
	src.BranchHeads = []*git.Branch{
		{Name: "b1", Head: "abc123"},
		{Name: "b2", Head: "def456"},
	}
	expected.Update.BranchHeads = []*Branch{
		{Name: "b1", Head: "abc123"},
		{Name: "b2", Head: "def456"},
	}
	// Commits.
	src.Commits = []*vcsinfo.LongCommit{
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
	src.Tasks = []*incremental.Task{
		{
			Commits:        []string{"abc123", "def456"},
			Name:           "Test_Android27_Metal",
			Id:             "999",
			Revision:       "abc123",
			Status:         "SUCCESS",
			SwarmingTaskId: "555",
		},
	}
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
	newTrue := func() *bool { b := true; return &b }
	// Comments
	src.CommitComments = map[string][]*incremental.CommitComment{
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
	src.TaskSpecComments = map[string][]*incremental.TaskSpecComment{
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
	src.TaskComments = map[string]map[string][]*incremental.TaskComment{
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
	src.StartOver = newTrue()
	expected.Metadata.StartOver = true
	src.SwarmingUrl = "surl"
	src.TaskSchedulerUrl = "tsurl"
	dest = ConvertUpdate(src, "mypod")
	require.Equal(t, expected, dest)
}

func TestGetBotUsage_MultipleDimensionSets_ValidResponse(t *testing.T) {
	unittest.SmallTest(t)
	ctx, mcc, server := setupServerWithMockCapacityClient()
	mcc.On("CapacityMetrics").Return(map[string]capacity.BotConfig{
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
	assert.Equal(t, &GetBotUsageResponse{
		BotSets: []*BotSet{
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
		},
	}, resp)
}

// TODO(weston): Add tests for the remainder of status after adding testing helpers for iCache etc.
