package ingester

import (
	"context"
	"sort"
	"time"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/louhi"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

// NotificationToFlowExecution converts a Notification to a FlowExecution. Note
// that, since not all Notifications contain all of the information about the
// flow, the returned FlowExecution may not be complete.
func NotificationToFlowExecution(ctx context.Context, n *louhi.Notification, ts time.Time) *louhi.FlowExecution {
	var result louhi.FlowResult
	var finishedAt time.Time
	if n.EventAction == louhi.EventAction_FAILED {
		result = louhi.FlowResultFailure
		finishedAt = ts
	} else if n.EventAction == louhi.EventAction_FINISHED {
		// Note: at the time of writing, I don't know whether we get both a
		// FINISHED and a FAILED notification for a failed flow, or just the
		// FAILED notification. If the former, we may incorrectly mark the flow
		// as a success until we receive the FAILED notification.
		result = louhi.FlowResultSuccess
		finishedAt = ts
	}
	return &louhi.FlowExecution{
		Artifacts:    n.ArtifactLink,
		CreatedAt:    ts,
		FinishedAt:   finishedAt,
		FlowID:       n.FlowUniqueKey,
		FlowName:     n.FlowName,
		GeneratedCLs: n.GeneratedCls,
		GitBranch:    n.Branch,
		GitCommit:    n.RefSha,
		ID:           n.PipelineExecutionId,
		Link:         n.Link,
		ModifiedAt:   ts,
		ProjectID:    n.ProjectId,
		Result:       result,
		StartedBy:    n.StartedBy,
		TriggerType:  louhi.TriggerType(n.TriggerType),
	}
}

// Ingester is used for ingesting Louhi Notifications into a DB.
type Ingester struct {
	db     louhi.DB
	gerrit gerrit.GerritInterface
	repos  []gitiles.GitilesRepo
}

// NewIngester returns an Ingester instance.
func NewIngester(db louhi.DB, g gerrit.GerritInterface, repos []gitiles.GitilesRepo) *Ingester {
	return &Ingester{
		db:     db,
		gerrit: g,
		repos:  repos,
	}
}

// UpdateFlowFromNotification retrieves the FlowExecution from the DB, updates
// it from the Notifaction, and updates it into the DB.
func (i *Ingester) UpdateFlowFromNotification(ctx context.Context, n *louhi.Notification, ts time.Time) error {
	newFlow := NotificationToFlowExecution(ctx, n, ts)
	oldFlow, err := i.db.GetFlowExecution(ctx, newFlow.ID)
	if err != nil {
		return skerr.Wrapf(err, "failed to retrieve flow %q from DB", newFlow.ID)
	}

	// This might be the first time we've seen this flow.
	if oldFlow == nil {
		oldFlow = newFlow
	}
	if len(newFlow.Artifacts) > 0 {
		oldFlow.Artifacts = util.NewStringSet(oldFlow.Artifacts, newFlow.Artifacts).Keys()
		sort.Strings(oldFlow.Artifacts)
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
	if len(newFlow.GeneratedCLs) > 0 {
		oldFlow.GeneratedCLs = util.NewStringSet(oldFlow.GeneratedCLs, newFlow.GeneratedCLs).Keys()
		sort.Strings(oldFlow.GeneratedCLs)
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
	if oldFlow.Result == louhi.FlowResultUnknown || (oldFlow.Result == louhi.FlowResultSuccess && newFlow.Result == louhi.FlowResultFailure) {
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

	// Retrieve the CL information for the flow, but only if we haven't done so
	// yet.
	if oldFlow.SourceCL == "" && oldFlow.GitCommit != "" {
		// Retrieve the commit details. Unfortunately, the Louhi notification
		// doesn't include the repo URL, so we have to scan through our list.
		var clDetails *vcsinfo.LongCommit
		for _, repo := range i.repos {
			details, err := repo.Details(ctx, oldFlow.GitCommit)
			if err == nil {
				clDetails = details
				break
			}
		}
		if clDetails == nil {
			return skerr.Fmt("failed to retrieve CL details for commit %s", oldFlow.GitCommit)
		}
		issue, err := i.gerrit.ExtractIssueFromCommit(clDetails.Body)
		if err != nil {
			return skerr.Wrapf(err, "failed to extract issue number from commit body: %s", clDetails.Body)
		}
		oldFlow.SourceCL = i.gerrit.Url(issue)
	}

	if err := i.db.PutFlowExecution(ctx, oldFlow); err != nil {
		return skerr.Wrapf(err, "failed to update flow %q in DB", n.PipelineExecutionId)
	}
	return nil
}
