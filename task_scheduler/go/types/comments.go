package types

import (
	"bytes"
	"encoding/gob"
	"time"

	"go.skia.org/infra/go/sklog"
)

// TaskComment contains a comment about a Task. {Repo, Revision, Name,
// Timestamp} is used as the unique id for this comment. If TaskId is empty, the
// comment applies to all matching tasks.
type TaskComment struct {
	Repo     string `json:"repo"`
	Revision string `json:"revision"`
	Name     string `json:"name"` // Name of TaskSpec.
	// Timestamp is compared ignoring timezone. The timezone reflects User's
	// location.
	Timestamp time.Time `json:"time"`
	TaskId    string    `json:"taskId"`
	User      string    `json:"user"`
	Message   string    `json:"message"`
}

func (c TaskComment) Copy() *TaskComment {
	return &c
}

// TaskSpecComment contains a comment about a TaskSpec. {Repo, Name, Timestamp}
// is used as the unique id for this comment.
type TaskSpecComment struct {
	Repo string `json:"repo"`
	Name string `json:"name"` // Name of TaskSpec.
	// Timestamp is compared ignoring timezone. The timezone reflects User's
	// location.
	Timestamp     time.Time `json:"time"`
	User          string    `json:"user"`
	Flaky         bool      `json:"flaky"`
	IgnoreFailure bool      `json:"ignoreFailure"`
	Message       string    `json:"message"`
}

func (c TaskSpecComment) Copy() *TaskSpecComment {
	return &c
}

// CommitComment contains a comment about a commit. {Repo, Revision, Timestamp}
// is used as the unique id for this comment.
type CommitComment struct {
	Repo     string `json:"repo"`
	Revision string `json:"revision"`
	// Timestamp is compared ignoring timezone. The timezone reflects User's
	// location.
	Timestamp     time.Time `json:"time"`
	User          string    `json:"user"`
	IgnoreFailure bool      `json:"ignoreFailure"`
	Message       string    `json:"message"`
}

func (c CommitComment) Copy() *CommitComment {
	return &c
}

// RepoComments contains comments that all pertain to the same repository.
type RepoComments struct {
	// Repo is the repository (Repo field) of all the comments contained in
	// this RepoComments.
	Repo string
	// TaskComments maps commit hash and TaskSpec name to the comments for
	// the matching Task, sorted by timestamp.
	TaskComments map[string]map[string][]*TaskComment
	// TaskSpecComments maps TaskSpec name to the comments for that
	// TaskSpec, sorted by timestamp.
	TaskSpecComments map[string][]*TaskSpecComment
	// CommitComments maps commit hash to the comments for that commit,
	// sorted by timestamp.
	CommitComments map[string][]*CommitComment
}

func (orig *RepoComments) Copy() *RepoComments {
	// TODO(benjaminwagner): Make this more efficient.
	b := bytes.Buffer{}
	if err := gob.NewEncoder(&b).Encode(orig); err != nil {
		sklog.Fatal(err)
	}
	copy := RepoComments{}
	if err := gob.NewDecoder(&b).Decode(&copy); err != nil {
		sklog.Fatal(err)
	}
	return &copy
}
