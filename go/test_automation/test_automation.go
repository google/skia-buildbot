package test_automation

import (
	"os"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// Special ID of the root step.
	STEP_ID_ROOT = "root"
)

// run represents a full test automation run.
type run struct {
	emitter *stepEmitter
	output  string
	report  *ReportReceiver
}

// New begins a new test automation run.
func New(output string) (*Step, error) {
	// TODO(borenet): Catch SIGINT, SIGKILL and report.
	report := &ReportReceiver{}
	emitter := &stepEmitter{
		receivers: map[string]Receiver{
			"DebugReceiver": &DebugReceiver{},
		},
	}
	r := &run{
		emitter: emitter,
		output:  output,
	}
	if output != "" {
		emitter.receivers["ReportReceiver"] = report
		r.report = report
	}
	rv := newStep("root", r, nil).Start()
	return rv, nil
}

// Perform any cleanup work for the run.
func (r *run) Done() {
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
