package api

import (
	"context"
	"strconv"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/chromeperf"
	perf_issuetracker "go.skia.org/infra/perf/go/issuetracker"
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

func convertStrArrToIntArr(keys []string) ([]int, error) {
	intKeys := make([]int, len(keys))
	for i, key := range keys {
		val, err := strconv.Atoi(key)
		if err != nil {
			return nil, skerr.Wrapf(err, "invalid key format: %s", key)
		}
		intKeys[i] = val
	}
	return intKeys, nil
}

type FileBugRequestForChromePerf struct {
	Keys        []int    `json:"keys"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Component   string   `json:"component"`
	Assignee    string   `json:"assignee,omitempty"`
	Ccs         []string `json:"ccs,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	TraceNames  []string `json:"trace_names,omitempty"`
	Host        string   `json:"host,omitempty"`
}

func (b *ChromeperfTriageBackend) FileBug(ctx context.Context, req *perf_issuetracker.FileBugRequest) (*SkiaFileBugResponse, error) {
	chromeperfResponse := &ChromeperfFileBugResponse{}
	keys, err := convertStrArrToIntArr(req.Keys)
	if err != nil {
		return nil, err
	}

	chromePerfReq := &FileBugRequestForChromePerf{
		Keys:        keys,
		Title:       req.Title,
		Description: req.Description,
		Component:   req.Component,
		Assignee:    req.Assignee,
		Ccs:         req.Ccs,
		Labels:      req.Labels,
		TraceNames:  req.TraceNames,
		Host:        req.Host,
	}

	err = b.chromeperfClient.SendPostRequest(ctx, "file_bug_skia", "", chromePerfReq, chromeperfResponse, acceptedStatusCodes)
	if err != nil {
		return nil, skerr.Wrapf(err, "File new bug request failed due to an internal server error. Please try again.")
	}
	if chromeperfResponse.Error != "" {
		return nil, skerr.Fmt("Error when filing a new bug. Please double check each request parameter, and try again: %v", chromeperfResponse.Error)
	}
	return &SkiaFileBugResponse{BugId: chromeperfResponse.BugId}, nil
}

type EditAnomaliesRequestForChromePerf struct {
	Keys          []int    `json:"keys"`
	Action        string   `json:"action"`
	StartRevision int      `json:"start_revision,omitempty"`
	EndRevision   int      `json:"end_revision,omitempty"`
	TraceNames    []string `json:"trace_names"`
}

func (b *ChromeperfTriageBackend) EditAnomalies(ctx context.Context, req *EditAnomaliesRequest) (*EditAnomaliesResponse, error) {
	chromeperfResponse := &EditAnomaliesResponse{}
	keys, err := convertStrArrToIntArr(req.Keys)
	if err != nil {
		return nil, err
	}
	chromePerfReq := &EditAnomaliesRequestForChromePerf{
		Keys:          keys,
		Action:        req.Action,
		StartRevision: req.StartRevision,
		EndRevision:   req.EndRevision,
		TraceNames:    req.TraceNames,
	}

	err = b.chromeperfClient.SendPostRequest(ctx, "edit_anomalies_skia", "", chromePerfReq, chromeperfResponse, acceptedStatusCodes)
	if err != nil {
		return nil, skerr.Wrapf(err, "Edit anomalies request failed due to an internal server error. Please try again.")
	}
	if chromeperfResponse.Error != "" {
		return nil, skerr.Fmt("Error when editing anomalies: %s. Please double check each request parameter, and try again.", chromeperfResponse.Error)
	}
	return chromeperfResponse, nil
}

// The temporary object is used only because the Chrome Performance API requires the key to be of the integer data type
type SkiaAssociateBugRequestForChromePerf struct {
	BugId      int      `json:"bug_id"`
	Keys       []int    `json:"keys"`
	TraceNames []string `json:"trace_names"`
}

func (b *ChromeperfTriageBackend) AssociateAlerts(ctx context.Context, req *SkiaAssociateBugRequest) (*SkiaAssociateBugResponse, error) {
	skiaExistingBugResponse := &ChromeperfAssociateBugResponse{}
	keys, err := convertStrArrToIntArr(req.Keys)
	if err != nil {
		return nil, err
	}

	chromePerfReq := &SkiaAssociateBugRequestForChromePerf{
		BugId:      req.BugId,
		Keys:       keys,
		TraceNames: req.TraceNames,
	}

	err = b.chromeperfClient.SendPostRequest(ctx, "associate_alerts_skia", "", chromePerfReq, skiaExistingBugResponse, acceptedStatusCodes)
	if err != nil {
		return nil, skerr.Wrapf(err, "Associate alerts request failed due to an internal server error. Please try again.")
	}
	if skiaExistingBugResponse.Error != "" {
		return nil, skerr.Fmt("Error when associating alerts with an existing bug. Please double check each request parameter, and try again. %v", skiaExistingBugResponse.Error)
	}
	return &SkiaAssociateBugResponse{BugId: req.BugId}, nil
}
