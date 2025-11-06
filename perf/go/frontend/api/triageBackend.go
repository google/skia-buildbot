package api

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// TriageBackend defines the interface for triaging operations.
type TriageBackend interface {
	FileBug(ctx context.Context, req *FileBugRequest) (*SkiaFileBugResponse, error)
	EditAnomalies(ctx context.Context, req *EditAnomaliesRequest) (*EditAnomaliesResponse, error)
	AssociateAlerts(ctx context.Context, req *SkiaAssociateBugRequest) (*SkiaAssociateBugResponse, error)
}

type triageBackend struct {
}

func NewTriageBackend() TriageBackend {
	return &triageBackend{}
}

func (t *triageBackend) FileBug(ctx context.Context, req *FileBugRequest) (*SkiaFileBugResponse, error) {
	panic("unimplemented!")
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
	panic("unimplemented!")
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
