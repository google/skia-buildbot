package incremental

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

// commentsCache is a struct used for tracking various types of comments.
type commentsCache struct {
	db           db.CommentDB
	mtx          sync.Mutex
	prevComments map[string]*types.RepoComments
	repos        []string
}

// CommitComment is a wrapper around types.CommitComment which interprets the
// timestamp (in nanoseconds) as the ID, for convenience.
type CommitComment struct {
	types.CommitComment
	Id string `json:"id"`
}

// TaskComment is a wrapper around types.TaskComment which interprets the
// timestamp (in nanoseconds) as the ID, for convenience.
type TaskComment struct {
	types.TaskComment
	Id string `json:"id"`
}

// TaskSpecComment is a wrapper around types.TaskSpecComment which interprets the
// timestamp (in nanoseconds) as the ID, for convenience.
type TaskSpecComment struct {
	types.TaskSpecComment
	Id string `json:"id"`
}

// RepoComments is a variant of types.RepoComments which uses the above wrappers
// around the typical comment types.
type RepoComments struct {
	// CommitComments maps commit hash to the comments for that commit,
	// sorted by timestamp.
	CommitComments map[string][]*CommitComment
	// TaskComments maps commit hash and TaskSpec name to the comments for
	// the matching Task, sorted by timestamp.
	TaskComments map[string]map[string][]*TaskComment
	// TaskSpecComments maps TaskSpec name to the comments for that
	// TaskSpec, sorted by timestamp.
	TaskSpecComments map[string][]*TaskSpecComment
}

// commitComments converts the types.CommitComments to CommitComments.
func commitComments(inp map[string][]*types.CommitComment) map[string][]*CommitComment {
	rv := make(map[string][]*CommitComment, len(inp))
	for k, v := range inp {
		comments := make([]*CommitComment, 0, len(v))
		for _, c := range v {
			comments = append(comments, &CommitComment{
				CommitComment: *c,
				Id:            fmt.Sprintf("%d", c.Timestamp.UnixNano()),
			})
		}
		rv[k] = comments
	}
	return rv
}

// taskComments converts the types.TaskComments to TaskComments.
func taskComments(inp map[string]map[string][]*types.TaskComment) map[string]map[string][]*TaskComment {
	rv := make(map[string]map[string][]*TaskComment, len(inp))
	for k, v := range inp {
		submap := make(map[string][]*TaskComment, len(v))
		for k2, v2 := range v {
			comments := make([]*TaskComment, 0, len(v2))
			for _, c := range v2 {
				comments = append(comments, &TaskComment{
					TaskComment: *c,
					Id:          fmt.Sprintf("%d", c.Timestamp.UnixNano()),
				})
			}
			submap[k2] = comments
		}
		rv[k] = submap
	}
	return rv
}

// taskSpecComments converts the types.TaskSpecComments to TaskSpecComments.
func taskSpecComments(inp map[string][]*types.TaskSpecComment) map[string][]*TaskSpecComment {
	rv := make(map[string][]*TaskSpecComment, len(inp))
	for k, v := range inp {
		comments := make([]*TaskSpecComment, 0, len(v))
		for _, c := range v {
			comments = append(comments, &TaskSpecComment{
				TaskSpecComment: *c,
				Id:              fmt.Sprintf("%d", c.Timestamp.UnixNano()),
			})
		}
		rv[k] = comments
	}
	return rv
}

// newCommentsCache returns a commentsCache instance.
func newCommentsCache(d db.CommentDB, repos repograph.Map) *commentsCache {
	repoNames := make([]string, 0, len(repos))
	for repo := range repos {
		repoNames = append(repoNames, repo)
	}
	pc := make(map[string]*types.RepoComments, len(repoNames))
	for _, repo := range repoNames {
		pc[repo] = &types.RepoComments{}
	}
	return &commentsCache{
		db:           d,
		prevComments: pc,
		repos:        repoNames,
	}
}

// Reset clears previously-seen comments from the cache so that the next call
// to Update() returns all comments.
func (c *commentsCache) Reset() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.prevComments = make(map[string]*types.RepoComments, len(c.repos))
	for _, repo := range c.repos {
		c.prevComments[repo] = &types.RepoComments{}
	}
}

// Update returns any sets of comments which have changed since the last call
// to Update.
func (c *commentsCache) Update(ctx context.Context, w window.Window) (map[string]RepoComments, error) {
	defer metrics2.FuncTimer().Stop()
	c.mtx.Lock()
	defer c.mtx.Unlock()
	repoComments, err := c.db.GetCommentsForRepos(ctx, c.repos, w.EarliestStart())
	if err != nil {
		return nil, err
	}
	rv := make(map[string]RepoComments, len(c.repos))
	for _, rc := range repoComments {
		entry := RepoComments{}
		if !reflect.DeepEqual(rc.CommitComments, c.prevComments[rc.Repo].CommitComments) && len(rc.CommitComments) > 0 {
			entry.CommitComments = commitComments(rc.CommitComments)
		}
		if !reflect.DeepEqual(rc.TaskComments, c.prevComments[rc.Repo].TaskComments) && len(rc.TaskComments) > 0 {
			entry.TaskComments = taskComments(rc.TaskComments)
		}
		if !reflect.DeepEqual(rc.TaskSpecComments, c.prevComments[rc.Repo].TaskSpecComments) && len(rc.TaskSpecComments) > 0 {
			entry.TaskSpecComments = taskSpecComments(rc.TaskSpecComments)
		}
		rv[rc.Repo] = entry
		c.prevComments[rc.Repo] = rc
	}
	return rv, nil
}
