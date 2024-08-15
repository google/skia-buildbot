package internal

import (
	"net"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	ag_pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	ag_mock "go.skia.org/infra/perf/go/anomalygroup/proto/v1/mocks"
	c_pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/workflows"
	pinpoint "go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/catapult"
	pp_pb "go.skia.org/infra/pinpoint/proto/v1"

	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/grpc"
)

func setupAnomalyGroupService(t *testing.T) (string, *ag_mock.AnomalyGroupServiceServer, func()) {
	lis, err := net.Listen("tcp", "localhost:9001")
	require.NoError(t, err)
	s := grpc.NewServer()
	service := ag_mock.NewAnomalyGroupServiceServer(t)
	ag_pb.RegisterAnomalyGroupServiceServer(s, service)
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
	env.RegisterWorkflowWithOptions(catapult.CulpritFinderWorkflow, workflow.RegisterOptions{Name: pinpoint.CulpritFinderWorkflow})

	anomalyGroupId := "group_id1"
	mockAnomalyIds := []string{"anomaly1"}
	var startCommit int64 = 1
	var endCommit int64 = 10
	mockAnomaly := &ag_pb.Anomaly{
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
	server.On("LoadAnomalyGroupByID", mock.Anything, &ag_pb.LoadAnomalyGroupByIDRequest{
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&ag_pb.LoadAnomalyGroupByIDResponse{
				AnomalyGroup: &ag_pb.AnomalyGroup{
					GroupId:     anomalyGroupId,
					GroupAction: ag_pb.GroupActionType_BISECT,
					AnomalyIds:  mockAnomalyIds,
				},
			}, nil)
	server.On("FindTopAnomalies", mock.Anything, &ag_pb.FindTopAnomaliesRequest{
		AnomalyGroupId: anomalyGroupId,
		Limit:          1}).
		Return(
			&ag_pb.FindTopAnomaliesResponse{Anomalies: []*ag_pb.Anomaly{mockAnomaly}}, nil)
	mockStartRevision := "revision1"
	mockEndRevision := "revision10"
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, startCommit).Return(mockStartRevision, nil).Once()
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, endCommit).Return(mockEndRevision, nil).Once()

	env.OnWorkflow(pinpoint.CulpritFinderWorkflow,
		mock.Anything,
		&pinpoint.CulpritFinderParams{
			Request: &pp_pb.ScheduleCulpritFinderRequest{
				StartGitHash:         mockStartRevision,
				EndGitHash:           mockEndRevision,
				Configuration:        mockAnomaly.Paramset["bot"],
				Benchmark:            mockAnomaly.Paramset["benchmark"],
				Story:                mockAnomaly.Paramset["story"],
				Chart:                mockAnomaly.Paramset["measurement"],
				AggregationMethod:    "",
				ImprovementDirection: mockAnomaly.ImprovementDirection,
			},
			CallbackParams: &pp_pb.CulpritProcessingCallbackParams{
				AnomalyGroupId:    anomalyGroupId,
				CulpritServiceUrl: c_addr,
			},
		}).Return(&pp_pb.CulpritFinderExecution{
		JobId: "bisectionId",
	}, nil).Once()
	server.On("UpdateAnomalyGroup", mock.Anything, mock.Anything).
		Return(
			&ag_pb.UpdateAnomalyGroupResponse{}, nil)
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

func TestMaybeTriggerBisection_GroupActionBisect_ParseChartAggregation(t *testing.T) {
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
	env.RegisterWorkflowWithOptions(catapult.CulpritFinderWorkflow, workflow.RegisterOptions{Name: pinpoint.CulpritFinderWorkflow})

	anomalyGroupId := "group_id1"
	mockAnomalyIds := []string{"anomaly1"}
	var startCommit int64 = 1
	var endCommit int64 = 10
	mockAnomaly := &ag_pb.Anomaly{
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
	server.On("LoadAnomalyGroupByID", mock.Anything, &ag_pb.LoadAnomalyGroupByIDRequest{
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&ag_pb.LoadAnomalyGroupByIDResponse{
				AnomalyGroup: &ag_pb.AnomalyGroup{
					GroupId:     anomalyGroupId,
					GroupAction: ag_pb.GroupActionType_BISECT,
					AnomalyIds:  mockAnomalyIds,
				},
			}, nil)
	server.On("FindTopAnomalies", mock.Anything, &ag_pb.FindTopAnomaliesRequest{
		AnomalyGroupId: anomalyGroupId,
		Limit:          1}).
		Return(
			&ag_pb.FindTopAnomaliesResponse{Anomalies: []*ag_pb.Anomaly{mockAnomaly}}, nil)
	mockStartRevision := "revision1"
	mockEndRevision := "revision10"
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, startCommit).Return(mockStartRevision, nil).Once()
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, endCommit).Return(mockEndRevision, nil).Once()

	env.OnWorkflow(pinpoint.CulpritFinderWorkflow,
		mock.Anything,
		&pinpoint.CulpritFinderParams{
			Request: &pp_pb.ScheduleCulpritFinderRequest{
				StartGitHash:         mockStartRevision,
				EndGitHash:           mockEndRevision,
				Configuration:        mockAnomaly.Paramset["bot"],
				Benchmark:            mockAnomaly.Paramset["benchmark"],
				Story:                mockAnomaly.Paramset["story"],
				Chart:                "runs_per_minute",
				AggregationMethod:    "max",
				ImprovementDirection: mockAnomaly.ImprovementDirection,
			},
		}).Return(&pp_pb.CulpritFinderExecution{
		JobId: "bisectionId",
	}, nil).Once()
	server.On("UpdateAnomalyGroup", mock.Anything, mock.Anything).
		Return(
			&ag_pb.UpdateAnomalyGroupResponse{}, nil)
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
	env.RegisterWorkflowWithOptions(catapult.CulpritFinderWorkflow, workflow.RegisterOptions{Name: pinpoint.CulpritFinderWorkflow})

	anomalyGroupId := "group_id1"
	mockAnomalyIds := []string{"anomaly1"}
	ag_server.On("LoadAnomalyGroupByID", mock.Anything, &ag_pb.LoadAnomalyGroupByIDRequest{
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&ag_pb.LoadAnomalyGroupByIDResponse{
				AnomalyGroup: &ag_pb.AnomalyGroup{
					GroupId:     anomalyGroupId,
					GroupAction: ag_pb.GroupActionType_REPORT,
					AnomalyIds:  mockAnomalyIds,
				},
			}, nil)
	mockAnomalies := []*ag_pb.Anomaly{
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
	mockCulpritAnomalies := []*c_pb.Anomaly{
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
	ag_server.On("FindTopAnomalies", mock.Anything, &ag_pb.FindTopAnomaliesRequest{
		AnomalyGroupId: anomalyGroupId,
		Limit:          10}).
		Return(
			&ag_pb.FindTopAnomaliesResponse{Anomalies: mockAnomalies}, nil)
	c_server.On("NotifyUserOfAnomaly", mock.Anything, &c_pb.NotifyUserOfAnomalyRequest{
		AnomalyGroupId: anomalyGroupId,
		Anomaly:        mockCulpritAnomalies,
	}).Return(
		&c_pb.NotifyUserOfAnomalyResponse{}, nil)

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
	env.RegisterWorkflowWithOptions(catapult.CulpritFinderWorkflow, workflow.RegisterOptions{Name: pinpoint.CulpritFinderWorkflow})

	anomalyGroupId := "group_id1"
	mockAnomalyIds := []string{"anomaly1"}
	var startCommit int64 = 1
	var endCommit int64 = 10
	mockAnomaly := &ag_pb.Anomaly{
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
	server.On("LoadAnomalyGroupByID", mock.Anything, &ag_pb.LoadAnomalyGroupByIDRequest{
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&ag_pb.LoadAnomalyGroupByIDResponse{
				AnomalyGroup: &ag_pb.AnomalyGroup{
					GroupId:     anomalyGroupId,
					GroupAction: ag_pb.GroupActionType_BISECT,
					AnomalyIds:  mockAnomalyIds,
				},
			}, nil)
	server.On("FindTopAnomalies", mock.Anything, &ag_pb.FindTopAnomaliesRequest{
		AnomalyGroupId: anomalyGroupId,
		Limit:          1}).
		Return(
			&ag_pb.FindTopAnomaliesResponse{Anomalies: []*ag_pb.Anomaly{mockAnomaly}}, nil)
	mockStartRevision := "revision1"
	mockEndRevision := "revision10"
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, startCommit).Return(mockStartRevision, nil).Once()
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, endCommit).Return(mockEndRevision, nil).Once()

	env.OnWorkflow(pinpoint.CulpritFinderWorkflow,
		mock.Anything,
		&pinpoint.CulpritFinderParams{
			Request: &pp_pb.ScheduleCulpritFinderRequest{
				StartGitHash:         mockStartRevision,
				EndGitHash:           mockEndRevision,
				Configuration:        mockAnomaly.Paramset["bot"],
				Benchmark:            mockAnomaly.Paramset["benchmark"],
				Story:                "system:health:story:name",
				Chart:                mockAnomaly.Paramset["measurement"],
				AggregationMethod:    "",
				ImprovementDirection: mockAnomaly.ImprovementDirection,
			},
			CallbackParams: &pp_pb.CulpritProcessingCallbackParams{
				AnomalyGroupId:    anomalyGroupId,
				CulpritServiceUrl: c_addr,
			},
		}).Return(&pp_pb.CulpritFinderExecution{
		JobId: "bisectionId",
	}, nil).Once()
	server.On("UpdateAnomalyGroup", mock.Anything, mock.Anything).
		Return(
			&ag_pb.UpdateAnomalyGroupResponse{}, nil)
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
