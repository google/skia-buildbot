package types

import (
	"bytes"
	"encoding/gob"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
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
	TaskId    string    `json:"taskId,omitempty"`
	User      string    `json:"user,omitempty"`
	Message   string    `json:"message,omitempty"`
	Deleted   *bool     `json:"deleted,omitempty"`
}

func (c TaskComment) Copy() *TaskComment {
	rv := &c
	if c.Deleted != nil {
		v := *c.Deleted
		rv.Deleted = &v
	}
	return rv
}

func (c *TaskComment) Id() string {
	return c.Repo + "#" + c.Revision + "#" + c.Name + "#" + c.Timestamp.Format(util.SAFE_TIMESTAMP_FORMAT)
}

// TaskSpecComment contains a comment about a TaskSpec. {Repo, Name, Timestamp}
// is used as the unique id for this comment.
type TaskSpecComment struct {
	Repo string `json:"repo"`
	Name string `json:"name"` // Name of TaskSpec.
	// Timestamp is compared ignoring timezone. The timezone reflects User's
	// location.
	Timestamp     time.Time `json:"time"`
	User          string    `json:"user,omitempty"`
	Flaky         bool      `json:"flaky"`
	IgnoreFailure bool      `json:"ignoreFailure"`
	Message       string    `json:"message,omitempty"`
	Deleted       *bool     `json:"deleted,omitempty"`
}

func (c TaskSpecComment) Copy() *TaskSpecComment {
	rv := &c
	if c.Deleted != nil {
		v := *c.Deleted
		rv.Deleted = &v
	}
	return rv
}

func (c *TaskSpecComment) Id() string {
	return c.Repo + "#" + c.Name + "#" + c.Timestamp.Format(util.SAFE_TIMESTAMP_FORMAT)
}

// CommitComment contains a comment about a commit. {Repo, Revision, Timestamp}
// is used as the unique id for this comment.
type CommitComment struct {
	Repo     string `json:"repo"`
	Revision string `json:"revision"`
	// Timestamp is compared ignoring timezone. The timezone reflects User's
	// location.
	Timestamp     time.Time `json:"time"`
	User          string    `json:"user,omitempty"`
	IgnoreFailure bool      `json:"ignoreFailure"`
	Message       string    `json:"message,omitempty"`
	Deleted       *bool     `json:"deleted,omitempty"`
}

func (c CommitComment) Copy() *CommitComment {
	rv := &c
	if c.Deleted != nil {
		v := *c.Deleted
		rv.Deleted = &v
	}
	return rv
}

func (c *CommitComment) Id() string {
	return c.Repo + "#" + c.Revision + "#" + c.Timestamp.Format(util.SAFE_TIMESTAMP_FORMAT)
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

// TaskCommentSlice implements sort.Interface. To sort taskComments
// []*TaskComment, use sort.Sort(TaskCommentSlice(taskComments)).
type TaskCommentSlice []*TaskComment

func (s TaskCommentSlice) Len() int { return len(s) }

func (s TaskCommentSlice) Less(i, j int) bool {
	return s[i].Timestamp.Before(s[j].Timestamp)
}

func (s TaskCommentSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// TaskSpecCommentSlice implements sort.Interface. To sort taskSpecComments
// []*TaskSpecComment, use sort.Sort(TaskSpecCommentSlice(taskSpecComments)).
type TaskSpecCommentSlice []*TaskSpecComment

func (s TaskSpecCommentSlice) Len() int { return len(s) }

func (s TaskSpecCommentSlice) Less(i, j int) bool {
	return s[i].Timestamp.Before(s[j].Timestamp)
}

func (s TaskSpecCommentSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// CommitCommentSlice implements sort.Interface. To sort commitComments
// []*CommitComment, use sort.Sort(CommitCommentSlice(commitComments)).
type CommitCommentSlice []*CommitComment

func (s CommitCommentSlice) Len() int { return len(s) }

func (s CommitCommentSlice) Less(i, j int) bool {
	return s[i].Timestamp.Before(s[j].Timestamp)
}

func (s CommitCommentSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
