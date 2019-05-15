package td

import (
	"context"
)

const (
	contextKeyStep = "TaskDriverStep"
	contextKeyRun  = "TaskDriverRun"
	contextKeyEnv  = "TaskDriverEnv"
)

func getStep(ctx context.Context) *StepProperties {
	rv := safeGetStep(ctx)
	if rv == nil {
		panic("Context has no step associated with it!")
	}
	return rv
}

func safeGetStep(ctx context.Context) *StepProperties {
	rv := ctx.Value(contextKeyStep)
	if rv == nil {
		return nil
	}
	return rv.(*StepProperties)
}

func setStep(ctx context.Context, s *StepProperties) context.Context {
	return context.WithValue(ctx, contextKeyStep, s)
}

func getRun(ctx context.Context) *run {
	rv := ctx.Value(contextKeyRun)
	if rv == nil {
		panic("Context has no run associated with it!")
	}
	return rv.(*run)
}

func setRun(ctx context.Context, r *run) context.Context {
	return context.WithValue(ctx, contextKeyRun, r)
}

func getEnv(ctx context.Context) []string {
	rv := ctx.Value(contextKeyEnv)
	if rv == nil {
		return nil
	}
	return rv.([]string)
}

func setEnv(ctx context.Context, env []string) context.Context {
	return context.WithValue(ctx, contextKeyEnv, env)
}

// Set the given environment on the Context. Steps which use the Context will
// inherit the environment variables. Merges with any previous calls to SetEnv.
func SetEnv(ctx context.Context, env []string) context.Context {
	ctx = setEnv(ctx, MergeEnv(getEnv(ctx), env))
	// Required for exec to inherit the env.
	ctx = execCtx(ctx)
	return ctx
}

// derive the environment for a given step, based on any environment set via
// SetEnv, and any parent step.
func deriveEnv(ctx context.Context) []string {
	rv := []string{}
	rv = MergeEnv(rv, getEnv(ctx))
	parent := safeGetStep(ctx)
	if parent != nil {
		rv = MergeEnv(rv, parent.Environ)
	}
	return rv
}
