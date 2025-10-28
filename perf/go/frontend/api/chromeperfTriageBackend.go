package api

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/chromeperf"
)

// ChromeperfTriageBackend implements the TriageBackend interface using the chromeperf client.
type ChromeperfTriageBackend struct {
	chromeperfClient chromeperf.ChromePerfClient
}

var acceptedStatusCodes = []int{200, 400, 401, 500}

func NewChromeperfTriageBackend(client chromeperf.ChromePerfClient) *ChromeperfTriageBackend {
	return &ChromeperfTriageBackend{
		chromeperfClient: client,
	}
}

func (b *ChromeperfTriageBackend) FileBug(ctx context.Context, req *FileBugRequest) (*SkiaFileBugResponse, error) {
	chromeperfResponse := &ChromeperfFileBugResponse{}
	err := b.chromeperfClient.SendPostRequest(ctx, "file_bug_skia", "", req, chromeperfResponse, acceptedStatusCodes)
	if err != nil {
		return nil, skerr.Wrapf(err, "File new bug request failed due to an internal server error. Please try again.")
	}
	if chromeperfResponse.Error != "" {
		return nil, skerr.Fmt("Error when filing a new bug. Please double check each request parameter, and try again: %v", chromeperfResponse.Error)
	}
	return &SkiaFileBugResponse{BugId: chromeperfResponse.BugId}, nil
}

func (b *ChromeperfTriageBackend) EditAnomalies(ctx context.Context, req *EditAnomaliesRequest) (*EditAnomaliesResponse, error) {
	chromeperfResponse := &EditAnomaliesResponse{}
	err := b.chromeperfClient.SendPostRequest(ctx, "edit_anomalies_skia", "", req, chromeperfResponse, acceptedStatusCodes)
	if err != nil {
		return nil, skerr.Wrapf(err, "Edit anomalies request failed due to an internal server error. Please try again.")
	}
	if chromeperfResponse.Error != "" {
		return nil, skerr.Fmt("Error when editing anomalies: %s. Please double check each request parameter, and try again.", chromeperfResponse.Error)
	}
	return chromeperfResponse, nil
}

func (b *ChromeperfTriageBackend) AssociateAlerts(ctx context.Context, req *SkiaAssociateBugRequest) (*SkiaAssociateBugResponse, error) {
	skiaExistingBugResponse := &ChromeperfAssociateBugResponse{}
	err := b.chromeperfClient.SendPostRequest(ctx, "associate_alerts_skia", "", req, skiaExistingBugResponse, acceptedStatusCodes)
	if err != nil {
		return nil, skerr.Wrapf(err, "Associate alerts request failed due to an internal server error. Please try again.")
	}
	if skiaExistingBugResponse.Error != "" {
		return nil, skerr.Fmt("Error when associating alerts with an existing bug. Please double check each request parameter, and try again. %v", skiaExistingBugResponse.Error)
	}
	return &SkiaAssociateBugResponse{BugId: req.BugId}, nil
}
