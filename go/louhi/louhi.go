package louhi

// Generate the go code from the protocol buffer definitions.
//go:generate bazelisk run --config=mayberemote //:protoc -- --go_opt=paths=source_relative --go_out=. ./louhi.proto
//go:generate rm -rf ./go.skia.org
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w louhi.pb.go

import (
	"context"
	"time"
)

// TriggerType describes how a flow was triggered.
type TriggerType string

const (
	TriggerTypeCommit = "git-change-trigger"
	TriggerTypeCron   = "cron-trigger"
	TriggerTypeManual = "MANUAL"
)

// FlowResult describes the result of a flow.
type FlowResult string

const (
	FlowResultUnknown = ""
	FlowResultSuccess = "success"
	FlowResultFailure = "failure"
)

// FlowExecution describes one instance of a Louhi flow.
type FlowExecution struct {
	Artifacts    []string
	CreatedAt    time.Time
	FinishedAt   time.Time
	FlowName     string
	FlowID       string
	GeneratedCLs []string
	GitBranch    string
	GitCommit    string
	ID           string
	Link         string
	ModifiedAt   time.Time
	ProjectID    string
	Result       FlowResult
	SourceCL     string
	StartedBy    string
	TriggerType  TriggerType
}

// Finished returns true if the flow has finished.
func (fe *FlowExecution) Finished() bool {
	return fe.Result != FlowResultUnknown
}

// DB stores information about Louhi flows.
type DB interface {
	// PutFlowExecution inserts or updates the FlowExecution in the DB.
	PutFlowExecution(ctx context.Context, fe *FlowExecution) error
	// GetFlowExecution retrieves the FlowExecution from the DB. Should return
	// nil and no error if the flow with the given ID does not exist.
	GetFlowExecution(ctx context.Context, id string) (*FlowExecution, error)
	// GetLatestFlowExecutions retrieves the most recent flow executions by
	// flow name.
	GetLatestFlowExecutions(ctx context.Context) (map[string]*FlowExecution, error)
}
