package td

import (
	"context"

	"go.skia.org/infra/go/exec"
)

var (
	contextKey = &taskDriverContext{}
)

type taskDriverContext struct{}

type Context struct {
	// run is always non-nil.
	run    *run
	parent *Context

	// The below fields override those of the parent Context, if set.

	// Current step.
	step *StepProperties

	// Environment variables, set via WithEnv.
	env []string

	// execRun provides a Run function to be called by execCtx. This is used
	// for testing, where we may want to mock out subprocess invocations.
	execRun func(context.Context, *exec.Command) error
}

// getCtx retrieves the current Context. Panics if none exists.
func getCtx(ctx context.Context) *Context {
	rv := ctx.Value(contextKey)
	if rv == nil {
		panic("No Context!")
	}
	return rv.(*Context)
}

// GetEnv returns the Environment variables.
func GetEnv(ctx context.Context) []string {
	rv := ctx.Value(contextKey)
	if rv == nil {
		return []string{}
	}
	return rv.(*Context).env
}

// withChildCtx adds the new Context as a child of the existing Context.
func withChildCtx(ctx context.Context, child *Context) context.Context {
	parent := getCtx(ctx)
	child.parent = parent
	child.run = parent.run
	if child.step == nil {
		child.step = parent.step
		child.env = MergeEnv(parent.env, child.env)
	} else {
		child.step.Environ = MergeEnv(parent.env, child.step.Environ)
		// Override child.env; it shouldn't be set when adding a step.
		child.env = child.step.Environ
	}
	if child.execRun == nil {
		child.execRun = parent.execRun
	}
	ctx = context.WithValue(ctx, contextKey, child)
	// Any time we set the parent step, env, or execRun, we need to set a
	// new execCtx to ensure that exec has access to the new information.
	ctx = execCtx(ctx)
	return ctx
}

// Set the given environment on the Context. Steps which use the Context will
// inherit the environment variables. Merges with any previous calls to WithEnv.
func WithEnv(ctx context.Context, env []string) context.Context {
	return withChildCtx(ctx, &Context{
		env: env,
	})
}

// WithExecRunFn allows the Run function to be overridden for testing.
func WithExecRunFn(ctx context.Context, run func(context.Context, *exec.Command) error) context.Context {
	return withChildCtx(ctx, &Context{
		execRun: run,
	})
}
