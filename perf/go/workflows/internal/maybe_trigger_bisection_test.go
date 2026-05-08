package internal

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	anomalygroup_proto "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	anomalygroup_mock "go.skia.org/infra/perf/go/anomalygroup/proto/v1/mocks"
	culprit_proto "go.skia.org/infra/perf/go/culprit/proto/v1"

	"go.skia.org/infra/perf/go/workflows"
	"go.skia.org/infra/pinpoint/go/pinpoint"

	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/grpc"
)

func setupAnomalyGroupService(
	t *testing.T,
) (string, *anomalygroup_mock.AnomalyGroupServiceServer, func()) {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	s := grpc.NewServer()
	service := anomalygroup_mock.NewAnomalyGroupServiceServer(t)
	anomalygroup_proto.RegisterAnomalyGroupServiceServer(s, service)
	go func() {
		require.NoError(t, s.Serve(lis))
	}()
	return lis.Addr().String(), service, func() {
		s.Stop()
	}
}

func TestMaybeTriggerBisection_GroupActionBisect_HappyPath(t *testing.T) {
	addr, server, cleanup := setupAnomalyGroupService(t)
	defer cleanup()
	c_addr, _, c_cleanup := setupCulpritService(t)
	defer c_cleanup()
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	agsa := &AnomalyGroupServiceActivity{insecure_conn: true}
	gsa := &GerritServiceActivity{insecure_conn: true}
	csa := &CulpritServiceActivity{insecure_conn: true}
	env.RegisterActivity(agsa)
	env.RegisterActivity(gsa)
	env.RegisterActivity(csa)

	anomalyGroupId := "group_id1"
	mockAnomalyIds := []string{"anomaly1"}
	var startCommit int64 = 1
	var endCommit int64 = 10
	mockAnomaly := &anomalygroup_proto.Anomaly{
		StartCommit: startCommit,
		EndCommit:   endCommit,
		Paramset: map[string]string{
			"bot":         "linux-perf",
			"benchmark":   "speedometer",
			"story":       "speedometer",
			"measurement": "runsperminute",
			"stat":        "sum",
		},
		ImprovementDirection: "UP",
	}
	server.On("LoadAnomalyGroupByID", mock.Anything, &anomalygroup_proto.LoadAnomalyGroupByIDRequest{
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&anomalygroup_proto.LoadAnomalyGroupByIDResponse{
				AnomalyGroup: &anomalygroup_proto.AnomalyGroup{
					GroupId:     anomalyGroupId,
					GroupAction: anomalygroup_proto.GroupActionType_BISECT,
					AnomalyIds:  mockAnomalyIds,
				},
			}, nil)
	server.On("FindTopAnomalies", mock.Anything, &anomalygroup_proto.FindTopAnomaliesRequest{
		AnomalyGroupId: anomalyGroupId,
		Limit:          1}).
		Return(
			&anomalygroup_proto.FindTopAnomaliesResponse{
				Anomalies: []*anomalygroup_proto.Anomaly{mockAnomaly},
			}, nil)
	mockStartRevision := "revision1"
	mockEndRevision := "revision10"
	env.OnActivity(agsa.CheckBisectionAllowed, mock.Anything).Return(true, nil).Once()
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, startCommit).
		Return(mockStartRevision, nil).
		Once()
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, endCommit).
		Return(mockEndRevision, nil).
		Once()

	env.OnActivity((&pinpoint.Client{}).CreateBisect, mock.Anything, mock.Anything, true).
		Return(&pinpoint.CreatePinpointResponse{JobID: "bisectionId"}, nil).
		Once()
	env.OnActivity((&pinpoint.Client{}).FetchJobState, mock.Anything, mock.Anything).
		Return(&pinpoint.FetchJobStateResponse{Status: "completed"}, nil).
		Once()
	server.On("UpdateAnomalyGroup", mock.Anything, mock.Anything).
		Return(
			&anomalygroup_proto.UpdateAnomalyGroupResponse{}, nil)
	env.ExecuteWorkflow(MaybeTriggerBisectionWorkflow, &workflows.MaybeTriggerBisectionParam{
		AnomalyGroupServiceUrl: addr,
		AnomalyGroupId:         anomalyGroupId,
		CulpritServiceUrl:      c_addr,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var resp *workflows.MaybeTriggerBisectionResult
	require.NoError(t, env.GetWorkflowResult(&resp))
	env.AssertExpectations(t)
}

func TestMaybeTriggerBisection_GroupActionBisect_ParseChartStat(t *testing.T) {
	addr, server, cleanup := setupAnomalyGroupService(t)
	defer cleanup()
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	agsa := &AnomalyGroupServiceActivity{insecure_conn: true}
	gsa := &GerritServiceActivity{insecure_conn: true}
	csa := &CulpritServiceActivity{insecure_conn: true}
	env.RegisterActivity(agsa)
	env.RegisterActivity(gsa)
	env.RegisterActivity(csa)

	anomalyGroupId := "group_id1"
	mockAnomalyIds := []string{"anomaly1"}
	var startCommit int64 = 1
	var endCommit int64 = 10
	mockAnomaly := &anomalygroup_proto.Anomaly{
		StartCommit: startCommit,
		EndCommit:   endCommit,
		Paramset: map[string]string{
			"bot":         "linux-perf",
			"benchmark":   "speedometer",
			"story":       "speedometer",
			"measurement": "runs_per_minute_max",
			"stat":        "error",
		},
		ImprovementDirection: "UP",
	}
	server.On("LoadAnomalyGroupByID", mock.Anything, &anomalygroup_proto.LoadAnomalyGroupByIDRequest{
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&anomalygroup_proto.LoadAnomalyGroupByIDResponse{
				AnomalyGroup: &anomalygroup_proto.AnomalyGroup{
					GroupId:     anomalyGroupId,
					GroupAction: anomalygroup_proto.GroupActionType_BISECT,
					AnomalyIds:  mockAnomalyIds,
				},
			}, nil)
	server.On("FindTopAnomalies", mock.Anything, &anomalygroup_proto.FindTopAnomaliesRequest{
		AnomalyGroupId: anomalyGroupId,
		Limit:          1}).
		Return(
			&anomalygroup_proto.FindTopAnomaliesResponse{
				Anomalies: []*anomalygroup_proto.Anomaly{mockAnomaly},
			}, nil)
	mockStartRevision := "revision1"
	mockEndRevision := "revision10"
	env.OnActivity(agsa.CheckBisectionAllowed, mock.Anything).Return(true, nil).Once()
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, startCommit).
		Return(mockStartRevision, nil).
		Once()
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, endCommit).
		Return(mockEndRevision, nil).
		Once()

	env.OnActivity((&pinpoint.Client{}).CreateBisect, mock.Anything, mock.Anything, true).
		Return(&pinpoint.CreatePinpointResponse{JobID: "bisectionId"}, nil).
		Once()
	env.OnActivity((&pinpoint.Client{}).FetchJobState, mock.Anything, mock.Anything).
		Return(&pinpoint.FetchJobStateResponse{Status: "completed"}, nil).
		Once()
	server.On("UpdateAnomalyGroup", mock.Anything, mock.Anything).
		Return(
			&anomalygroup_proto.UpdateAnomalyGroupResponse{}, nil)
	env.ExecuteWorkflow(MaybeTriggerBisectionWorkflow, &workflows.MaybeTriggerBisectionParam{
		AnomalyGroupServiceUrl: addr,
		AnomalyGroupId:         anomalyGroupId,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var resp *workflows.MaybeTriggerBisectionResult
	require.NoError(t, env.GetWorkflowResult(&resp))
	env.AssertExpectations(t)
}

func TestMaybeTriggerBisection_GroupActionReport_HappyPath(t *testing.T) {
	ag_addr, ag_server, ag_cleanup := setupAnomalyGroupService(t)
	defer ag_cleanup()
	c_addr, c_server, c_cleanup := setupCulpritService(t)
	defer c_cleanup()
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	agsa := &AnomalyGroupServiceActivity{insecure_conn: true}
	gsa := &GerritServiceActivity{insecure_conn: true}
	csa := &CulpritServiceActivity{insecure_conn: true}
	env.RegisterActivity(agsa)
	env.RegisterActivity(gsa)
	env.RegisterActivity(csa)

	anomalyGroupId := "group_id1"
	mockAnomalyIds := []string{"anomaly1"}
	ag_server.On("LoadAnomalyGroupByID", mock.Anything, &anomalygroup_proto.LoadAnomalyGroupByIDRequest{
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&anomalygroup_proto.LoadAnomalyGroupByIDResponse{
				AnomalyGroup: &anomalygroup_proto.AnomalyGroup{
					GroupId:     anomalyGroupId,
					GroupAction: anomalygroup_proto.GroupActionType_REPORT,
					AnomalyIds:  mockAnomalyIds,
				},
			}, nil)
	mockAnomalies := []*anomalygroup_proto.Anomaly{
		{
			StartCommit: int64(100),
			EndCommit:   int64(300),
			Paramset: map[string]string{
				"bot":         "linux-perf",
				"benchmark":   "speedometer",
				"story":       "speedometer",
				"measurement": "runsperminute",
				"stat":        "error",
			},
			ImprovementDirection: "UP",
		},
		{
			StartCommit: int64(130),
			EndCommit:   int64(500),
			Paramset: map[string]string{
				"bot":         "win-10-perf",
				"benchmark":   "speedometer2",
				"story":       "speedometer2",
				"measurement": "runsperminute",
				"stat":        "value",
			},
			ImprovementDirection: "UP",
		},
	}
	mockCulpritAnomalies := []*culprit_proto.Anomaly{
		{
			StartCommit: int64(100),
			EndCommit:   int64(300),
			Paramset: map[string]string{
				"bot":         "linux-perf",
				"benchmark":   "speedometer",
				"story":       "speedometer",
				"measurement": "runsperminute",
				"stat":        "error",
			},
			ImprovementDirection: "UP",
		},
		{
			StartCommit: int64(130),
			EndCommit:   int64(500),
			Paramset: map[string]string{
				"bot":         "win-10-perf",
				"benchmark":   "speedometer2",
				"story":       "speedometer2",
				"measurement": "runsperminute",
				"stat":        "value",
			},
			ImprovementDirection: "UP",
		},
	}
	ag_server.On("FindTopAnomalies", mock.Anything, &anomalygroup_proto.FindTopAnomaliesRequest{
		AnomalyGroupId: anomalyGroupId,
		Limit:          10}).
		Return(
			&anomalygroup_proto.FindTopAnomaliesResponse{Anomalies: mockAnomalies}, nil)
	mockIssueId := "mock_issue_id"
	c_server.On("NotifyUserOfAnomaly", mock.Anything, &culprit_proto.NotifyUserOfAnomalyRequest{
		AnomalyGroupId: anomalyGroupId,
		Anomaly:        mockCulpritAnomalies,
	}).Return(
		&culprit_proto.NotifyUserOfAnomalyResponse{IssueId: mockIssueId}, nil)

	ag_server.On("UpdateAnomalyGroup", mock.Anything, &anomalygroup_proto.UpdateAnomalyGroupRequest{
		AnomalyGroupId: anomalyGroupId,
		IssueId:        mockIssueId,
	}).Return(
		&anomalygroup_proto.UpdateAnomalyGroupResponse{}, nil)

	env.ExecuteWorkflow(MaybeTriggerBisectionWorkflow, &workflows.MaybeTriggerBisectionParam{
		AnomalyGroupServiceUrl: ag_addr,
		CulpritServiceUrl:      c_addr,
		AnomalyGroupId:         anomalyGroupId,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var resp *workflows.MaybeTriggerBisectionResult
	require.NoError(t, env.GetWorkflowResult(&resp))
	env.AssertExpectations(t)
}

func TestMaybeTriggerBisection_GroupActionBisect_BisectionNotAllowed(t *testing.T) {
	ag_addr, ag_server, ag_cleanup := setupAnomalyGroupService(t)
	defer ag_cleanup()
	c_addr, c_server, c_cleanup := setupCulpritService(t)
	defer c_cleanup()
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	agsa := &AnomalyGroupServiceActivity{insecure_conn: true}
	gsa := &GerritServiceActivity{insecure_conn: true}
	csa := &CulpritServiceActivity{insecure_conn: true}
	env.RegisterActivity(agsa)
	env.RegisterActivity(gsa)
	env.RegisterActivity(csa)

	anomalyGroupId := "group_id1"
	mockAnomalyIds := []string{"anomaly1"}
	ag_server.On("LoadAnomalyGroupByID", mock.Anything, &anomalygroup_proto.LoadAnomalyGroupByIDRequest{
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&anomalygroup_proto.LoadAnomalyGroupByIDResponse{
				AnomalyGroup: &anomalygroup_proto.AnomalyGroup{
					GroupId:     anomalyGroupId,
					GroupAction: anomalygroup_proto.GroupActionType_BISECT,
					AnomalyIds:  mockAnomalyIds,
				},
			}, nil)
	env.OnActivity(agsa.CheckBisectionAllowed, mock.Anything).Return(false, nil).Once()
	mockAnomalies := []*anomalygroup_proto.Anomaly{
		{
			StartCommit: int64(100),
			EndCommit:   int64(300),
			Paramset: map[string]string{
				"bot":         "linux-perf",
				"benchmark":   "speedometer",
				"story":       "speedometer",
				"measurement": "runsperminute",
				"stat":        "error",
			},
			ImprovementDirection: "UP",
		},
		{
			StartCommit: int64(130),
			EndCommit:   int64(500),
			Paramset: map[string]string{
				"bot":         "win-10-perf",
				"benchmark":   "speedometer2",
				"story":       "speedometer2",
				"measurement": "runsperminute",
				"stat":        "value",
			},
			ImprovementDirection: "UP",
		},
	}
	mockCulpritAnomalies := []*culprit_proto.Anomaly{
		{
			StartCommit: int64(100),
			EndCommit:   int64(300),
			Paramset: map[string]string{
				"bot":         "linux-perf",
				"benchmark":   "speedometer",
				"story":       "speedometer",
				"measurement": "runsperminute",
				"stat":        "error",
			},
			ImprovementDirection: "UP",
		},
		{
			StartCommit: int64(130),
			EndCommit:   int64(500),
			Paramset: map[string]string{
				"bot":         "win-10-perf",
				"benchmark":   "speedometer2",
				"story":       "speedometer2",
				"measurement": "runsperminute",
				"stat":        "value",
			},
			ImprovementDirection: "UP",
		},
	}
	ag_server.On("FindTopAnomalies", mock.Anything, &anomalygroup_proto.FindTopAnomaliesRequest{
		AnomalyGroupId: anomalyGroupId,
		Limit:          10}).
		Return(
			&anomalygroup_proto.FindTopAnomaliesResponse{Anomalies: mockAnomalies}, nil)
	mockIssueId := "mock_issue_id"
	c_server.On("NotifyUserOfAnomaly", mock.Anything, &culprit_proto.NotifyUserOfAnomalyRequest{
		AnomalyGroupId: anomalyGroupId,
		Anomaly:        mockCulpritAnomalies,
	}).Return(
		&culprit_proto.NotifyUserOfAnomalyResponse{IssueId: mockIssueId}, nil)

	ag_server.On("UpdateAnomalyGroup", mock.Anything, &anomalygroup_proto.UpdateAnomalyGroupRequest{
		AnomalyGroupId: anomalyGroupId,
		IssueId:        mockIssueId,
	}).Return(
		&anomalygroup_proto.UpdateAnomalyGroupResponse{}, nil)

	env.ExecuteWorkflow(MaybeTriggerBisectionWorkflow, &workflows.MaybeTriggerBisectionParam{
		AnomalyGroupServiceUrl: ag_addr,
		CulpritServiceUrl:      c_addr,
		AnomalyGroupId:         anomalyGroupId,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var resp *workflows.MaybeTriggerBisectionResult
	require.NoError(t, env.GetWorkflowResult(&resp))
	env.AssertExpectations(t)
}

func TestMaybeTriggerBisection_GroupActionBisect_HappyPath_StoryNameUpdate(t *testing.T) {
	addr, server, cleanup := setupAnomalyGroupService(t)
	defer cleanup()
	c_addr, _, c_cleanup := setupCulpritService(t)
	defer c_cleanup()
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	agsa := &AnomalyGroupServiceActivity{insecure_conn: true}
	gsa := &GerritServiceActivity{insecure_conn: true}
	csa := &CulpritServiceActivity{insecure_conn: true}
	env.RegisterActivity(agsa)
	env.RegisterActivity(gsa)
	env.RegisterActivity(csa)

	anomalyGroupId := "group_id1"
	mockAnomalyIds := []string{"anomaly1"}
	var startCommit int64 = 1
	var endCommit int64 = 10
	mockAnomaly := &anomalygroup_proto.Anomaly{
		StartCommit: startCommit,
		EndCommit:   endCommit,
		Paramset: map[string]string{
			"bot":         "linux-perf",
			"benchmark":   "system_health.common_desktop",
			"story":       "system_health_story_name",
			"measurement": "runsperminute",
			"stat":        "sum",
		},
		ImprovementDirection: "UP",
	}
	server.On("LoadAnomalyGroupByID", mock.Anything, &anomalygroup_proto.LoadAnomalyGroupByIDRequest{
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&anomalygroup_proto.LoadAnomalyGroupByIDResponse{
				AnomalyGroup: &anomalygroup_proto.AnomalyGroup{
					GroupId:     anomalyGroupId,
					GroupAction: anomalygroup_proto.GroupActionType_BISECT,
					AnomalyIds:  mockAnomalyIds,
				},
			}, nil)
	server.On("FindTopAnomalies", mock.Anything, &anomalygroup_proto.FindTopAnomaliesRequest{
		AnomalyGroupId: anomalyGroupId,
		Limit:          1}).
		Return(
			&anomalygroup_proto.FindTopAnomaliesResponse{
				Anomalies: []*anomalygroup_proto.Anomaly{mockAnomaly},
			}, nil)
	mockStartRevision := "revision1"
	mockEndRevision := "revision10"
	env.OnActivity(agsa.CheckBisectionAllowed, mock.Anything).Return(true, nil).Once()
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, startCommit).
		Return(mockStartRevision, nil).
		Once()
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, endCommit).
		Return(mockEndRevision, nil).
		Once()

	env.OnActivity((&pinpoint.Client{}).CreateBisect, mock.Anything, mock.Anything, true).
		Return(&pinpoint.CreatePinpointResponse{JobID: "bisectionId"}, nil).
		Once()
	env.OnActivity((&pinpoint.Client{}).FetchJobState, mock.Anything, mock.Anything).
		Return(&pinpoint.FetchJobStateResponse{Status: "completed"}, nil).
		Once()
	server.On("UpdateAnomalyGroup", mock.Anything, mock.Anything).
		Return(
			&anomalygroup_proto.UpdateAnomalyGroupResponse{}, nil)
	env.ExecuteWorkflow(MaybeTriggerBisectionWorkflow, &workflows.MaybeTriggerBisectionParam{
		AnomalyGroupServiceUrl: addr,
		AnomalyGroupId:         anomalyGroupId,
		CulpritServiceUrl:      c_addr,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var resp *workflows.MaybeTriggerBisectionResult
	require.NoError(t, env.GetWorkflowResult(&resp))
	env.AssertExpectations(t)
}

func TestCreateLegacyBisectJob(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	agsa := &AnomalyGroupServiceActivity{insecure_conn: true}
	env.RegisterActivity(agsa)

	mockAnomaly := &anomalygroup_proto.Anomaly{
		StartCommit: 1,
		EndCommit:   10,
		Paramset: map[string]string{
			"bot":         "linux-perf",
			"benchmark":   "speedometer",
			"story":       "speedometer",
			"measurement": "runsperminute",
			"stat":        "sum",
			"test_path":   "ChromiumPerf/linux-perf/speedometer/runsperminute/speedometer",
		},
		ImprovementDirection: "UP",
	}
	mockStartRevision := "revision1"
	mockEndRevision := "revision10"

	expectedReq := &pinpoint.BisectJobCreateRequest{
		ComparisonMode: "performance",
		StartGitHash:   mockStartRevision,
		EndGitHash:     mockEndRevision,
		Configuration:  mockAnomaly.Paramset["bot"],
		Benchmark:      mockAnomaly.Paramset["benchmark"],
		Story:          mockAnomaly.Paramset["story"],
		Chart:          mockAnomaly.Paramset["measurement"],
		Statistic:      "",
		TestPath:       "ChromiumPerf/linux-perf/speedometer/runsperminute/speedometer",
	}

	env.OnActivity((&pinpoint.Client{}).CreateBisect, mock.Anything, expectedReq, true).
		Return(&pinpoint.CreatePinpointResponse{JobID: "legacyBisectionId"}, nil).
		Once()

	var actualJobId string
	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
		jobId, err := createBisectJob(
			ctx,
			mockAnomaly,
			mockStartRevision,
			mockEndRevision,
		)
		actualJobId = jobId
		return err
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, "legacyBisectionId", actualJobId)
	env.AssertExpectations(t)
}

func TestCreateLegacyBisectJob_EmptyJobID(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	agsa := &AnomalyGroupServiceActivity{insecure_conn: true}
	env.RegisterActivity(agsa)

	mockAnomaly := &anomalygroup_proto.Anomaly{
		StartCommit: 1,
		EndCommit:   10,
		Paramset: map[string]string{
			"bot":         "linux-perf",
			"benchmark":   "speedometer",
			"story":       "speedometer",
			"measurement": "runsperminute",
			"stat":        "sum",
			"test_path":   "ChromiumPerf/linux-perf/speedometer/runsperminute/speedometer",
		},
		ImprovementDirection: "UP",
	}
	mockStartRevision := "revision1"
	mockEndRevision := "revision10"

	expectedReq := &pinpoint.BisectJobCreateRequest{
		ComparisonMode: "performance",
		StartGitHash:   mockStartRevision,
		EndGitHash:     mockEndRevision,
		Configuration:  mockAnomaly.Paramset["bot"],
		Benchmark:      mockAnomaly.Paramset["benchmark"],
		Story:          mockAnomaly.Paramset["story"],
		Chart:          mockAnomaly.Paramset["measurement"],
		Statistic:      "",
		TestPath:       "ChromiumPerf/linux-perf/speedometer/runsperminute/speedometer",
	}

	env.OnActivity((&pinpoint.Client{}).CreateBisect, mock.Anything, expectedReq, true).
		Return(&pinpoint.CreatePinpointResponse{JobID: ""}, nil).
		Once()

	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
		_, err := createBisectJob(
			ctx,
			mockAnomaly,
			mockStartRevision,
			mockEndRevision,
		)
		return err
	})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	require.Contains(t, env.GetWorkflowError().Error(), "Chromeperf failed to create a new job")
	env.AssertExpectations(t)
}

func TestWaitPinpointJobCompletion_Completed(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	jobId := "test-job-id"

	env.OnActivity(
		(&pinpoint.Client{}).FetchJobState,
		mock.Anything,
		pinpoint.FetchJobStateRequest{JobID: jobId},
	).Return(&pinpoint.FetchJobStateResponse{Status: "running"}, nil).
		Once()
	env.OnActivity(
		(&pinpoint.Client{}).FetchJobState,
		mock.Anything,
		pinpoint.FetchJobStateRequest{JobID: jobId},
	).Return(&pinpoint.FetchJobStateResponse{Status: "completed"}, nil).
		Once()

	var actualResp *pinpoint.FetchJobStateResponse
	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
		resp, err := waitPinpointJobCompletion(ctx, jobId)
		actualResp = resp
		return err
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.NotNil(t, actualResp)
	require.Equal(t, "completed", actualResp.Status)
	env.AssertExpectations(t)
}

func TestWaitPinpointJobCompletion_Failed(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	jobId := "test-job-id"

	env.OnActivity(
		(&pinpoint.Client{}).FetchJobState,
		mock.Anything,
		pinpoint.FetchJobStateRequest{JobID: jobId},
	).Return(&pinpoint.FetchJobStateResponse{Status: "failed"}, nil).
		Once()

	var actualResp *pinpoint.FetchJobStateResponse
	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
		resp, err := waitPinpointJobCompletion(ctx, jobId)
		actualResp = resp
		return err
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.NotNil(t, actualResp)
	require.Equal(t, "failed", actualResp.Status)
	env.AssertExpectations(t)
}

func TestWaitPinpointJobCompletion_Cancelled(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	jobId := "test-job-id"

	env.OnActivity(
		(&pinpoint.Client{}).FetchJobState,
		mock.Anything,
		pinpoint.FetchJobStateRequest{JobID: jobId},
	).Return(&pinpoint.FetchJobStateResponse{Status: "cancelled"}, nil).
		Once()

	var actualResp *pinpoint.FetchJobStateResponse
	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
		resp, err := waitPinpointJobCompletion(ctx, jobId)
		actualResp = resp
		return err
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.NotNil(t, actualResp)
	require.Equal(t, "cancelled", actualResp.Status)
	env.AssertExpectations(t)
}

func TestWaitPinpointJobCompletion_Timeout(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	jobId := "test-job-id"

	// Timeout is 10 hours, interval is 30 minutes.
	// It will poll 21 times before timing out.
	env.OnActivity(
		(&pinpoint.Client{}).FetchJobState,
		mock.Anything,
		pinpoint.FetchJobStateRequest{JobID: jobId},
	).Return(&pinpoint.FetchJobStateResponse{Status: "running"}, nil).
		Times(21)

	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
		_, err := waitPinpointJobCompletion(ctx, jobId)
		return err
	})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	require.Contains(t, env.GetWorkflowError().Error(), "Pinpoint job timeout")
	env.AssertExpectations(t)
}

func TestExtractCulprits(t *testing.T) {
	testCases := []struct {
		name     string
		jobState string
		expected []string
	}{
		{
			name:     "EmptyResponse",
			jobState: `{}`,
			expected: nil,
		},
		{
			name: "NoDifferentComparison",
			jobState: `{
				"state": [
					{
						"comparisons": {"prev": "same"},
						"change": {
							"commits": [
								{"repository": "chromium", "git_hash": "commit1"}
							]
						}
					}
				]
			}`,
			expected: nil,
		},
		{
			name: "WithDifferentComparison_SingleCommit",
			jobState: `{
				"state": [
					{
						"comparisons": {"next": "different"},
						"change": {
							"commits": [
								{"repository": "chromium", "git_hash": "commit1"}
							]
						}
					},
					{
						"comparisons": {"prev": "different"},
						"change": {
							"commits": [
								{"repository": "chromium", "git_hash": "commit2"}
							]
						}
					}
				]
			}`,
			expected: []string{"commit2"},
		},
		{
			name: "WithDifferentComparison_MultipleCommits_MultipleRepos",
			jobState: `{
				"state": [
					{
						"comparisons": {"next": "different"},
						"change": {
							"commits": [
								{"repository": "chromium", "git_hash": "commit0"},
								{"repository": "v8", "git_hash": "commit00"}
							]
						}
					},
					{
						"comparisons": {"prev": "different", "next": "different"},
						"change": {
							"commits": [
								{"repository": "chromium", "git_hash": "commit1"},
								{"repository": "v8", "git_hash": "commit11"}
							]
						}
					},
					{
						"comparisons": {"prev": "different"},
						"change": {
							"commits": [
								{"repository": "chromium", "git_hash": "commit2"},
								{"repository": "v8", "git_hash": "commit22"}
							]
						}
					}
				]
			}`,
			expected: []string{"commit1", "commit2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var response pinpoint.FetchJobStateResponse
			err := json.Unmarshal([]byte(tc.jobState), &response)
			require.NoError(t, err)
			culprits, err := extractCulprits(&response)
			require.NoError(t, err)
			require.Equal(t, tc.expected, culprits)
		})
	}
}
