package skia

import (
	"context"
	"flag"
	"strings"

	"github.com/google/shlex"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/mcp/services/skia/task_details"
	"go.skia.org/infra/mcp/services/skia/task_scheduler"
)

type SkiaService struct {
	tsClient          *task_scheduler.TaskSchedulerClient
	taskDetailsClient *task_details.TaskDetailsClient
}

// Initialize the service with the provided arguments.
func (s *SkiaService) Init(serviceArgs string) error {
	ctx := context.TODO()

	fs := flag.NewFlagSet("SkiaService", flag.ContinueOnError)
	firestoreInstance := fs.String("firestore_instance", "production", "Firestore DB instance to use.")
	btProject := fs.String("bigtable_project", "", "BigTable project to use.")
	btInstance := fs.String("bigtable_instance", "", "BigTable instance to use.")

	// Args are passed in as a quoted string.
	args, err := shlex.Split(strings.Trim(serviceArgs, "\"'"))
	if err != nil {
		return skerr.Wrapf(err, "invalid service args: %q", serviceArgs)
	}

	if err := fs.Parse(args); err != nil {
		return skerr.Wrap(err)
	}

	tsClient, err := task_scheduler.NewClient(ctx, *firestoreInstance)
	if err != nil {
		return skerr.Wrap(err)
	}
	s.tsClient = tsClient

	taskDetailsClient, err := task_details.NewClient(ctx, *btProject, *btInstance, *firestoreInstance)
	if err != nil {
		return skerr.Wrap(err)
	}
	s.taskDetailsClient = taskDetailsClient
	return nil
}

// GetTools returns the supported tools by the service.
func (s SkiaService) GetTools() []common.Tool {
	var tools []common.Tool
	// Task results via Task Scheduler DB.
	tools = append(tools, task_scheduler.GetTools(s.tsClient)...)
	// Task steps and logs.
	tools = append(tools, task_details.GetTools(s.taskDetailsClient)...)

	// TODO(b/491418947): Perf results.
	// TODO(b/491418947): Gold results.
	// TODO(b/491418947): Autoroller statuses.
	// TODO(b/491418947): Git history.
	return tools
}

func (s *SkiaService) GetResources() []common.Resource {
	return nil
}

func (s *SkiaService) Shutdown() error {
	return nil
}
