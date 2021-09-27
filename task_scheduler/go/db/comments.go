package db

import (
	"context"
	"time"

	"go.skia.org/infra/task_scheduler/go/types"
)

// CommentDB stores comments on Tasks, TaskSpecs, and commits.
//
// Clients must be tolerant of comments that refer to nonexistent Tasks,
// TaskSpecs, or commits.
type CommentDB interface {
	// GetComments returns all comments for the given repos.
	//
	// If from is specified, it is a hint that TaskComments and CommitComments
	// before this time will be ignored by the caller, thus they may be ommitted.
	GetCommentsForRepos(ctx context.Context, repos []string, from time.Time) ([]*types.RepoComments, error)

	// PutTaskComment inserts the TaskComment into the database. May return
	// ErrAlreadyExists.
	PutTaskComment(context.Context, *types.TaskComment) error

	// DeleteTaskComment deletes the matching TaskComment from the database.
	// Non-ID fields of the argument are ignored.
	DeleteTaskComment(context.Context, *types.TaskComment) error

	// PutTaskSpecComment inserts the TaskSpecComment into the database. May
	// return ErrAlreadyExists.
	PutTaskSpecComment(context.Context, *types.TaskSpecComment) error

	// DeleteTaskSpecComment deletes the matching TaskSpecComment from the
	// database. Non-ID fields of the argument are ignored.
	DeleteTaskSpecComment(context.Context, *types.TaskSpecComment) error

	// PutCommitComment inserts the CommitComment into the database. May return
	// ErrAlreadyExists.
	PutCommitComment(context.Context, *types.CommitComment) error

	// DeleteCommitComment deletes the matching CommitComment from the database.
	// Non-ID fields of the argument are ignored.
	DeleteCommitComment(context.Context, *types.CommitComment) error

	// ModifiedTaskCommentsCh returns a channel which produces TaskComments
	// as they are modified in the DB. The channel is closed when the given
	// Context is canceled. At least one (possibly empty) item will be sent
	// on the returned channel.
	ModifiedTaskCommentsCh(context.Context) <-chan []*types.TaskComment

	// ModifiedTaskSpecCommentsCh returns a channel which produces
	// TaskSpecComments as they are modified in the DB. The channel is
	// closed when the given Context is canceled. At least one (possibly
	// empty) item will be sent on the returned channel.
	ModifiedTaskSpecCommentsCh(context.Context) <-chan []*types.TaskSpecComment

	// ModifiedCommitCommentsCh returns a channel which produces
	// CommitComments as they are modified in the DB. The channel is closed
	// when the given Context is canceled. At least one (possibly empty)
	// item will be sent on the returned channel.
	ModifiedCommitCommentsCh(context.Context) <-chan []*types.CommitComment
}
