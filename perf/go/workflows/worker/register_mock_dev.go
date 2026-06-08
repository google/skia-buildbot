//go:build dev

package main

import (
	"context"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/pinpoint"
	"go.temporal.io/sdk/worker"
)

type MockPinpointClient struct{}

func (m *MockPinpointClient) CreateBisect(
	ctx context.Context,
	req *pinpoint.BisectJobCreateRequest,
	isNewAnomaly bool,
) (*pinpoint.CreatePinpointResponse, error) {
	sklog.Infof("[MockPinpoint] CreateBisect received request: %+v", req)
	return &pinpoint.CreatePinpointResponse{
		JobID: "fedcba98-7654-3210-fedc-ba9876543210",
	}, nil
}

func (m *MockPinpointClient) FetchJobState(
	ctx context.Context,
	req pinpoint.FetchJobStateRequest,
) (*pinpoint.FetchJobStateResponse, error) {
	sklog.Infof("[MockPinpoint] FetchJobState polling for job: %s", req.JobID)
	diffCount := 1
	return &pinpoint.FetchJobStateResponse{
		JobID:           req.JobID,
		Status:          "completed",
		DifferenceCount: &diffCount,
		State: []pinpoint.StateItem{
			{
				Comparisons: map[string]string{
					"prev": "different",
				},
				Change: pinpoint.Change{
					Commits: []pinpoint.Commit{
						{
							Repository: "chromium",
							GitHash:    "dummy_culprit_hash_987654321",
						},
					},
				},
			},
		},
	}, nil
}

func registerMockActivities(w worker.Worker) {
	sklog.Infof("Registering Mock Pinpoint Client activities.")
	w.RegisterActivity(&MockPinpointClient{})
}

const devMode = true
