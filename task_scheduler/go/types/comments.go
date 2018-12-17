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

// TaskCommentEncoder encodes TaskComments into bytes via GOB encoding. Not
// safe for concurrent use.
type TaskCommentEncoder struct {
	util.GobEncoder
}

// Next returns one of the TaskComments provided to Process (in arbitrary order)
// and its serialized bytes. If any comments remain, returns the TaskComment,
// the serialized bytes, nil. If all comments have been returned, returns nil,
// nil, nil. If an error is encountered, returns nil, nil, error.
func (e *TaskCommentEncoder) Next() (*TaskComment, []byte, error) {
	item, serialized, err := e.GobEncoder.Next()
	if err != nil {
		return nil, nil, err
	} else if item == nil {
		return nil, nil, nil
	}
	return item.(*TaskComment), serialized, err
}

// TaskCommentDecoder decodes bytes into TaskComments via GOB decoding. Not safe
// for concurrent use.
type TaskCommentDecoder struct {
	*util.GobDecoder
}

// NewTaskCommentDecoder returns a TaskCommentDecoder instance.
func NewTaskCommentDecoder() *TaskCommentDecoder {
	return &TaskCommentDecoder{
		GobDecoder: util.NewGobDecoder(func() interface{} {
			return &TaskComment{}
		}, func(ch <-chan interface{}) interface{} {
			items := []*TaskComment{}
			for item := range ch {
				items = append(items, item.(*TaskComment))
			}
			return items
		}),
	}
}

// Result returns all decoded TaskComments provided to Process (in arbitrary
// order), or any error encountered.
func (d *TaskCommentDecoder) Result() ([]*TaskComment, error) {
	res, err := d.GobDecoder.Result()
	if err != nil {
		return nil, err
	}
	return res.([]*TaskComment), nil
}

// TaskSpecCommentEncoder encodes TaskSpecComments into bytes via GOB encoding.
// Not safe for concurrent use.
type TaskSpecCommentEncoder struct {
	util.GobEncoder
}

// Next returns one of the TaskSpecComments provided to Process (in arbitrary
// order) and its serialized bytes. If any comments remain, returns the
// TaskSpecComment, the serialized bytes, nil. If all comments have been
// returned, returns nil, nil, nil. If an error is encountered, returns nil,
// nil, error.
func (e *TaskSpecCommentEncoder) Next() (*TaskSpecComment, []byte, error) {
	item, serialized, err := e.GobEncoder.Next()
	if err != nil {
		return nil, nil, err
	} else if item == nil {
		return nil, nil, nil
	}
	return item.(*TaskSpecComment), serialized, err
}

// TaskSpecCommentDecoder decodes bytes into TaskSpecComments via GOB decoding.
// Not safe for concurrent use.
type TaskSpecCommentDecoder struct {
	*util.GobDecoder
}

// NewTaskSpecCommentDecoder returns a TaskSpecCommentDecoder instance.
func NewTaskSpecCommentDecoder() *TaskSpecCommentDecoder {
	return &TaskSpecCommentDecoder{
		GobDecoder: util.NewGobDecoder(func() interface{} {
			return &TaskSpecComment{}
		}, func(ch <-chan interface{}) interface{} {
			items := []*TaskSpecComment{}
			for item := range ch {
				items = append(items, item.(*TaskSpecComment))
			}
			return items
		}),
	}
}

// Result returns all decoded TaskSpecComments provided to Process (in arbitrary
// order), or any error encountered.
func (d *TaskSpecCommentDecoder) Result() ([]*TaskSpecComment, error) {
	res, err := d.GobDecoder.Result()
	if err != nil {
		return nil, err
	}
	return res.([]*TaskSpecComment), nil
}

// CommitCommentEncoder encodes CommitComments into bytes via GOB encoding. Not
// safe for concurrent use.
type CommitCommentEncoder struct {
	util.GobEncoder
}

// Next returns one of the CommitComments provided to Process (in arbitrary
// order) and its serialized bytes. If any comments remain, returns the
// CommitComment, the serialized bytes, nil. If all comments have been returned,
// returns nil, nil, nil. If an error is encountered, returns nil, nil, error.
func (e *CommitCommentEncoder) Next() (*CommitComment, []byte, error) {
	item, serialized, err := e.GobEncoder.Next()
	if err != nil {
		return nil, nil, err
	} else if item == nil {
		return nil, nil, nil
	}
	return item.(*CommitComment), serialized, err
}

// CommitCommentDecoder decodes bytes into CommitComments via GOB decoding.
// Not safe for concurrent use.
type CommitCommentDecoder struct {
	*util.GobDecoder
}

// NewCommitCommentDecoder returns a CommitCommentDecoder instance.
func NewCommitCommentDecoder() *CommitCommentDecoder {
	return &CommitCommentDecoder{
		GobDecoder: util.NewGobDecoder(func() interface{} {
			return &CommitComment{}
		}, func(ch <-chan interface{}) interface{} {
			items := []*CommitComment{}
			for item := range ch {
				items = append(items, item.(*CommitComment))
			}
			return items
		}),
	}
}

// Result returns all decoded CommitComments provided to Process (in arbitrary
// order), or any error encountered.
func (d *CommitCommentDecoder) Result() ([]*CommitComment, error) {
	res, err := d.GobDecoder.Result()
	if err != nil {
		return nil, err
	}
	return res.([]*CommitComment), nil
}
