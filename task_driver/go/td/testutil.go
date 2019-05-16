package td

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	assert "github.com/stretchr/testify/require"
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
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	output := filepath.Join(wd, "output.json")
	report := newReportReceiver(output)
	return &TestingRun{
		t:      t,
		ctx:    newRun(context.Background(), report, "fake-task-id", "fake-test-task", &RunProperties{Local: true}),
		wd:     wd,
		report: report,
		cleanup: func() {
			assert.NoError(t, os.RemoveAll(wd))
		},
	}
}

// Finish the test Step and return its results.
func (r *TestingRun) EndRun(expectPanic bool, err *error) *StepReport {
	return r.finishRun(expectPanic, err, recover())
}

func (r *TestingRun) finishRun(expectPanic bool, err *error, recovered interface{}) (rv *StepReport) {
	defer func() {
		assert.NoError(r.t, getCtx(r.ctx).run.Close())
	}()
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

// Run testing steps inside the given context.
func RunTestSteps(t sktest.TestingT, expectPanic bool, fn func(context.Context) error) (rv *StepReport) {
	tr := StartTestRun(t)
	defer tr.Cleanup()
	var err error
	defer func() {
		rv = tr.finishRun(expectPanic, &err, recover())
	}()
	ctx := tr.Root()
	err = fn(ctx)
	return
}
