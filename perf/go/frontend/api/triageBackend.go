package api

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	perf_issuetracker "go.skia.org/infra/perf/go/issuetracker"
	"go.skia.org/infra/perf/go/regression"
)

// TriageBackend defines the interface for triaging operations.
type TriageBackend interface {
	FileBug(ctx context.Context, req *perf_issuetracker.FileBugRequest) (*SkiaFileBugResponse, error)
	EditAnomalies(ctx context.Context, req *EditAnomaliesRequest) (*EditAnomaliesResponse, error)
	AssociateAlerts(ctx context.Context, req *SkiaAssociateBugRequest) (*SkiaAssociateBugResponse, error)
}

type triageBackend struct {
	issueTracker perf_issuetracker.IssueTracker
	regStore     regression.Store
}

func NewTriageBackend(issueTracker perf_issuetracker.IssueTracker, regStore regression.Store) TriageBackend {
	return &triageBackend{
		issueTracker: issueTracker,
		regStore:     regStore,
	}
}

func (t *triageBackend) FileBug(ctx context.Context, req *perf_issuetracker.FileBugRequest) (*SkiaFileBugResponse, error) {
	// TODO(b/455571922) Perform integration tests when Associate Alerts is done.
	bugId, err := t.issueTracker.FileBug(ctx, req)
	if err != nil {
		return nil, err
	}
	_, err = t.AssociateAlerts(ctx, &SkiaAssociateBugRequest{
		BugId:      bugId,
		Keys:       req.Keys,
		TraceNames: req.TraceNames,
	})
	if err != nil {
		return &SkiaFileBugResponse{BugId: bugId}, skerr.Wrapf(err,
			`Bug with id = %d has been filed. Failed to associate %d anomalies with this bug.
			A sheriff must manually assign the newly filed bug to those anomalies, or close it.`,
			bugId, len(req.Keys))
	}
	return &SkiaFileBugResponse{BugId: bugId}, nil
}

func (t *triageBackend) EditAnomalies(ctx context.Context, req *EditAnomaliesRequest) (*EditAnomaliesResponse, error) {
	action := req.Action
	if action == "IGNORE" {
		return t.ignoreAnomalies(ctx, req)
	}
	if action == "RESET" {
		return t.resetAnomalies(ctx, req)
	}
	if action == "NUDGE" {
		return t.nudgeAnomalies(ctx, req)
	}
	sklog.Errorf("unknown edit anomalies action %s", action)
	return &EditAnomaliesResponse{}, skerr.Fmt("unknown action")
}

func (t *triageBackend) AssociateAlerts(ctx context.Context, req *SkiaAssociateBugRequest) (*SkiaAssociateBugResponse, error) {
	// Shortcircuit checks to avoid sending unnecessary requests through Issuetracker.
	// BugIDs should be positive.
	if req.BugId <= 0 {
		return nil, skerr.Fmt("BugId must be a positive integer")
	}
	if len(req.Keys) == 0 {
		return nil, skerr.Fmt("Keys are required")
	}
	// Verify the issue exists
	issue, err := t.issueTracker.ListIssues(ctx, perf_issuetracker.ListIssuesRequest{IssueIds: []int{req.BugId}})
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to list issue with bug_id = %d", req.BugId)
	}
	if len(issue) == 0 {
		return nil, skerr.Fmt("Failed to associate alert with a non-existent issue")
	}

	if err := t.regStore.SetBugID(ctx, req.Keys, req.BugId); err != nil {
		return nil, skerr.Wrapf(err, "failed to associate alerts with bug id %d", req.BugId)
	}

	return &SkiaAssociateBugResponse{req.BugId}, nil
}

func (t *triageBackend) ignoreAnomalies(ctx context.Context, req *EditAnomaliesRequest) (*EditAnomaliesResponse, error) {
	panic("unimplemented")
}

func (t *triageBackend) resetAnomalies(ctx context.Context, req *EditAnomaliesRequest) (*EditAnomaliesResponse, error) {
	panic("unimplemented")
}

func (t *triageBackend) nudgeAnomalies(ctx context.Context, req *EditAnomaliesRequest) (*EditAnomaliesResponse, error) {
	panic("unimplemented")
}
