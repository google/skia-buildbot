package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/perf/go/culprit/mocks"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
)

func setUp(_ *testing.T) (*culpritService, *mocks.Store) {
	mockstore := new(mocks.Store)
	service := New(mockstore)
	return service, mockstore
}

func TestPersistCulprit_ValidInput_ShouldInvokeStoreUpsert(t *testing.T) {
	c, store := setUp(t)
	ctx := context.Background()
	culprits := []*pb.Culprit{{
		Commit: &pb.Commit{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main",
			Revision: "123",
		},
	}, {
		Commit: &pb.Commit{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main1",
			Revision: "456",
		},
	}}
	req := &pb.PersistCulpritRequest{
		Culprits: culprits, AnomalyGroupId: "111",
	}
	store.On("Upsert", mock.Anything, "111", culprits).Return(nil)

	_, err := c.PersistCulprit(ctx, req)

	// assert that the expectations were met
	store.AssertExpectations(t)
	assert.Nil(t, err)
}
