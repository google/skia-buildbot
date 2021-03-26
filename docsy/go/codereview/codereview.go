package codereview

import "context"

// Issue is the identifier for a CodeReview issue.
type Issue string

// A special value for an issue which means to use the main branch at HEAD.
const MainIssue Issue = "-1"

// ListModifiedFilesResult is the results from calling CodeReviewListModifiedFiles.
type ListModifiedFilesResult struct {
	// Filename relative to the root of the git repo.
	Filename string

	// Deleted is true if the file was deleted at the given patchset.
	Deleted bool
}

// CodeReview represents an abstraction of the information we want from a code
// review system such as Gerrit.
type CodeReview interface {
	// ListModifiedFiles returns a list of the modified files for the given
	// issue at the given Ref.
	ListModifiedFiles(ctx context.Context, issue Issue, ref string) ([]ListModifiedFilesResult, error)

	// GetFiles returns the contents of the given file at the given Ref as a byte slice.
	GetFile(ctx context.Context, filename string, ref string) ([]byte, error)

	// GetPatchsetInfo returns the most recent patchset Ref of the given issue and
	// also if the issue has been closed.
	GetPatchsetInfo(ctx context.Context, issue Issue) (string, bool, error)
}
