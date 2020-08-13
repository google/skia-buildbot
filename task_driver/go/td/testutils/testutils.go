package testutils

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/task_driver/go/td"
	"go.skia.org/infra/task_driver/go/td/properties"
)

type TestingRun struct {
	t       sktest.TestingT
	ctx     context.Context
	wd      string
	report  *td.ReportReceiver
	cleanup func()
}

// StartTestRun returns a root-level Step to be used for testing. This is
// an alternative so that we don't need to call Init() in testing.
func StartTestRun(t sktest.TestingT) *TestingRun {
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	output := filepath.Join(wd, "output.json")
	report := td.NewReportReceiver(output)
	return &TestingRun{
		t:      t,
		ctx:    td.NewRun(context.Background(), report, "fake-task-id", "fake-test-task", &properties.RunProperties{Local: true}),
		wd:     wd,
		report: report,
		cleanup: func() {
			require.NoError(t, os.RemoveAll(wd))
		},
	}
}

// Finish the test Step and return its results.
func (r *TestingRun) EndRun(expectPanic bool, err *error) *td.StepReport {
	return r.finishRun(expectPanic, err, recover())
}

func (r *TestingRun) finishRun(expectPanic bool, err *error, recovered interface{}) (rv *td.StepReport) {
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
			rv = r.report.Root()
		}()
	}
	finishStep(r.ctx, recovered)
	return r.report.Root()
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
func RunTestSteps(t sktest.TestingT, expectPanic bool, fn func(context.Context) error) (rv *td.StepReport) {
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
