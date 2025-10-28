package api

import (
	"context"
)

// TriageBackend defines the interface for triaging operations.
type TriageBackend interface {
	FileBug(ctx context.Context, req *FileBugRequest) (*SkiaFileBugResponse, error)
	EditAnomalies(ctx context.Context, req *EditAnomaliesRequest) (*EditAnomaliesResponse, error)
	AssociateAlerts(ctx context.Context, req *SkiaAssociateBugRequest) (*SkiaAssociateBugResponse, error)
}
