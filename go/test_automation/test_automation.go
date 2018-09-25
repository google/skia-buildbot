package test_automation

import (
	"context"
	"os"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// Run represents a full test automation run.
type Run struct {
	emitter *stepEmitter
	output  string
	report  *ReportReceiver
}

// New begins a new test automation Run.
func New(output string) (*Run, error) {
	// TODO(borenet): Catch SIGINT, SIGKILL and report.
	report := &ReportReceiver{}
	emitter := &stepEmitter{
		receivers: map[string]Receiver{
			"DebugReceiver": &DebugReceiver{},
		},
	}
	rv := &Run{
		emitter: emitter,
		output:  output,
	}
	if output != "" {
		emitter.receivers["ReportReceiver"] = report
		rv.report = report
	}

	return rv, nil
}

// Return a context.Context associated with this Run. Any calls to exec which
// use this Context will be attached to the currently-running step.
func (r *Run) Ctx() context.Context {
	return r.emitter.ExecCtx(context.Background())
}

// Mark the Run as finished. Perform any cleanup work.
func (r *Run) Done() {
	// TODO(borenet): Recover from panic.
	if r.output != "" {
		if r.output == "-" {
			if err := r.report.Report(os.Stdout); err != nil {
				sklog.Fatal(err)
			}
		} else {
			if err := util.WithWriteFile(r.output, r.report.Report); err != nil {
				sklog.Fatal(err)
			}
		}
	}
}

// RunStep is a convenience function which runs the given function as a Step.
func (r *Run) RunStep(fn func() error) error {
	return r.Step().Do(fn)
}
