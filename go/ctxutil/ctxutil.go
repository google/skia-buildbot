package ctxutil

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// ConfirmContextHasDeadline logs errors with the full stack trace if the given
// context does not have a deadline. Useful for confirming that all SQL client
// calls have a timeout.
func ConfirmContextHasDeadline(ctx context.Context) {
	if _, ok := ctx.Deadline(); !ok {
		stack := []string{}
		for _, st := range skerr.CallStack(10, 1) {
			stack = append(stack, st.String())
		}
		sklog.Errorf("ctx is missing deadline at %s", stack)
	}
}
