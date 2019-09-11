package memory

import (
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/modified"
	"go.skia.org/infra/task_scheduler/go/types"
)

// CommentBox implements CommentDB with in-memory storage.
//
// When created via NewCommentBoxWithPersistence, CommentBox will persist the
// in-memory representation on every change using the provided writer function.
type CommentBox struct {
	db.ModifiedComments

	// mtx protects comments.
	mtx sync.RWMutex
	// comments is map[repo_name]*types.RepoComments.
	comments map[string]*types.RepoComments
	// writer is called to persist comments after every change.
	writer func(map[string]*types.RepoComments) error
}

// NewCommentBoxWithPersistence creates a CommentBox that is initialized with
// init and sends the updated in-memory representation to writer after each
// change. The value of init and the argument to writer is
// map[repo_name]*types.RepoComments. init must not be modified by the caller. writer
// must not call any methods of CommentBox. writer may return an error to
// prevent a change from taking effect.
func NewCommentBoxWithPersistence(modComments db.ModifiedComments, init map[string]*types.RepoComments, writer func(map[string]*types.RepoComments) error) *CommentBox {
	if modComments == nil {
		modComments = &modified.ModifiedCommentsImpl{}
	}
	return &CommentBox{
		ModifiedComments: modComments,
		comments:         init,
		writer:           writer,
	}
}

// See documentation for CommentDB.GetCommentsForRepos.
func (b *CommentBox) GetCommentsForRepos(repos []string, from time.Time) ([]*types.RepoComments, error) {
	b.mtx.RLock()
	defer b.mtx.RUnlock()
	rv := make([]*types.RepoComments, len(repos))
	for i, repo := range repos {
		if rc, ok := b.comments[repo]; ok {
			rv[i] = rc.Copy()
		} else {
			rv[i] = &types.RepoComments{Repo: repo}
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

// getRepoComments returns the initialized *types.RepoComments for the given repo.
func (b *CommentBox) getRepoComments(repo string) *types.RepoComments {
	if b.comments == nil {
		b.comments = make(map[string]*types.RepoComments, 1)
	}
	rc, ok := b.comments[repo]
	if !ok {
		rc = &types.RepoComments{
			Repo:             repo,
			TaskComments:     map[string]map[string][]*types.TaskComment{},
			TaskSpecComments: map[string][]*types.TaskSpecComment{},
			CommitComments:   map[string][]*types.CommitComment{},
		}
		b.comments[repo] = rc
	}
	return rc
}

// putTaskComment validates c and adds c to b.comments, or returns
// db.ErrAlreadyExists if a different comment has the same ID fields. Assumes b.mtx
// is write-locked.
func (b *CommentBox) putTaskComment(c *types.TaskComment) error {
	if c.Repo == "" || c.Revision == "" || c.Name == "" || util.TimeIsZero(c.Timestamp) {
		return fmt.Errorf("TaskComment missing required fields. %#v", c)
	}
	rc := b.getRepoComments(c.Repo)
	nameMap, ok := rc.TaskComments[c.Revision]
	if !ok {
		nameMap = map[string][]*types.TaskComment{}
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
					return db.ErrAlreadyExists
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
		cSlice = []*types.TaskComment{c.Copy()}
	}
	nameMap[c.Name] = cSlice
	return nil
}

// deleteTaskComment validates c, then finds and removes a comment matching c's
// ID fields, returning the comment if found. Assumes b.mtx is write-locked.
func (b *CommentBox) deleteTaskComment(c *types.TaskComment) (*types.TaskComment, error) {
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
func (b *CommentBox) PutTaskComment(c *types.TaskComment) error {
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
	b.TrackModifiedTaskComment(c)
	return nil
}

// See documentation for CommentDB.DeleteTaskComment.
func (b *CommentBox) DeleteTaskComment(c *types.TaskComment) error {
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
		deleted := true
		c.Deleted = &deleted
		existing.Deleted = &deleted
		b.TrackModifiedTaskComment(existing)
	}
	return nil
}

// putTaskSpecComment validates c and adds c to b.comments, or returns
// db.ErrAlreadyExists if a different comment has the same ID fields. Assumes b.mtx
// is write-locked.
func (b *CommentBox) putTaskSpecComment(c *types.TaskSpecComment) error {
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
					return db.ErrAlreadyExists
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
		cSlice = []*types.TaskSpecComment{c.Copy()}
	}
	rc.TaskSpecComments[c.Name] = cSlice
	return nil
}

// deleteTaskSpecComment validates c, then finds and removes a comment matching
// c's ID fields, returning the comment if found. Assumes b.mtx is write-locked.
func (b *CommentBox) deleteTaskSpecComment(c *types.TaskSpecComment) (*types.TaskSpecComment, error) {
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
func (b *CommentBox) PutTaskSpecComment(c *types.TaskSpecComment) error {
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
	b.TrackModifiedTaskSpecComment(c)
	return nil
}

// See documentation for CommentDB.DeleteTaskSpecComment.
func (b *CommentBox) DeleteTaskSpecComment(c *types.TaskSpecComment) error {
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
		deleted := true
		c.Deleted = &deleted
		existing.Deleted = &deleted
		b.TrackModifiedTaskSpecComment(existing)
	}
	return nil
}

// putCommitComment validates c and adds c to b.comments, or returns
// db.ErrAlreadyExists if a different comment has the same ID fields. Assumes b.mtx
// is write-locked.
func (b *CommentBox) putCommitComment(c *types.CommitComment) error {
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
					return db.ErrAlreadyExists
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
		cSlice = []*types.CommitComment{c.Copy()}
	}
	rc.CommitComments[c.Revision] = cSlice
	return nil
}

// deleteCommitComment validates c, then finds and removes a comment matching
// c's ID fields, returning the comment if found. Assumes b.mtx is write-locked.
func (b *CommentBox) deleteCommitComment(c *types.CommitComment) (*types.CommitComment, error) {
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
func (b *CommentBox) PutCommitComment(c *types.CommitComment) error {
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
	b.TrackModifiedCommitComment(c)
	return nil
}

// See documentation for CommentDB.DeleteCommitComment.
func (b *CommentBox) DeleteCommitComment(c *types.CommitComment) error {
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
		deleted := true
		c.Deleted = &deleted
		existing.Deleted = &deleted
		b.TrackModifiedCommitComment(existing)
	}
	return nil
}
