package td

import (
	"context"

	"go.skia.org/infra/go/exec"
)

const (
	contextKey = "TaskDriverContext"
)

type Context struct {
	// run is always non-nil.
	run    *run
	parent *Context

	// Only one of the below is set.

	// Current step.
	step *StepProperties

	// Environment variables, set via SetEnv.
	env []string

	// execRun provides a Run function to be called by execCtx. This is used
	// for testing, where we may want to mock out subprocess invocations.
	execRun func(*exec.Command) error
}

func getCtx(ctx context.Context) *Context {
	rv := ctx.Value(contextKey)
	if rv == nil {
		panic("No Context!")
	}
	return rv.(*Context)
}

func setCtx(ctx context.Context, child *Context) context.Context {
	parent := getCtx(ctx)
	child.parent = parent
	child.run = parent.run
	ctx = context.WithValue(ctx, contextKey, child)
	// Any time we set the parent step, env, or execRun, we need to set a
	// new execCtx to ensure that exec has access to the new information.
	ctx = execCtx(ctx)
	return ctx
}

func safeGetStep(ctx context.Context) *StepProperties {
	c := getCtx(ctx)
	for c.step == nil && c.parent != nil {
		c = c.parent
	}
	return c.step
}

func getStep(ctx context.Context) *StepProperties {
	rv := safeGetStep(ctx)
	if rv == nil {
		panic("Context has no step associated with it!")
	}
	return rv
}

func setStep(ctx context.Context, s *StepProperties) context.Context {
	return setCtx(ctx, &Context{
		step: s,
	})
}

func getRun(ctx context.Context) *run {
	c := getCtx(ctx)
	if c.run == nil {
		panic("Context has no run associated with it!")
	}
	return c.run
}

func setRun(ctx context.Context, r *run) context.Context {
	// Special case; this is always a root-level Context.
	return context.WithValue(ctx, contextKey, &Context{
		run: r,
	})
}

// Set the given environment on the Context. Steps which use the Context will
// inherit the environment variables. Merges with any previous calls to SetEnv.
func SetEnv(ctx context.Context, env []string) context.Context {
	return setCtx(ctx, &Context{
		env: env,
	})
}

// derive the environment for a given step, based on any environment set via
// SetEnv, and any parent step.
func deriveEnv(ctx context.Context, stepEnv []string) []string {
	rv := stepEnv
	c := getCtx(ctx)
	for c.step == nil && c.parent != nil {
		if c.env != nil {
			rv = MergeEnv(c.env, rv)
		}
		c = c.parent
	}
	if c.step != nil {
		rv = MergeEnv(c.step.Environ, rv)
	}
	return rv
}

// SetExecRunFn allows the Run function to be overridden for testing.
func SetExecRunFn(ctx context.Context, run func(*exec.Command) error) context.Context {
	return setCtx(ctx, &Context{
		execRun: run,
	})
}

// getExecRunFn retrieves any Run function which was set on the context.
func getExecRunFn(ctx context.Context) func(*exec.Command) error {
	c := getCtx(ctx)
	for c != nil {
		if c.execRun != nil {
			return c.execRun
		}
		c = c.parent
	}
	return exec.DefaultRun
}
