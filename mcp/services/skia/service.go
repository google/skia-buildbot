package skia

import (
	"context"
	"flag"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/mcp/services/skia/task_scheduler"
)

type SkiaService struct {
	tsClient *task_scheduler.TaskSchedulerClient
}

// Initialize the service with the provided arguments.
func (s *SkiaService) Init(serviceArgs string) error {
	fs := flag.NewFlagSet("SkiaService", flag.ContinueOnError)
	firestoreInstance := fs.String("firestore_instance", "production", "Firestore DB instance to use.")
	if err := fs.Parse(strings.Fields(serviceArgs)); err != nil {
		return skerr.Wrap(err)
	}
	tsClient, err := task_scheduler.NewClient(context.TODO(), *firestoreInstance)
	if err != nil {
		return skerr.Wrap(err)
	}
	s.tsClient = tsClient
	return nil
}

// GetTools returns the supported tools by the service.
func (s SkiaService) GetTools() []common.Tool {
	var tools []common.Tool
	// Task results via Task Scheduler DB.
	tools = append(tools, task_scheduler.GetTools(s.tsClient)...)

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
