package louhi

// Generate the go code from the protocol buffer definitions.
//go:generate bazelisk run --config=mayberemote //:protoc -- --go_opt=paths=source_relative --go_out=. ./louhi.proto
//go:generate rm -rf ./go.skia.org
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w louhi.pb.go

import (
	"context"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
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
	Artifacts   []string
	CreatedAt   time.Time
	FinishedAt  time.Time
	FlowName    string
	FlowID      string
	GitBranch   string
	GitCommit   string
	ID          string
	Link        string
	ModifiedAt  time.Time
	ProjectID   string
	Result      FlowResult
	StartedBy   string
	TriggerType TriggerType
}

// Finished returns true if the flow has finished.
func (fe *FlowExecution) Finished() bool {
	return fe.Result != FlowResultUnknown
}

// NotificationToFlowExecution converts a Notification to a FlowExecution. Note
// that, since not all Notifications contain all of the information about the
// flow, the returned FlowExecution may not be complete.
func NotificationToFlowExecution(ctx context.Context, n *Notification, ts time.Time) *FlowExecution {
	var result FlowResult
	var finishedAt time.Time
	if n.EventAction == EventAction_FAILED {
		result = FlowResultFailure
		finishedAt = ts
	} else if n.EventAction == EventAction_FINISHED {
		// Note: at the time of writing, I don't know whether we get both a
		// FINISHED and a FAILED notification for a failed flow, or just the
		// FAILED notification. If the former, we may incorrectly mark the flow
		// as a success until we receive the FAILED notification.
		result = FlowResultSuccess
		finishedAt = ts
	}
	return &FlowExecution{
		Artifacts:   n.ArtifactLink,
		CreatedAt:   ts,
		FinishedAt:  finishedAt,
		FlowID:      n.FlowUniqueKey,
		FlowName:    n.FlowName,
		GitBranch:   n.Branch,
		GitCommit:   n.RefSha,
		ID:          n.PipelineExecutionId,
		Link:        n.Link,
		ModifiedAt:  ts,
		ProjectID:   n.ProjectId,
		Result:      result,
		StartedBy:   n.StartedBy,
		TriggerType: TriggerType(n.TriggerType),
	}
}

// UpdateFlowFromNotification retrieves the FlowExecution from the DB, updates
// it from the Notifaction, and updates it into the DB.
func UpdateFlowFromNotification(ctx context.Context, db DB, n *Notification, ts time.Time) error {
	newFlow := NotificationToFlowExecution(ctx, n, ts)
	oldFlow, err := db.GetFlowExecution(ctx, newFlow.ID)
	if err != nil {
		return skerr.Wrapf(err, "failed to retrieve flow %q from DB", newFlow.ID)
	}

	// This might be the first time we've seen this flow.
	if oldFlow == nil {
		oldFlow = newFlow
	}
	if len(newFlow.Artifacts) > 0 {
		oldFlow.Artifacts = util.NewStringSet(oldFlow.Artifacts, newFlow.Artifacts).Keys()
	}
	if util.TimeIsZero(oldFlow.CreatedAt) || (!util.TimeIsZero(newFlow.CreatedAt) && newFlow.CreatedAt.Before(oldFlow.CreatedAt)) {
		oldFlow.CreatedAt = newFlow.CreatedAt
	}
	if oldFlow.FlowName == "" {
		oldFlow.FlowName = newFlow.FlowName
	}
	if oldFlow.FlowID == "" {
		oldFlow.FlowID = newFlow.FlowID
	}
	if oldFlow.GitBranch == "" {
		oldFlow.GitBranch = newFlow.GitBranch
	}
	if oldFlow.GitCommit == "" {
		oldFlow.GitCommit = newFlow.GitCommit
	}
	// Note: this should never happen, since we use PipelineExecutionId as the
	// database ID, so it'll be populated if it made it into the DB.
	if oldFlow.ID == "" {
		oldFlow.ID = newFlow.ID
	}
	if oldFlow.Link == "" {
		oldFlow.Link = newFlow.Link
	}
	if oldFlow.ProjectID == "" {
		oldFlow.ProjectID = newFlow.ProjectID
	}
	if oldFlow.Result == FlowResultUnknown || (oldFlow.Result == FlowResultSuccess && newFlow.Result == FlowResultFailure) {
		oldFlow.Result = newFlow.Result
		oldFlow.FinishedAt = newFlow.FinishedAt
	}
	if oldFlow.StartedBy == "" {
		oldFlow.StartedBy = newFlow.StartedBy
	}
	if oldFlow.TriggerType == "" {
		oldFlow.TriggerType = newFlow.TriggerType
	}
	if newFlow.ModifiedAt.After(oldFlow.ModifiedAt) {
		oldFlow.ModifiedAt = newFlow.ModifiedAt
	}

	if err := db.PutFlowExecution(ctx, oldFlow); err != nil {
		return skerr.Wrapf(err, "failed to update flow %q in DB", n.PipelineExecutionId)
	}
	return nil
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
