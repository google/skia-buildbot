package task_driver

import (
	"path/filepath"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
)

type TestingRun struct {
	t       testutils.TestingT
	s       *Step
	wd      string
	cleanup func()
}

// InitForTesting returns a root-level Step to be used for testing. This is
// an alternative so that we don't need to call Init() in testing.
func InitForTesting(t testutils.TestingT) *TestingRun {
	wd, cleanup := testutils.TempDir(t)
	output := filepath.Join(wd, "output.json")
	report := newReportReceiver(output)
	r := &run{
		done: func() {},
		emitter: newStepEmitter("fake-task-id", map[string]Receiver{
			"ReportReceiver": report,
		}),
		report: report,
	}
	s := newStep(STEP_ID_ROOT, r, nil).Start()
	return &TestingRun{
		t:       t,
		s:       s,
		wd:      wd,
		cleanup: cleanup,
	}
}

// Finish the test Step and return its results.
func (r *TestingRun) Finish(err *error) *StepReport {
	r.s.Done(err)
	assert.NoError(r.t, r.s.run.report.Report())
	return r.s.run.report.root
}

// Cleanup the TestingRun.
func (r *TestingRun) Cleanup() {
	r.cleanup()
}

// Return the root-level Step.
func (r *TestingRun) Root() *Step {
	return r.s
}

// Return the temporary dir used for this TestingRun.
func (r *TestingRun) Dir() string {
	return r.wd
}

// Run testing steps inside the given context.
func RunTestSteps(t testutils.TestingT, fn func(*Step) error) *StepReport {
	tr := InitForTesting(t)
	defer tr.Cleanup()
	s := tr.Root()
	err := fn(s)
	return tr.Finish(&err)
}
