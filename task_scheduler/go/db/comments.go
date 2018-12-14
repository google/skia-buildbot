package db

import (
	"time"

	"go.skia.org/infra/task_scheduler/go/types"
)

// ModifiedCommentsReader tracks which comments have been added or deleted and
// returns results to subscribers based on what has changed since the last call
// to GetModifiedComments.
type ModifiedComments interface {
	// GetModifiedComments returns all comments added or deleted since the
	// last time GetModifiedComments was run with the given id. The returned
	// comments are sorted by timestamp. If GetModifiedComments returns an
	// error, the caller should call StopTrackingModifiedComments and
	// StartTrackingModifiedComments again, and load all data from scratch
	// to be sure that no comments were missed.
	GetModifiedComments(string) ([]*types.TaskComment, []*types.TaskSpecComment, []*types.CommitComment, error)

	// StartTrackingModifiedComments initiates tracking of modified comments
	// for the current caller. Returns a unique ID which can be used by the
	// caller to retrieve comments which have been added or deleted since
	// the last query. The ID expires after a period of inactivity.
	StartTrackingModifiedComments() (string, error)

	// StopTrackingModifiedComments cancels tracking of modified comments
	// for the provided ID.
	StopTrackingModifiedComments(string)

	// TrackModifiedTaskComment indicates the given comment should be
	// returned from the next call to GetModifiedComments from each
	// subscriber.
	TrackModifiedTaskComment(*types.TaskComment)

	// TrackModifiedTaskSpecComment indicates the given comment should be
	// returned from the next call to GetModifiedComments from each
	// subscriber.
	TrackModifiedTaskSpecComment(*types.TaskSpecComment)

	// TrackModifiedCommitComment indicates the given comment should be
	// returned from the next call to GetModifiedComments from each
	// subscriber.
	TrackModifiedCommitComment(*types.CommitComment)
}

// CommentDB stores comments on Tasks, TaskSpecs, and commits.
//
// Clients must be tolerant of comments that refer to nonexistent Tasks,
// TaskSpecs, or commits.
type CommentDB interface {
	ModifiedComments

	// GetComments returns all comments for the given repos.
	//
	// If from is specified, it is a hint that TaskComments and CommitComments
	// before this time will be ignored by the caller, thus they may be ommitted.
	GetCommentsForRepos(repos []string, from time.Time) ([]*types.RepoComments, error)

	// PutTaskComment inserts the TaskComment into the database. May return
	// ErrAlreadyExists.
	PutTaskComment(*types.TaskComment) error

	// DeleteTaskComment deletes the matching TaskComment from the database.
	// Non-ID fields of the argument are ignored.
	DeleteTaskComment(*types.TaskComment) error

	// PutTaskSpecComment inserts the TaskSpecComment into the database. May
	// return ErrAlreadyExists.
	PutTaskSpecComment(*types.TaskSpecComment) error

	// DeleteTaskSpecComment deletes the matching TaskSpecComment from the
	// database. Non-ID fields of the argument are ignored.
	DeleteTaskSpecComment(*types.TaskSpecComment) error

	// PutCommitComment inserts the CommitComment into the database. May return
	// ErrAlreadyExists.
	PutCommitComment(*types.CommitComment) error

	// DeleteCommitComment deletes the matching CommitComment from the database.
	// Non-ID fields of the argument are ignored.
	DeleteCommitComment(*types.CommitComment) error
}
