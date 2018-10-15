package td

import (
	"context"
)

const (
	contextKeyStep = "TaskDriverStep"
	contextKeyRun  = "TaskDriverRun"
)

func getStep(ctx context.Context) *StepProperties {
	rv := ctx.Value(contextKeyStep)
	if rv == nil {
		panic("Context has no step associated with it!")
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
