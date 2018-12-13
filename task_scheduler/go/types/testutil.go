package types

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/common"
)

const DEFAULT_TEST_REPO = "go-on-now.git"

func MakeTestTask(ts time.Time, commits []string) *Task {
	return &Task{
		Created: ts,
		TaskKey: TaskKey{
			RepoState: RepoState{
				Repo:     DEFAULT_TEST_REPO,
				Revision: commits[0],
			},
			Name: "Test-Task",
		},
		Commits:        commits,
		SwarmingTaskId: "swarmid",
	}
}

func MakeTestJob(ts time.Time) *Job {
	return &Job{
		Created:      ts,
		Dependencies: map[string][]string{},
		RepoState: RepoState{
			Repo: DEFAULT_TEST_REPO,
		},
		Name:  "Test-Job",
		Tasks: map[string][]*TaskSummary{},
	}
}

// MakeFullJob creates a Job instance which has all of its fields filled.
func MakeFullJob(now time.Time) *Job {
	return &Job{
		BuildbucketBuildId:  12345,
		BuildbucketLeaseKey: 987,
		Created:             now.Add(time.Nanosecond),
		DbModified:          now.Add(time.Millisecond),
		Dependencies:        map[string][]string{"A": {"B"}, "B": {}},
		Finished:            now.Add(time.Second),
		Id:                  "abc123",
		IsForce:             true,
		Name:                "C",
		Priority:            1.2,
		RepoState: RepoState{
			Repo: DEFAULT_TEST_REPO,
		},
		Status: JOB_STATUS_SUCCESS,
		Tasks: map[string][]*TaskSummary{
			"task-name": {&TaskSummary{
				Id:             "12345",
				Status:         TASK_STATUS_FAILURE,
				SwarmingTaskId: "abc123",
			}},
		},
	}
}

// MakeTaskComment creates a comment with its ID fields based on the given repo,
// name, commit, and ts, and other fields based on n.
func MakeTaskComment(n int, repo int, name int, commit int, ts time.Time) *TaskComment {
	return &TaskComment{
		Repo:      fmt.Sprintf("%s%d", common.REPO_SKIA, repo),
		Revision:  fmt.Sprintf("c%d", commit),
		Name:      fmt.Sprintf("n%d", name),
		Timestamp: ts,
		TaskId:    fmt.Sprintf("id%d", n),
		User:      fmt.Sprintf("u%d", n),
		Message:   fmt.Sprintf("m%d", n),
	}
}

// MakeTaskSpecComment creates a comment with its ID fields based on the given
// repo, name, and ts, and other fields based on n.
func MakeTaskSpecComment(n int, repo int, name int, ts time.Time) *TaskSpecComment {
	return &TaskSpecComment{
		Repo:          fmt.Sprintf("%s%d", common.REPO_SKIA, repo),
		Name:          fmt.Sprintf("n%d", name),
		Timestamp:     ts,
		User:          fmt.Sprintf("u%d", n),
		Flaky:         n%2 == 0,
		IgnoreFailure: n>>1%2 == 0,
		Message:       fmt.Sprintf("m%d", n),
	}
}

// MakeCommitComment creates a comment with its ID fields based on the given
// repo, commit, and ts, and other fields based on n.
func MakeCommitComment(n int, repo int, commit int, ts time.Time) *CommitComment {
	return &CommitComment{
		Repo:          fmt.Sprintf("%s%d", common.REPO_SKIA, repo),
		Revision:      fmt.Sprintf("c%d", commit),
		Timestamp:     ts,
		User:          fmt.Sprintf("u%d", n),
		IgnoreFailure: n>>1%2 == 0,
		Message:       fmt.Sprintf("m%d", n),
	}
}
