package types

import (
	"context"
	"time"

	"go.skia.org/infra/go/cipd"
)

// CacheRequest is a request for a named cache on a machine.
type CacheRequest struct {
	Name string
	Path string
}

// TaskRequest contains all of the information necessary to request execution of
// a Task.
type TaskRequest struct {
	Caches              []*CacheRequest
	CasInput            string
	CipdPackages        []*cipd.Package
	Command             []string
	Dimensions          []string
	Env                 map[string]string
	EnvPrefixes         map[string][]string
	ExecutionTimeout    time.Duration
	Expiration          time.Duration
	ExtraArgs           []string
	Idempotent          bool
	IoTimeout           time.Duration
	Name                string
	Outputs             []string
	ServiceAccount      string
	Tags                []string
	TaskSchedulerTaskID string
}

// TaskResult describes a Task after it has been triggered.
type TaskResult struct {
	CasOutput string
	Created   time.Time
	Finished  time.Time
	ID        string
	MachineID string
	Started   time.Time
	Status    TaskStatus
	Tags      map[string][]string
}

// Machine describes a machine which can run tasks.
type Machine struct {
	ID            string
	Dimensions    []string
	IsDead        bool
	IsQuarantined bool
	CurrentTaskID string
}

// TaskExecutor is a framework for executing Tasks.
type TaskExecutor interface {
	// GetFreeMachines returns all of the machines in the given pool which are
	// not currently running a task.
	GetFreeMachines(ctx context.Context, pool string) ([]*Machine, error)
	// GetPendingTasks returns all of the tasks in the given pool which have not
	// yet started.
	GetPendingTasks(ctx context.Context, pool string) ([]*TaskResult, error)
	// GetTaskResult retrieves the result of the given task.
	GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
	// GetTaskCompletionStatuses returns a slice of bools indicating whether
	// each of the given tasks have finished.
	GetTaskCompletionStatuses(ctx context.Context, taskIDs []string) ([]bool, error)
	// TriggerTask triggers a new task to run.
	TriggerTask(ctx context.Context, req *TaskRequest) (*TaskResult, error)
}
