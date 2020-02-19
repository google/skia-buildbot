package comment

import (
	"context"

	"go.skia.org/infra/golden/go/comment/trace"
)

// Store is an abstraction about a way to store comments.
type Store interface {
	// CreateComment stores the given trace.Comment. It will provide a new ID for the trace
	// and return it as the first return parameter if successful.
	CreateComment(context.Context, trace.Comment) (trace.ID, error)

	// DeleteComment deletes a trace.Comment with a given id. Implementations may return an
	// error if it does not exist.
	DeleteComment(ctx context.Context, id trace.ID) error

	// UpdateComment updates a stored trace.Comment with the given values. It will not
	// replace the CreatedBy or CreatedTS, but everything else can be mutated.
	UpdateComment(context.Context, trace.Comment) error

	// ListComments returns all trace.Comment comments in the store.
	ListComments(context.Context) ([]trace.Comment, error)
}
