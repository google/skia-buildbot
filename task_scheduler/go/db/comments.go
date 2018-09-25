package db

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sync"
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

// CommentDB stores comments on Tasks, TaskSpecs, and commits.
//
// Clients must be tolerant of comments that refer to nonexistent Tasks,
// TaskSpecs, or commits.
type CommentDB interface {
	// GetComments returns all comments for the given repos.
	//
	// If from is specified, it is a hint that TaskComments and CommitComments
	// before this time will be ignored by the caller, thus they may be ommitted.
	GetCommentsForRepos(repos []string, from time.Time) ([]*RepoComments, error)

	// PutTaskComment inserts the TaskComment into the database. May return
	// ErrAlreadyExists.
	PutTaskComment(*TaskComment) error

	// DeleteTaskComment deletes the matching TaskComment from the database.
	// Non-ID fields of the argument are ignored.
	DeleteTaskComment(*TaskComment) error

	// PutTaskSpecComment inserts the TaskSpecComment into the database. May
	// return ErrAlreadyExists.
	PutTaskSpecComment(*TaskSpecComment) error

	// DeleteTaskSpecComment deletes the matching TaskSpecComment from the
	// database. Non-ID fields of the argument are ignored.
	DeleteTaskSpecComment(*TaskSpecComment) error

	// PutCommitComment inserts the CommitComment into the database. May return
	// ErrAlreadyExists.
	PutCommitComment(*CommitComment) error

	// DeleteCommitComment deletes the matching CommitComment from the database.
	// Non-ID fields of the argument are ignored.
	DeleteCommitComment(*CommitComment) error
}

// CommentBox implements CommentDB with in-memory storage.
//
// When created via NewCommentBoxWithPersistence, CommentBox will persist the
// in-memory representation on every change using the provided writer function.
//
// CommentBox can be default-initialized if only in-memory storage is desired.
type CommentBox struct {
	// mtx protects comments.
	mtx sync.RWMutex
	// comments is map[repo_name]*RepoComments.
	comments map[string]*RepoComments
	// writer is called to persist comments after every change.
	writer func(map[string]*RepoComments) error
}

// NewCommentBoxWithPersistence creates a CommentBox that is initialized with
// init and sends the updated in-memory representation to writer after each
// change. The value of init and the argument to writer is
// map[repo_name]*RepoComments. init must not be modified by the caller. writer
// must not call any methods of CommentBox. writer may return an error to
// prevent a change from taking effect.
func NewCommentBoxWithPersistence(init map[string]*RepoComments, writer func(map[string]*RepoComments) error) *CommentBox {
	return &CommentBox{
		comments: init,
		writer:   writer,
	}
}

// See documentation for CommentDB.GetCommentsForRepos.
func (b *CommentBox) GetCommentsForRepos(repos []string, from time.Time) ([]*RepoComments, error) {
	b.mtx.RLock()
	defer b.mtx.RUnlock()
	rv := make([]*RepoComments, len(repos))
	for i, repo := range repos {
		if rc, ok := b.comments[repo]; ok {
			rv[i] = rc.Copy()
		} else {
			rv[i] = &RepoComments{Repo: repo}
		}
	}
	return rv, nil
}

// write calls b.writer with comments if non-null.
func (b *CommentBox) write() error {
	if b.writer == nil {
		return nil
	}
	return b.writer(b.comments)
}

// getRepoComments returns the initialized *RepoComments for the given repo.
func (b *CommentBox) getRepoComments(repo string) *RepoComments {
	if b.comments == nil {
		b.comments = make(map[string]*RepoComments, 1)
	}
	rc, ok := b.comments[repo]
	if !ok {
		rc = &RepoComments{
			Repo:             repo,
			TaskComments:     map[string]map[string][]*TaskComment{},
			TaskSpecComments: map[string][]*TaskSpecComment{},
			CommitComments:   map[string][]*CommitComment{},
		}
		b.comments[repo] = rc
	}
	return rc
}

// putTaskComment validates c and adds c to b.comments, or returns
// ErrAlreadyExists if a different comment has the same ID fields. Assumes b.mtx
// is write-locked.
func (b *CommentBox) putTaskComment(c *TaskComment) error {
	if c.Repo == "" || c.Revision == "" || c.Name == "" || util.TimeIsZero(c.Timestamp) {
		return fmt.Errorf("TaskComment missing required fields. %#v", c)
	}
	rc := b.getRepoComments(c.Repo)
	nameMap, ok := rc.TaskComments[c.Revision]
	if !ok {
		nameMap = map[string][]*TaskComment{}
		rc.TaskComments[c.Revision] = nameMap
	}
	cSlice := nameMap[c.Name]
	// TODO(benjaminwagner): Would using utilities in the sort package make this
	// cleaner?
	if len(cSlice) > 0 {
		// Assume comments normally inserted at the end.
		insert := 0
		for i := len(cSlice) - 1; i >= 0; i-- {
			if cSlice[i].Timestamp.Equal(c.Timestamp) {
				if *cSlice[i] == *c {
					return nil
				} else {
					return ErrAlreadyExists
				}
			} else if cSlice[i].Timestamp.Before(c.Timestamp) {
				insert = i + 1
				break
			}
		}
		// Ensure capacity for another comment and move any comments after the
		// insertion point.
		cSlice = append(cSlice, nil)
		copy(cSlice[insert+1:], cSlice[insert:])
		cSlice[insert] = c.Copy()
	} else {
		cSlice = []*TaskComment{c.Copy()}
	}
	nameMap[c.Name] = cSlice
	return nil
}

// deleteTaskComment validates c, then finds and removes a comment matching c's
// ID fields, returning the comment if found. Assumes b.mtx is write-locked.
func (b *CommentBox) deleteTaskComment(c *TaskComment) (*TaskComment, error) {
	if c.Repo == "" || c.Revision == "" || c.Name == "" || util.TimeIsZero(c.Timestamp) {
		return nil, fmt.Errorf("TaskComment missing required fields. %#v", c)
	}
	if rc, ok := b.comments[c.Repo]; ok {
		if cSlice, ok := rc.TaskComments[c.Revision][c.Name]; ok {
			// Assume linear search is fast.
			for i, existing := range cSlice {
				if existing.Timestamp.Equal(c.Timestamp) {
					if len(cSlice) > 1 {
						rc.TaskComments[c.Revision][c.Name] = append(cSlice[:i], cSlice[i+1:]...)
					} else {
						delete(rc.TaskComments[c.Revision], c.Name)
						if len(rc.TaskComments[c.Revision]) == 0 {
							delete(rc.TaskComments, c.Revision)
						}
					}
					return existing, nil
				}
			}
		}
	}
	return nil, nil
}

// See documentation for CommentDB.PutTaskComment.
func (b *CommentBox) PutTaskComment(c *TaskComment) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if err := b.putTaskComment(c); err != nil {
		return err
	}
	if err := b.write(); err != nil {
		// If write returns an error, we must revert to previous.
		if _, delErr := b.deleteTaskComment(c); delErr != nil {
			sklog.Warningf("Unexpected error: %s", delErr)
		}
		return err
	}
	return nil
}

// See documentation for CommentDB.DeleteTaskComment.
func (b *CommentBox) DeleteTaskComment(c *TaskComment) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	existing, err := b.deleteTaskComment(c)
	if err != nil {
		return err
	}
	if existing != nil {
		if err := b.write(); err != nil {
			// If write returns an error, we must revert to previous.
			if putErr := b.putTaskComment(existing); putErr != nil {
				sklog.Warningf("Unexpected error: %s", putErr)
			}
			return err
		}
	}
	return nil
}

// putTaskSpecComment validates c and adds c to b.comments, or returns
// ErrAlreadyExists if a different comment has the same ID fields. Assumes b.mtx
// is write-locked.
func (b *CommentBox) putTaskSpecComment(c *TaskSpecComment) error {
	if c.Repo == "" || c.Name == "" || util.TimeIsZero(c.Timestamp) {
		return fmt.Errorf("TaskSpecComment missing required fields. %#v", c)
	}
	rc := b.getRepoComments(c.Repo)
	cSlice := rc.TaskSpecComments[c.Name]
	if len(cSlice) > 0 {
		// Assume comments normally inserted at the end.
		insert := 0
		for i := len(cSlice) - 1; i >= 0; i-- {
			if cSlice[i].Timestamp.Equal(c.Timestamp) {
				if *cSlice[i] == *c {
					return nil
				} else {
					return ErrAlreadyExists
				}
			} else if cSlice[i].Timestamp.Before(c.Timestamp) {
				insert = i + 1
				break
			}
		}
		// Ensure capacity for another comment and move any comments after the
		// insertion point.
		cSlice = append(cSlice, nil)
		copy(cSlice[insert+1:], cSlice[insert:])
		cSlice[insert] = c.Copy()
	} else {
		cSlice = []*TaskSpecComment{c.Copy()}
	}
	rc.TaskSpecComments[c.Name] = cSlice
	return nil
}

// deleteTaskSpecComment validates c, then finds and removes a comment matching
// c's ID fields, returning the comment if found. Assumes b.mtx is write-locked.
func (b *CommentBox) deleteTaskSpecComment(c *TaskSpecComment) (*TaskSpecComment, error) {
	if c.Repo == "" || c.Name == "" || util.TimeIsZero(c.Timestamp) {
		return nil, fmt.Errorf("TaskSpecComment missing required fields. %#v", c)
	}
	if rc, ok := b.comments[c.Repo]; ok {
		if cSlice, ok := rc.TaskSpecComments[c.Name]; ok {
			// Assume linear search is fast.
			for i, existing := range cSlice {
				if existing.Timestamp.Equal(c.Timestamp) {
					if len(cSlice) > 1 {
						rc.TaskSpecComments[c.Name] = append(cSlice[:i], cSlice[i+1:]...)
					} else {
						delete(rc.TaskSpecComments, c.Name)
					}
					return existing, nil
				}
			}
		}
	}
	return nil, nil
}

// See documentation for CommentDB.PutTaskSpecComment.
func (b *CommentBox) PutTaskSpecComment(c *TaskSpecComment) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if err := b.putTaskSpecComment(c); err != nil {
		return err
	}
	if err := b.write(); err != nil {
		// If write returns an error, we must revert to previous.
		if _, delErr := b.deleteTaskSpecComment(c); delErr != nil {
			sklog.Warningf("Unexpected error: %s", delErr)
		}
		return err
	}
	return nil
}

// See documentation for CommentDB.DeleteTaskSpecComment.
func (b *CommentBox) DeleteTaskSpecComment(c *TaskSpecComment) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	existing, err := b.deleteTaskSpecComment(c)
	if err != nil {
		return err
	}
	if existing != nil {
		if err := b.write(); err != nil {
			// If write returns an error, we must revert to previous.
			if putErr := b.putTaskSpecComment(existing); putErr != nil {
				sklog.Warningf("Unexpected error: %s", putErr)
			}
			return err
		}
	}
	return nil
}

// putCommitComment validates c and adds c to b.comments, or returns
// ErrAlreadyExists if a different comment has the same ID fields. Assumes b.mtx
// is write-locked.
func (b *CommentBox) putCommitComment(c *CommitComment) error {
	if c.Repo == "" || c.Revision == "" || util.TimeIsZero(c.Timestamp) {
		return fmt.Errorf("CommitComment missing required fields. %#v", c)
	}
	rc := b.getRepoComments(c.Repo)
	cSlice := rc.CommitComments[c.Revision]
	if len(cSlice) > 0 {
		// Assume comments normally inserted at the end.
		insert := 0
		for i := len(cSlice) - 1; i >= 0; i-- {
			if cSlice[i].Timestamp.Equal(c.Timestamp) {
				if *cSlice[i] == *c {
					return nil
				} else {
					return ErrAlreadyExists
				}
			} else if cSlice[i].Timestamp.Before(c.Timestamp) {
				insert = i + 1
				break
			}
		}
		// Ensure capacity for another comment and move any comments after the
		// insertion point.
		cSlice = append(cSlice, nil)
		copy(cSlice[insert+1:], cSlice[insert:])
		cSlice[insert] = c.Copy()
	} else {
		cSlice = []*CommitComment{c.Copy()}
	}
	rc.CommitComments[c.Revision] = cSlice
	return nil
}

// deleteCommitComment validates c, then finds and removes a comment matching
// c's ID fields, returning the comment if found. Assumes b.mtx is write-locked.
func (b *CommentBox) deleteCommitComment(c *CommitComment) (*CommitComment, error) {
	if c.Repo == "" || c.Revision == "" || util.TimeIsZero(c.Timestamp) {
		return nil, fmt.Errorf("CommitComment missing required fields. %#v", c)
	}
	if rc, ok := b.comments[c.Repo]; ok {
		if cSlice, ok := rc.CommitComments[c.Revision]; ok {
			// Assume linear search is fast.
			for i, existing := range cSlice {
				if existing.Timestamp.Equal(c.Timestamp) {
					if len(cSlice) > 1 {
						rc.CommitComments[c.Revision] = append(cSlice[:i], cSlice[i+1:]...)
					} else {
						delete(rc.CommitComments, c.Revision)
					}
					return existing, nil
				}
			}
		}
	}
	return nil, nil
}

// See documentation for CommentDB.PutCommitComment.
func (b *CommentBox) PutCommitComment(c *CommitComment) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if err := b.putCommitComment(c); err != nil {
		return err
	}
	if err := b.write(); err != nil {
		// If write returns an error, we must revert to previous.
		if _, delErr := b.deleteCommitComment(c); delErr != nil {
			sklog.Warningf("Unexpected error: %s", delErr)
		}
		return err
	}
	return nil
}

// See documentation for CommentDB.DeleteCommitComment.
func (b *CommentBox) DeleteCommitComment(c *CommitComment) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	existing, err := b.deleteCommitComment(c)
	if err != nil {
		return err
	}
	if existing != nil {
		if err := b.write(); err != nil {
			// If write returns an error, we must revert to previous.
			if putErr := b.putCommitComment(existing); putErr != nil {
				sklog.Warningf("Unexpected error: %s", putErr)
			}
			return err
		}
	}
	return nil
}
