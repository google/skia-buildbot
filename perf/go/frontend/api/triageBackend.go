package api

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	perf_issuetracker "go.skia.org/infra/perf/go/issuetracker"
)

// TriageBackend defines the interface for triaging operations.
type TriageBackend interface {
	FileBug(ctx context.Context, req *perf_issuetracker.FileBugRequest) (*SkiaFileBugResponse, error)
	EditAnomalies(ctx context.Context, req *EditAnomaliesRequest) (*EditAnomaliesResponse, error)
	AssociateAlerts(ctx context.Context, req *SkiaAssociateBugRequest) (*SkiaAssociateBugResponse, error)
}

type triageBackend struct {
	issueTracker perf_issuetracker.IssueTracker
}

func NewTriageBackend(issueTracker perf_issuetracker.IssueTracker) TriageBackend {
	return &triageBackend{
		issueTracker: issueTracker,
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
	return nil, skerr.Fmt("unimplemented call to associate alerts")
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
