package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mocks "go.skia.org/infra/perf/go/anomalygroup/mock"
	pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
)

func setUp(_ *testing.T) (*anomalygroupService, *mocks.Store) {
	mockstore := new(mocks.Store)
	service := New(mockstore)
	return service, mockstore
}

func TestCreateNewAnomalyGroup(t *testing.T) {
	service, store := setUp(t)
	ctx := context.Background()
	req := &pb.CreateAnomalyGroupRequest{
		SubscriptionName:     "sub",
		SubscriptionRevision: "rev",
		Domain:               "domain-name",
		Benchmark:            "benchmark-name",
		StartCommit:          int64(100),
		EndCommit:            int64(200),
		Action:               pb.GroupActionType_REPORT,
	}
	store.On("Create", mock.Anything,
		"sub", "rev", "domain-name", "benchmark-name",
		int64(100), int64(200), "REPORT").Return("123", nil)

	_, err := service.CreateNewAnomalyGroup(ctx, req)

	// assert that the expectations were met
	store.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestLoadAnomalyGroupByID(t *testing.T) {
	service, store := setUp(t)
	ctx := context.Background()
	req := &pb.ReadAnomalyGroupRequest{
		AnomalyGroupId: "ce7107ae-3552-49e9-bd89-120ff97c3cea",
	}
	store.On("LoadById", mock.Anything,
		"ce7107ae-3552-49e9-bd89-120ff97c3cea").Return(nil, nil)

	_, err := service.LoadAnomalyGroupByID(ctx, req)

	// assert that the expectations were met
	store.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestUpdateGroup_BisectID(t *testing.T) {
	service, store := setUp(t)
	ctx := context.Background()
	req := &pb.UpdateAnomalyGroupRequest{
		AnomalyGroupId: "ce7107ae-3552-49e9-bd89-120ff97c3cea",
		BisectionId:    "3cb85993-d0a8-452e-86ec-cb5154aada9c",
	}
	store.On("UpdateBisectID", mock.Anything,
		"ce7107ae-3552-49e9-bd89-120ff97c3cea",
		"3cb85993-d0a8-452e-86ec-cb5154aada9c").Return(nil)

	_, err := service.UpdateAnomalyGroup(ctx, req)

	// assert that the expectations were met
	store.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestUpdateGroup_ReportedIssueID(t *testing.T) {
	service, store := setUp(t)
	ctx := context.Background()
	req := &pb.UpdateAnomalyGroupRequest{
		AnomalyGroupId: "ce7107ae-3552-49e9-bd89-120ff97c3cea",
		IssueId:        "24fa5591-946b-44e4-bf09-3fd271588ee5",
	}
	store.On("UpdateReportedIssueID", mock.Anything,
		"ce7107ae-3552-49e9-bd89-120ff97c3cea",
		"24fa5591-946b-44e4-bf09-3fd271588ee5").Return(nil)

	_, err := service.UpdateAnomalyGroup(ctx, req)

	// assert that the expectations were met
	store.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestUpdateGroup_AnomalyID(t *testing.T) {
	service, store := setUp(t)
	ctx := context.Background()
	req := &pb.UpdateAnomalyGroupRequest{
		AnomalyGroupId: "ce7107ae-3552-49e9-bd89-120ff97c3cea",
		AnomalyId:      "b1fb4036-1883-4d9e-85d4-ed607629017a",
	}
	store.On("AddAnomalyID", mock.Anything,
		"ce7107ae-3552-49e9-bd89-120ff97c3cea",
		"b1fb4036-1883-4d9e-85d4-ed607629017a").Return(nil)

	_, err := service.UpdateAnomalyGroup(ctx, req)

	// assert that the expectations were met
	store.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestUpdateGroup_CulpritIDs(t *testing.T) {
	service, store := setUp(t)
	ctx := context.Background()
	req := &pb.UpdateAnomalyGroupRequest{
		AnomalyGroupId: "ce7107ae-3552-49e9-bd89-120ff97c3cea",
		CulpritIds:     []string{"ffd48105-ce5a-425e-982a-fb4221c46f21"},
	}
	store.On("AddCulpritIDs", mock.Anything,
		"ce7107ae-3552-49e9-bd89-120ff97c3cea",
		[]string{"ffd48105-ce5a-425e-982a-fb4221c46f21"}).Return(nil)

	_, err := service.UpdateAnomalyGroup(ctx, req)

	// assert that the expectations were met
	store.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestFindExistingGroups(t *testing.T) {
	service, store := setUp(t)
	ctx := context.Background()
	req := &pb.FindExistingGroupsRequest{
		SubscriptionName:     "sub",
		SubscriptionRevision: "rev",
		Action:               pb.GroupActionType_BISECT,
		StartCommit:          int64(100),
		EndCommit:            int64(300),
		TestPath:             "domain-name/bot-x/benchmark-a/measurement-x/test-1",
	}
	store.On("FindExistingGroup", mock.Anything,
		"sub", "rev", "domain-name", "benchmark-a",
		int64(100), int64(300), "BISECT").Return(nil, nil)

	_, err := service.FindExistingGroups(ctx, req)

	// assert that the expectations were met
	store.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestFindExistingGroups_BadTestPath(t *testing.T) {
	service, _ := setUp(t)
	ctx := context.Background()
	req := &pb.FindExistingGroupsRequest{
		SubscriptionName:     "sub",
		SubscriptionRevision: "rev",
		Action:               pb.GroupActionType_BISECT,
		StartCommit:          int64(100),
		EndCommit:            int64(300),
		TestPath:             "domain-name/bot-x/benchmark-a/measurement-x",
	}

	_, err := service.FindExistingGroups(ctx, req)

	// assert that the expectations were met
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid fromat of test path")
}
