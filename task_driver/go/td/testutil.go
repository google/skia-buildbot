package td

import (
	"context"
	"os"
	"path/filepath"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sktest"
)

type TestingRun struct {
	t       sktest.TestingT
	ctx     context.Context
	wd      string
	report  *ReportReceiver
	cleanup func()
}

// StartTestRun returns a root-level Step to be used for testing. This is
// an alternative so that we don't need to call Init() in testing.
func StartTestRun(t sktest.TestingT) *TestingRun {
	return StartTestRunWithContext(t, context.Background())
}

// StartTestRunWithContext is like StartTestRun but uses the given Context.
func StartTestRunWithContext(t sktest.TestingT, ctx context.Context) *TestingRun {
	wd, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	output := filepath.Join(wd, "output.json")
	report := newReportReceiver(output)
	ctx, cancel := context.WithCancel(ctx)
	return &TestingRun{
		t:      t,
		ctx:    newRun(ctx, report, "fake-task-id", "fake-test-task", &RunProperties{Local: true}),
		wd:     wd,
		report: report,
		cleanup: func() {
			require.NoError(t, os.RemoveAll(wd))
			cancel()
		},
	}
}

// Finish the test Step and return its results.
func (r *TestingRun) EndRun(expectPanic bool, err *error) *StepReport {
	return r.finishRun(expectPanic, err, recover())
}

func (r *TestingRun) finishRun(expectPanic bool, err *error, recovered interface{}) (rv *StepReport) {
	defer func() {
		require.NoError(r.t, getCtx(r.ctx).run.Close())
	}()
	if expectPanic {
		require.NotNil(r.t, recovered)
	} else {
		require.Nil(r.t, recovered)
	}
	// stepFinished re-raises panics, to ensure that they continue to
	// propagate. Since this is the top level and we don't want our tests
	// to die, recover the panic.
	if expectPanic {
		defer func() {
			if rec := recover(); rec != nil {
				sklog.Infof("Recovered panic: %v", rec)
			}
			rv = r.report.root
		}()
	}
	finishStep(r.ctx, recovered)
	return r.report.root
}

// Cleanup the TestingRun.
func (r *TestingRun) Cleanup() {
	r.cleanup()
}

// Return the root-level Step.
func (r *TestingRun) Root() context.Context {
	return r.ctx
}

// Return the temporary dir used for this TestingRun.
func (r *TestingRun) Dir() string {
	return r.wd
}

// RunTestSteps runs testing steps inside the given context.
func RunTestSteps(t sktest.TestingT, expectPanic bool, fn func(context.Context) error) (rv *StepReport) {
	return RunTestStepsWithContext(t, context.Background(), expectPanic, fn)
}

// RunTestStepsWithContext is the same as RunTestSteps but uses the given context.
func RunTestStepsWithContext(t sktest.TestingT, ctx context.Context, expectPanic bool, fn func(context.Context) error) (rv *StepReport) {
	tr := StartTestRunWithContext(t, ctx)
	defer tr.Cleanup()
	var err error
	defer func() {
		rv = tr.finishRun(expectPanic, &err, recover())
	}()
	err = fn(tr.Root())
	return
}
