package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/perf/go/autobisection/mocks"
	pb "go.skia.org/infra/perf/go/autobisection/proto/v1"
	"go.skia.org/infra/perf/go/autobisection/sqlautobisectionstore/schema"
)

func TestSaveAutobisection(t *testing.T) {
	ctx := context.Background()
	mockStore := mocks.NewStore(t)
	s := New(mockStore)

	req := &pb.SaveAutobisectionRequest{
		JobId:            "job123",
		WorkflowId:       "wf123",
		AnomalyGroupId:   "ag123",
		AnomalyId:        "a123",
		RegressionStatus: pb.RegressionStatus_NO_CULPRIT_FOUND,
	}

	expectedSchema := &schema.AutobisectionSchema{
		JobID:            "job123",
		WorkflowID:       "wf123",
		AnomalyGroupID:   "ag123",
		AnomalyId:        "a123",
		RegressionStatus: pb.RegressionStatus_NO_CULPRIT_FOUND.String(),
	}

	mockStore.On("Save", mock.Anything, expectedSchema).Return(nil)

	resp, err := s.SaveAutobisection(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	mockStore.AssertExpectations(t)
}
