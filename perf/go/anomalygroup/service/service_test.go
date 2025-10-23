package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	temporal_mocks "go.temporal.io/sdk/mocks"

	mocks "go.skia.org/infra/perf/go/anomalygroup/mocks"
	ag "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	c_mock "go.skia.org/infra/perf/go/culprit/mocks"
	"go.skia.org/infra/perf/go/dataframe"
	reg "go.skia.org/infra/perf/go/regression"
	reg_mocks "go.skia.org/infra/perf/go/regression/mocks"
	"go.skia.org/infra/perf/go/ui/frame"
)

func setUp(_ *testing.T) (*anomalygroupService, *mocks.Store, *reg_mocks.Store) {
	mockAnomalyGroupstore := new(mocks.Store)
	mockCulpritstore := new(c_mock.Store)
	mockRegressionStore := new(reg_mocks.Store)
	mockTemporalClient := temporal_mocks.Client{}
	service := New(mockAnomalyGroupstore, mockCulpritstore, mockRegressionStore, &mockTemporalClient)
	return service, mockAnomalyGroupstore, mockRegressionStore
}

func TestCreateNewAnomalyGroup(t *testing.T) {
	service, store, _ := setUp(t)
	ctx := context.Background()
	req := &ag.CreateNewAnomalyGroupRequest{
		SubscriptionName:     "sub",
		SubscriptionRevision: "rev",
		Domain:               "domain-name",
		Benchmark:            "benchmark-name",
		StartCommit:          int64(100),
		EndCommit:            int64(200),
		Action:               ag.GroupActionType_REPORT,
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
	service, store, _ := setUp(t)
	ctx := context.Background()
	req := &ag.LoadAnomalyGroupByIDRequest{
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
	service, store, _ := setUp(t)
	ctx := context.Background()
	req := &ag.UpdateAnomalyGroupRequest{
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
	service, store, _ := setUp(t)
	ctx := context.Background()
	req := &ag.UpdateAnomalyGroupRequest{
		AnomalyGroupId: "ce7107ae-3552-49e9-bd89-120ff97c3cea",
		IssueId:        "12345",
	}
	store.On("UpdateReportedIssueID", mock.Anything,
		"ce7107ae-3552-49e9-bd89-120ff97c3cea",
		"12345").Return(nil)

	_, err := service.UpdateAnomalyGroup(ctx, req)

	// assert that the expectations were met
	store.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestUpdateGroup_AnomalyID(t *testing.T) {
	service, store, reg_store := setUp(t)
	ctx := context.Background()
	req := &ag.UpdateAnomalyGroupRequest{
		AnomalyGroupId: "ce7107ae-3552-49e9-bd-89-120ff97c3cea",
		AnomalyId:      "b1fb4036-1883-4d9e-85d4-ed607629017a",
	}
	reg_store.On("GetByIDs", mock.Anything, []string{req.AnomalyId}).Return(
		[]*reg.Regression{
			{
				CommitNumber:     200,
				PrevCommitNumber: 100,
			},
		}, nil)
	store.On("AddAnomalyID", mock.Anything,
		"ce7107ae-3552-49e9-bd-89-120ff97c3cea",
		"b1fb4036-1883-4d9e-85d4-ed607629017a",
		int64(101),
		int64(200)).Return(nil)

	_, err := service.UpdateAnomalyGroup(ctx, req)

	// assert that the expectations were met
	store.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestUpdateGroup_CulpritIDs(t *testing.T) {
	service, store, _ := setUp(t)
	ctx := context.Background()
	req := &ag.UpdateAnomalyGroupRequest{
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
	service, store, _ := setUp(t)
	ctx := context.Background()
	req := &ag.FindExistingGroupsRequest{
		SubscriptionName:     "sub",
		SubscriptionRevision: "rev",
		Action:               ag.GroupActionType_BISECT,
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
	service, _, _ := setUp(t)
	ctx := context.Background()
	req := &ag.FindExistingGroupsRequest{
		SubscriptionName:     "sub",
		SubscriptionRevision: "rev",
		Action:               ag.GroupActionType_BISECT,
		StartCommit:          int64(100),
		EndCommit:            int64(300),
		TestPath:             "domain-name/bot-x/benchmark-a/measurement-x",
	}

	_, err := service.FindExistingGroups(ctx, req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid fromat of test path")
}

func TestFindExistingGroups_anomalygroupFail(t *testing.T) {
	service, ag_store, _ := setUp(t)
	ctx := context.Background()
	req := &ag.FindExistingGroupsRequest{
		SubscriptionName:     "sub",
		SubscriptionRevision: "rev",
		Action:               ag.GroupActionType_BISECT,
		StartCommit:          int64(100),
		EndCommit:            int64(300),
		TestPath:             "domain-name/bot-x/benchmark-a/measurement-x/test-1	",
	}

	ag_store.On("FindExistingGroup", ctx, "sub", "rev", "domain-name",
		"benchmark-a", int64(100), int64(300), "BISECT").Return(nil, errors.New("fail"))

	_, err := service.FindExistingGroups(ctx, req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed on finding existing groups")
}

func TestFindTopAnomalies_TopOneOfTwo(t *testing.T) {
	group_id := "ce7107ae-3552-49e9-bd89-120ff97c3cea"
	anomaly_ids := []string{"b1fb4036-1883-4d9e-85d4-ed607629017a"}
	reg_ids := []string{"982ba5b6-430a-4e64-9b64-3f6f990dacf0", "982ba5b6-430a-4e64-9b64-3f6f990dacf1"}
	service, ag_store, reg_store := setUp(t)
	ctx := context.Background()
	req := &ag.FindTopAnomaliesRequest{
		AnomalyGroupId: group_id,
		Limit:          1,
	}

	ag_store.On("LoadById", mock.Anything, group_id).Return(
		&ag.AnomalyGroup{
			AnomalyIds: anomaly_ids,
		}, nil)
	reg_store.On("GetByIDs", mock.Anything, anomaly_ids).Return(
		[]*reg.Regression{
			{
				Id:           reg_ids[0],
				MedianBefore: 10,
				MedianAfter:  20,
				Frame: &frame.FrameResponse{
					DataFrame: &dataframe.DataFrame{
						ParamSet: map[string][]string{
							"bot":                   {"bot"},
							"benchmark":             {"bm"},
							"test":                  {"t"},
							"stat":                  {"st"},
							"improvement_direction": {"UP"},
							"subtest_1":             {"sub1"},
							"subtest_2":             {"sub1_sub2"},
							"subtest_3":             {"sub1_sub2_sub3"},
						},
					},
				},
			},
			{
				Id:           reg_ids[1],
				MedianBefore: 10,
				MedianAfter:  25,
				Frame: &frame.FrameResponse{
					DataFrame: &dataframe.DataFrame{
						ParamSet: map[string][]string{
							"bot":                   {"bot"},
							"benchmark":             {"bm"},
							"test":                  {"t"},
							"stat":                  {"st"},
							"improvement_direction": {"UP"},
							"subtest_1":             {"sub11"},
							"subtest_2":             {"sub11_sub22"},
							"subtest_3":             {"sub11_sub22_sub33"},
						},
					},
				},
			},
		}, nil)
	resp, err := service.FindTopAnomalies(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(resp.Anomalies))
	assert.Equal(t, "sub11_sub22_sub33", resp.Anomalies[0].Paramset["story"])
}

func TestFindTopAnomalies_NoSubTest3(t *testing.T) {
	group_id := "ce7107ae-3552-49e9-bd89-120ff97c3cea"
	anomaly_ids := []string{"b1fb4036-1883-4d9e-85d4-ed607629017a"}
	reg_ids := []string{"982ba5b6-430a-4e64-9b64-3f6f990dacf0"}
	service, ag_store, reg_store := setUp(t)
	ctx := context.Background()
	req := &ag.FindTopAnomaliesRequest{
		AnomalyGroupId: group_id,
		Limit:          1,
	}

	ag_store.On("LoadById", mock.Anything, group_id).Return(
		&ag.AnomalyGroup{
			AnomalyIds: anomaly_ids,
		}, nil)
	reg_store.On("GetByIDs", mock.Anything, anomaly_ids).Return(
		[]*reg.Regression{
			{
				Id:           reg_ids[0],
				MedianBefore: 10,
				MedianAfter:  20,
				Frame: &frame.FrameResponse{
					DataFrame: &dataframe.DataFrame{
						ParamSet: map[string][]string{
							"bot":                   {"bot"},
							"benchmark":             {"bm"},
							"test":                  {"t"},
							"stat":                  {"st"},
							"improvement_direction": {"UP"},
							"subtest_1":             {"sub1"},
							"subtest_2":             {"sub1_sub2"},
						},
					},
				},
			},
		}, nil)
	resp, err := service.FindTopAnomalies(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(resp.Anomalies))
	assert.Equal(t, "sub1_sub2", resp.Anomalies[0].Paramset["story"])
}

func TestFindTopAnomalies_GetAllSorted(t *testing.T) {
	group_id := "ce7107ae-3552-49e9-bd89-120ff97c3cea"
	anomaly_ids := []string{"b1fb4036-1883-4d9e-85d4-ed607629017a"}
	reg_ids := []string{
		"982ba5b6-430a-4e64-9b64-3f6f990dacf0",
		"982ba5b6-430a-4e64-9b64-3f6f990dacf1",
		"982ba5b6-430a-4e64-9b64-3f6f990dacf2"}
	service, ag_store, reg_store := setUp(t)
	ctx := context.Background()
	req := &ag.FindTopAnomaliesRequest{
		AnomalyGroupId: group_id,
		Limit:          0,
	}

	ag_store.On("LoadById", mock.Anything, group_id).Return(
		&ag.AnomalyGroup{
			AnomalyIds: anomaly_ids,
		}, nil)
	reg_store.On("GetByIDs", mock.Anything, anomaly_ids).Return(
		[]*reg.Regression{
			{
				Id:           reg_ids[0],
				MedianBefore: 10,
				MedianAfter:  20,
				Frame: &frame.FrameResponse{
					DataFrame: &dataframe.DataFrame{
						ParamSet: map[string][]string{
							"bot":                   {"bot"},
							"benchmark":             {"bm"},
							"test":                  {"t"},
							"stat":                  {"st"},
							"improvement_direction": {"UP"},
							"subtest_1":             {"sub1"},
							"subtest_2":             {"sub1_sub2"},
							"subtest_3":             {"sub1_sub2_sub3"},
						},
					},
				},
			},
			{
				Id:           reg_ids[1],
				MedianBefore: 10,
				MedianAfter:  25,
				Frame: &frame.FrameResponse{
					DataFrame: &dataframe.DataFrame{
						ParamSet: map[string][]string{
							"bot":                   {"bot"},
							"benchmark":             {"bm"},
							"test":                  {"t"},
							"stat":                  {"st"},
							"improvement_direction": {"UP"},
							"subtest_1":             {"sub11"},
							"subtest_2":             {"sub11_sub22"},
							"subtest_3":             {"sub11_sub22_sub33"},
						},
					},
				},
			},
			{
				Id:           reg_ids[2],
				MedianBefore: 10,
				MedianAfter:  22,
				Frame: &frame.FrameResponse{
					DataFrame: &dataframe.DataFrame{
						ParamSet: map[string][]string{
							"bot":                   {"bot"},
							"benchmark":             {"bm"},
							"test":                  {"t"},
							"stat":                  {"st"},
							"improvement_direction": {"UP"},
							"subtest_1":             {"sub111"},
							"subtest_2":             {"sub111_sub222"},
						},
					},
				},
			},
		}, nil)
	resp, err := service.FindTopAnomalies(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(resp.Anomalies))
	assert.Equal(t, "sub11_sub22_sub33", resp.Anomalies[0].Paramset["story"])
	assert.Equal(t, "sub111_sub222", resp.Anomalies[1].Paramset["story"])
	assert.Equal(t, "sub1_sub2_sub3", resp.Anomalies[2].Paramset["story"])
}

func TestFindTopAnomalies_anomalygroupFail(t *testing.T) {
	group_id := "ce7107ae-3552-49e9-bd89-120ff97c3cea"
	service, ag_store, _ := setUp(t)
	ctx := context.Background()
	req := &ag.FindTopAnomaliesRequest{
		AnomalyGroupId: group_id,
		Limit:          1,
	}

	ag_store.On("LoadById", mock.Anything, group_id).Return(
		nil, errors.New("cannot loadbyid"))
	_, err := service.FindTopAnomalies(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load anomaly group")
}

func TestFindTopAnomalies_regressionFail(t *testing.T) {
	group_id := "ce7107ae-3552-49e9-bd89-120ff97c3cea"
	anomaly_ids := []string{"b1fb4036-1883-4d9e-85d4-ed607629017a"}
	service, ag_store, reg_store := setUp(t)
	ctx := context.Background()
	req := &ag.FindTopAnomaliesRequest{
		AnomalyGroupId: group_id,
		Limit:          1,
	}

	ag_store.On("LoadById", mock.Anything, group_id).Return(
		&ag.AnomalyGroup{
			AnomalyIds: anomaly_ids,
		}, nil)
	reg_store.On("GetByIDs", mock.Anything, anomaly_ids).Return(
		nil, errors.New("cannot getbyids"))
	_, err := service.FindTopAnomalies(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load regressions from group")
}

func TestFindTopAnomalies_invalidParamset(t *testing.T) {
	group_id := "ce7107ae-3552-49e9-bd89-120ff97c3cea"
	anomaly_ids := []string{"b1fb4036-1883-4d9e-85d4-ed607629017a"}
	reg_ids := []string{"982ba5b6-430a-4e64-9b64-3f6f990dacf0"}
	service, ag_store, reg_store := setUp(t)
	ctx := context.Background()
	req := &ag.FindTopAnomaliesRequest{
		AnomalyGroupId: group_id,
		Limit:          1,
	}

	ag_store.On("LoadById", mock.Anything, group_id).Return(
		&ag.AnomalyGroup{
			AnomalyIds: anomaly_ids,
		}, nil)
	reg_store.On("GetByIDs", mock.Anything, anomaly_ids).Return(
		[]*reg.Regression{
			{
				Id:           reg_ids[0],
				MedianBefore: 10,
				MedianAfter:  20,
				Frame: &frame.FrameResponse{
					DataFrame: &dataframe.DataFrame{
						ParamSet: map[string][]string{
							// no 'stat'
							"bot":                   {"bot"},
							"benchmark":             {"bm"},
							"test":                  {"t"},
							"improvement_direction": {"UP"},
							"subtest_1":             {"sub1"},
							"subtest_2":             {"sub1_sub2"},
						},
					},
				},
			},
		}, nil)
	_, err := service.FindTopAnomalies(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid paramset")
}
