package internal

import (
	"net"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	anomalygroup_proto "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	anomalygroup_mock "go.skia.org/infra/perf/go/anomalygroup/proto/v1/mocks"
	culprit_proto "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/workflows"
	pinpoint "go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/catapult"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"

	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/grpc"
)

func setupAnomalyGroupService(t *testing.T) (string, *anomalygroup_mock.AnomalyGroupServiceServer, func()) {
	lis, err := net.Listen("tcp", "localhost:9001")
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
	env.RegisterWorkflowWithOptions(catapult.CulpritFinderWorkflow, workflow.RegisterOptions{Name: pinpoint.CulpritFinderWorkflow})

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
			&anomalygroup_proto.FindTopAnomaliesResponse{Anomalies: []*anomalygroup_proto.Anomaly{mockAnomaly}}, nil)
	mockStartRevision := "revision1"
	mockEndRevision := "revision10"
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, startCommit).Return(mockStartRevision, nil).Once()
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, endCommit).Return(mockEndRevision, nil).Once()

	env.OnWorkflow(pinpoint.CulpritFinderWorkflow,
		mock.Anything,
		&pinpoint.CulpritFinderParams{
			Request: &pinpoint_proto.ScheduleCulpritFinderRequest{
				StartGitHash:         mockStartRevision,
				EndGitHash:           mockEndRevision,
				Configuration:        mockAnomaly.Paramset["bot"],
				Benchmark:            mockAnomaly.Paramset["benchmark"],
				Story:                mockAnomaly.Paramset["story"],
				Chart:                mockAnomaly.Paramset["measurement"],
				Statistic:            "",
				ImprovementDirection: mockAnomaly.ImprovementDirection,
			},
			CallbackParams: &pinpoint_proto.CulpritProcessingCallbackParams{
				AnomalyGroupId:    anomalyGroupId,
				CulpritServiceUrl: c_addr,
			},
		}).Return(&pinpoint_proto.CulpritFinderExecution{
		JobId: "bisectionId",
	}, nil).Once()
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
	env.RegisterWorkflowWithOptions(catapult.CulpritFinderWorkflow, workflow.RegisterOptions{Name: pinpoint.CulpritFinderWorkflow})

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
			&anomalygroup_proto.FindTopAnomaliesResponse{Anomalies: []*anomalygroup_proto.Anomaly{mockAnomaly}}, nil)
	mockStartRevision := "revision1"
	mockEndRevision := "revision10"
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, startCommit).Return(mockStartRevision, nil).Once()
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, endCommit).Return(mockEndRevision, nil).Once()

	env.OnWorkflow(pinpoint.CulpritFinderWorkflow,
		mock.Anything,
		&pinpoint.CulpritFinderParams{
			Request: &pinpoint_proto.ScheduleCulpritFinderRequest{
				StartGitHash:         mockStartRevision,
				EndGitHash:           mockEndRevision,
				Configuration:        mockAnomaly.Paramset["bot"],
				Benchmark:            mockAnomaly.Paramset["benchmark"],
				Story:                mockAnomaly.Paramset["story"],
				Chart:                "runs_per_minute",
				Statistic:            "max",
				ImprovementDirection: mockAnomaly.ImprovementDirection,
			},
			CallbackParams: &pinpoint_proto.CulpritProcessingCallbackParams{
				AnomalyGroupId: anomalyGroupId,
			},
		}).Return(&pinpoint_proto.CulpritFinderExecution{
		JobId: "bisectionId",
	}, nil).Once()
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
	env.RegisterWorkflowWithOptions(catapult.CulpritFinderWorkflow, workflow.RegisterOptions{Name: pinpoint.CulpritFinderWorkflow})

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
	c_server.On("NotifyUserOfAnomaly", mock.Anything, &culprit_proto.NotifyUserOfAnomalyRequest{
		AnomalyGroupId: anomalyGroupId,
		Anomaly:        mockCulpritAnomalies,
	}).Return(
		&culprit_proto.NotifyUserOfAnomalyResponse{}, nil)

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
			&anomalygroup_proto.FindTopAnomaliesResponse{Anomalies: []*anomalygroup_proto.Anomaly{mockAnomaly}}, nil)
	mockStartRevision := "revision1"
	mockEndRevision := "revision10"
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, startCommit).Return(mockStartRevision, nil).Once()
	env.OnActivity(gsa.GetCommitRevision, mock.Anything, endCommit).Return(mockEndRevision, nil).Once()

	env.OnWorkflow(pinpoint.CulpritFinderWorkflow,
		mock.Anything,
		&pinpoint.CulpritFinderParams{
			Request: &pinpoint_proto.ScheduleCulpritFinderRequest{
				StartGitHash:         mockStartRevision,
				EndGitHash:           mockEndRevision,
				Configuration:        mockAnomaly.Paramset["bot"],
				Benchmark:            mockAnomaly.Paramset["benchmark"],
				Story:                "system:health:story:name",
				Chart:                mockAnomaly.Paramset["measurement"],
				Statistic:            "",
				ImprovementDirection: mockAnomaly.ImprovementDirection,
			},
			CallbackParams: &pinpoint_proto.CulpritProcessingCallbackParams{
				AnomalyGroupId:    anomalyGroupId,
				CulpritServiceUrl: c_addr,
			},
		}).Return(&pinpoint_proto.CulpritFinderExecution{
		JobId: "bisectionId",
	}, nil).Once()
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
