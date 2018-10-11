package task_driver

import (
	"context"
	"path/filepath"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

type TestingRun struct {
	t       testutils.TestingT
	ctx     context.Context
	wd      string
	report  *ReportReceiver
	cleanup func()
}

// InitForTesting returns a root-level Step to be used for testing. This is
// an alternative so that we don't need to call Init() in testing.
func InitForTesting(t testutils.TestingT) *TestingRun {
	wd, cleanup := testutils.TempDir(t)
	output := filepath.Join(wd, "output.json")
	report := newReportReceiver(output)
	emitter := newStepEmitter("fake-task-id", map[string]Receiver{
		"ReportReceiver": report,
	})
	return &TestingRun{
		t:       t,
		ctx:     newRun(emitter),
		wd:      wd,
		report:  report,
		cleanup: cleanup,
	}
}

// Finish the test Step and return its results.
func (r *TestingRun) RunFinished(expectPanic bool, err *error) *StepReport {
	return r.runFinished(expectPanic, err, recover())
}

func (r *TestingRun) runFinished(expectPanic bool, err *error, recovered interface{}) (rv *StepReport) {
	defer getRun(r.ctx).done()
	if expectPanic {
		assert.NotNil(r.t, recovered)
	} else {
		assert.Nil(r.t, recovered)
	}
	// stepFinished re-raises panics, to ensure that they continue to
	// propagate. Since this is the top level and we don't want our tests
	// to die, recover the panic.
	if expectPanic {
		defer func() {
			recover()
			rv = r.report.root
		}()
	}
	finishStep(r.ctx, err, recovered)
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

// Run testing steps inside the given context.
func RunTestSteps(t testutils.TestingT, expectPanic bool, fn func(context.Context) error) (rv *StepReport) {
	tr := InitForTesting(t)
	defer tr.Cleanup()
	var err error
	defer func() {
		rv = tr.runFinished(expectPanic, &err, recover())
	}()
	ctx := tr.Root()
	err = fn(ctx)
	return
}
