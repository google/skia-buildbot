package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
)

// CommentBox implements CommentDB with in-memory storage.
type CommentBox struct {
	// mtx protects comments.
	mtx sync.RWMutex
	// comments is map[repo_name]*types.RepoComments.
	comments        map[string]*types.RepoComments
	modTC           chan<- []*types.TaskComment
	modTCClients    map[chan<- []*types.TaskComment]context.Context
	modTCClientsMtx sync.Mutex
	modTCClientsWg  sync.WaitGroup
	modTSC          chan<- []*types.TaskSpecComment
	modTSClients    map[chan<- []*types.TaskSpecComment]context.Context
	modTSClientsMtx sync.Mutex
	modTSClientsWg  sync.WaitGroup
	modCC           chan<- []*types.CommitComment
	modCCClients    map[chan<- []*types.CommitComment]context.Context
	modCCClientsMtx sync.Mutex
	modCCClientsWg  sync.WaitGroup
}

// NewCommentBox returns a CommentBox instance.
func NewCommentBox() *CommentBox {
	modTC := make(chan []*types.TaskComment)
	modTSC := make(chan []*types.TaskSpecComment)
	modCC := make(chan []*types.CommitComment)
	rv := &CommentBox{
		comments:     map[string]*types.RepoComments{},
		modTC:        modTC,
		modTCClients: map[chan<- []*types.TaskComment]context.Context{},
		modTSC:       modTSC,
		modTSClients: map[chan<- []*types.TaskSpecComment]context.Context{},
		modCC:        modCC,
		modCCClients: map[chan<- []*types.CommitComment]context.Context{},
	}

	// These goroutines multiplex modified data out to the various clients.
	go func() {
		for data := range modTC {
			rv.modTCClientsMtx.Lock()
			for ch, ctx := range rv.modTCClients {
				if ctx.Err() != nil {
					continue
				}

				go func(ctx context.Context, ch chan<- []*types.TaskComment, data []*types.TaskComment) {
					send := make([]*types.TaskComment, 0, len(data))
					for _, elem := range data {
						send = append(send, elem.Copy())
					}
					select {
					case ch <- send:
					case <-ctx.Done():
					}
				}(ctx, ch, data)
			}
			rv.modTCClientsMtx.Unlock()
		}
	}()
	go func() {
		for data := range modTSC {
			rv.modTSClientsMtx.Lock()
			for ch, ctx := range rv.modTSClients {
				if ctx.Err() != nil {
					continue
				}

				go func(ctx context.Context, ch chan<- []*types.TaskSpecComment, data []*types.TaskSpecComment) {
					send := make([]*types.TaskSpecComment, 0, len(data))
					for _, elem := range data {
						send = append(send, elem.Copy())
					}
					select {
					case ch <- send:
					case <-ctx.Done():
					}
				}(ctx, ch, data)
			}
			rv.modTSClientsMtx.Unlock()
		}
	}()
	go func() {
		for data := range modCC {
			rv.modCCClientsMtx.Lock()
			for ch, ctx := range rv.modCCClients {
				if ctx.Err() != nil {
					continue
				}

				go func(ctx context.Context, ch chan<- []*types.CommitComment, data []*types.CommitComment) {
					send := make([]*types.CommitComment, 0, len(data))
					for _, elem := range data {
						send = append(send, elem.Copy())
					}
					select {
					case ch <- send:
					case <-ctx.Done():
					}
				}(ctx, ch, data)
			}
			rv.modCCClientsMtx.Unlock()
		}
	}()
	return rv
}

// See documentation for CommentDB.GetCommentsForRepos.
func (b *CommentBox) GetCommentsForRepos(_ context.Context, repos []string, from time.Time) ([]*types.RepoComments, error) {
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
func (b *CommentBox) PutTaskComment(_ context.Context, c *types.TaskComment) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if err := b.putTaskComment(c); err != nil {
		return err
	}
	b.modTC <- []*types.TaskComment{c.Copy()}
	return nil
}

// See documentation for CommentDB.DeleteTaskComment.
func (b *CommentBox) DeleteTaskComment(_ context.Context, c *types.TaskComment) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	existing, err := b.deleteTaskComment(c)
	if err != nil {
		return err
	}
	if existing != nil {
		deleted := true
		c.Deleted = &deleted
		existing.Deleted = &deleted
		b.modTC <- []*types.TaskComment{existing.Copy()}
	}
	return nil
}

// See docs for CommentDB interface.
func (b *CommentBox) ModifiedTaskCommentsCh(ctx context.Context) <-chan []*types.TaskComment {
	b.modTCClientsMtx.Lock()
	defer b.modTCClientsMtx.Unlock()
	localCh := make(chan []*types.TaskComment)
	rv := make(chan []*types.TaskComment)
	b.modTCClients[localCh] = ctx
	done := ctx.Done()
	go func() {
		// The DB spec states that we should pass an initial value along
		// the channel.
		rv <- []*types.TaskComment{}
		for {
			select {
			case mod := <-localCh:
				rv <- mod
			case <-done:
				close(rv)
				// Remove the local channel. Note that we don't
				// close it, because other goroutines might be
				// trying to write to it.
				b.modTCClientsMtx.Lock()
				delete(b.modTCClients, localCh)
				b.modTCClientsMtx.Unlock()
				return
			}
		}
	}()
	return rv
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
func (b *CommentBox) PutTaskSpecComment(_ context.Context, c *types.TaskSpecComment) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if err := b.putTaskSpecComment(c); err != nil {
		return err
	}
	b.modTSC <- []*types.TaskSpecComment{c.Copy()}
	return nil
}

// See documentation for CommentDB.DeleteTaskSpecComment.
func (b *CommentBox) DeleteTaskSpecComment(_ context.Context, c *types.TaskSpecComment) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	existing, err := b.deleteTaskSpecComment(c)
	if err != nil {
		return err
	}
	if existing != nil {
		deleted := true
		c.Deleted = &deleted
		existing.Deleted = &deleted
		b.modTSC <- []*types.TaskSpecComment{existing.Copy()}
	}
	return nil
}

// See docs for CommentDB interface.
func (b *CommentBox) ModifiedTaskSpecCommentsCh(ctx context.Context) <-chan []*types.TaskSpecComment {
	b.modTSClientsMtx.Lock()
	defer b.modTSClientsMtx.Unlock()
	localCh := make(chan []*types.TaskSpecComment)
	rv := make(chan []*types.TaskSpecComment)
	b.modTSClients[localCh] = ctx
	done := ctx.Done()
	go func() {
		// The DB spec states that we should pass an initial value along
		// the channel.
		rv <- []*types.TaskSpecComment{}
		for {
			select {
			case mod := <-localCh:
				rv <- mod
			case <-done:
				close(rv)
				// Remove the local channel. Note that we don't
				// close it, because other goroutines might be
				// trying to write to it.
				b.modTSClientsMtx.Lock()
				delete(b.modTSClients, localCh)
				b.modTSClientsMtx.Unlock()
				return
			}
		}
	}()
	return rv
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
func (b *CommentBox) PutCommitComment(_ context.Context, c *types.CommitComment) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if err := b.putCommitComment(c); err != nil {
		return err
	}
	b.modCC <- []*types.CommitComment{c.Copy()}
	return nil
}

// See documentation for CommentDB.DeleteCommitComment.
func (b *CommentBox) DeleteCommitComment(_ context.Context, c *types.CommitComment) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	existing, err := b.deleteCommitComment(c)
	if err != nil {
		return err
	}
	if existing != nil {
		deleted := true
		c.Deleted = &deleted
		existing.Deleted = &deleted
		b.modCC <- []*types.CommitComment{existing.Copy()}
	}
	return nil
}

// See docs for CommentDB interface.
func (b *CommentBox) ModifiedCommitCommentsCh(ctx context.Context) <-chan []*types.CommitComment {
	b.modCCClientsMtx.Lock()
	defer b.modCCClientsMtx.Unlock()
	localCh := make(chan []*types.CommitComment)
	rv := make(chan []*types.CommitComment)
	b.modCCClients[localCh] = ctx
	done := ctx.Done()
	go func() {
		// The DB spec states that we should pass an initial value along
		// the channel.
		rv <- []*types.CommitComment{}
		for {
			select {
			case mod := <-localCh:
				rv <- mod
			case <-done:
				close(rv)
				// Remove the local channel. Note that we don't
				// close it, because other goroutines might be
				// trying to write to it.
				b.modCCClientsMtx.Lock()
				delete(b.modCCClients, localCh)
				b.modCCClientsMtx.Unlock()
				return
			}
		}
	}()
	return rv
}

// Wait for all clients to receive modified data.
func (b *CommentBox) Wait() {
	b.modTCClientsWg.Wait()
	b.modTSClientsWg.Wait()
	b.modCCClientsWg.Wait()
}
