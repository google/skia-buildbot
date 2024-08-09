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

	// TODO(wenbinzhang): re-enable when bisection invoke is updated.
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
				AggregationMethod:    mockAnomaly.Paramset["stat"],
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

func TestMaybeTriggerBisection_GroupActionBisect_BadAggregation(t *testing.T) {
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
			"measurement": "runsperminute",
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

	env.ExecuteWorkflow(MaybeTriggerBisectionWorkflow, &workflows.MaybeTriggerBisectionParam{
		AnomalyGroupServiceUrl: addr,
		AnomalyGroupId:         anomalyGroupId,
	})

	require.True(t, env.IsWorkflowCompleted())
	err := env.GetWorkflowError()
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid aggretation method")
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
	c_server.On("NotifyUserOfAnomaly", mock.Anything, &c_pb.NotifyUserOfAnomalyRequest{
		AnomalyGroupId: anomalyGroupId,
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
