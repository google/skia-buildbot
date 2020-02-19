package trace

import (
	"time"

	"go.skia.org/infra/go/paramtools"
)

// Comment represents a comment made on a Gold trace.
type Comment struct {
	// ID uniquely represents a comment. It will be provided by the Store upon creation.
	ID ID
	// CreatedBy is the email address of the user who created this trace comment.
	CreatedBy string
	// UpdatedBy is the email address of the user who most recently updated this trace comment.
	UpdatedBy string
	// CreatedTS is when the comment was created.
	CreatedTS time.Time
	// UpdatedTS is when the comment was updated.
	UpdatedTS time.Time
	// Comment is an arbitrary string. There can be special rules that only the frontend cares about
	// (e.g. some markdown or coordinates).
	Comment string
	// QueryToMatch represents which traces this trace comment should apply to.
	QueryToMatch paramtools.ParamSet
}

// ID represents a unique identifier to a comment for the purposes of retrieval.
type ID string
