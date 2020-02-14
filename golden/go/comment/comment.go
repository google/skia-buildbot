package comment

import (
	"context"
	"time"

	"go.skia.org/infra/go/paramtools"
)

// Trace represents a comment made on a Gold trace.
type Trace struct {
	// ID uniquely represents a comment. It will be provided by the Store upon creation.
	ID string
	// CreatedBy is the email address of the user who created this Trace comment.
	CreatedBy string
	// UpdatedBy is the email address of the user who most recently updated this Trace comment.
	UpdatedBy string
	// CreatedTS is when the comment was created.
	CreatedTS time.Time
	// UpdatedTS is when the comment was updated.
	UpdatedTS time.Time
	// Comment is an arbitrary string. There can be special rules that only the frontend cares about
	// (e.g. some markdown or coordinates).
	Comment string
	// QueryToMatch represents which traces this Trace comment should apply to.
	QueryToMatch paramtools.ParamSet
}

// Store is an abstraction about a way to store comments.
type Store interface {
	// CreateTraceComment stores the given Trace comment. It will provide a new ID for the trace
	// and return it as the first return parameter if successful.
	CreateTraceComment(context.Context, Trace) (string, error)

	// DeleteTraceComment deletes a Trace comment with a given id. Implementations may return an
	// error if it does not exist.
	DeleteTraceComment(ctx context.Context, id string) error

	// UpdateTraceComment updates a stored Trace comment with the given values. It will not
	// replace the CreatedBy or CreatedTS, but everything else can be mutated.
	UpdateTraceComment(context.Context, Trace) error

	// ListTraceComments returns all Trace comments in the store.
	ListTraceComments(context.Context) ([]Trace, error)
}
