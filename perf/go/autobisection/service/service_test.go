package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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
		AnomalyGroupId:   "ag123",
		AnomalyId:        "a123",
		IsRealRegression: true,
	}

	expectedSchema := &schema.AutobisectionSchema{
		JobID:            "job123",
		AnomalyGroupID:   "ag123",
		AnomalyId:        "a123",
		IsRealRegression: true,
	}

	mockStore.On("Save", ctx, expectedSchema).Return(nil)

	resp, err := s.SaveAutobisection(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	mockStore.AssertExpectations(t)
}
