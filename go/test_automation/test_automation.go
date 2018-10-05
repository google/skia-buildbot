package test_automation

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/logging"
	"github.com/pborman/uuid"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// Special ID of the root step.
	STEP_ID_ROOT = "root"
)

// run represents a full test automation run.
type run struct {
	done    func()
	emitter *stepEmitter
	output  string
	report  *ReportReceiver
}

// Init begins a new test automation run.
func Init(projectId, taskId, taskName, output string, local bool) (*Step, error) {
	// TODO(borenet): Catch SIGINT, SIGKILL and report.
	// Prevent clobbering real task data.
	if local {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		taskId = fmt.Sprintf("%s_%s", hostname, uuid.New())
	}
	if projectId == "" {
		return nil, fmt.Errorf("Project ID is required.")
	}
	if taskId == "" {
		return nil, fmt.Errorf("Task ID is required.")
	}
	if taskName == "" {
		return nil, fmt.Errorf("Task name is required.")
	}
	report := &ReportReceiver{}
	ts, err := auth.NewDefaultTokenSource(local, logging.WriteScope)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	cloudLogging, err := NewCloudLoggingReceiver(ctx, projectId, taskId, taskName, ts)
	if err != nil {
		return nil, err
	}
	pubSub, err := NewPubSubReceiver(ctx, projectId)
	emitter := &stepEmitter{
		receivers: map[string]Receiver{
			"CloudLoggingReceiver": cloudLogging,
			"DebugReceiver":        &DebugReceiver{},
			"PubSubReceiver":       pubSub,
		},
		taskId: taskId,
	}
	r := &run{
		done: func() {
			if err := cloudLogging.Flush(); err != nil {
				sklog.Error(err)
			}
		},
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
	defer r.done()
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
