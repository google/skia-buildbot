package internal

import (
	"net"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	ag_proto "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	ag_mock "go.skia.org/infra/perf/go/anomalygroup/proto/v1/mocks"
	"go.skia.org/infra/perf/go/workflows"
	pinpoint "go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/catapult"

	// "go.skia.org/infra/pinpoint/go/workflows/catapult"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/grpc"
)

func setupTriggerBisection(t *testing.T) (string, *ag_mock.AnomalyGroupServiceServer, func()) {
	lis, err := net.Listen("tcp", "localhost:9001")
	require.NoError(t, err)
	s := grpc.NewServer()
	service := ag_mock.NewAnomalyGroupServiceServer(t)
	ag_proto.RegisterAnomalyGroupServiceServer(s, service)
	go func() {
		require.NoError(t, s.Serve(lis))
	}()
	return lis.Addr().String(), service, func() {
		s.Stop()
	}
}

func TestMaybeTriggerBisection_GroupActionBisect_HappyPath(t *testing.T) {
	addr, server, cleanup := setupTriggerBisection(t)
	defer cleanup()
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	agsa := &AnomalyGroupServiceActivity{insecure_conn: true}
	env.RegisterActivity(agsa)
	env.RegisterWorkflowWithOptions(catapult.CatapultBisectWorkflow, workflow.RegisterOptions{Name: pinpoint.CatapultBisect})

	anomalyGroupId := "group_id1"
	mockAnomalyIds := []string{"anomaly1"}
	mockAnomaly := &ag_proto.Anomaly{
		StartCommit: 1,
		EndCommit:   10,
		Paramset: map[string]string{
			"bot":         "linux-perf",
			"benchmark":   "speedometer",
			"story":       "speedometer",
			"measurement": "runsperminute",
			"stat":        "sum",
		},
		ImprovementDirection: "UP",
	}
	server.On("LoadAnomalyGroupByID", mock.Anything, &ag_proto.LoadAnomalyGroupByIDRequest{
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&ag_proto.LoadAnomalyGroupByIDResponse{
				AnomalyGroup: &ag_proto.AnomalyGroup{
					GroupId:     anomalyGroupId,
					GroupAction: ag_proto.GroupActionType_BISECT,
					AnomalyIds:  mockAnomalyIds,
				},
			}, nil)
	server.On("FindTopAnomalies", mock.Anything, &ag_proto.FindTopAnomaliesRequest{
		AnomalyGroupId: anomalyGroupId,
		Limit:          10}).
		Return(
			&ag_proto.FindTopAnomaliesResponse{Anomalies: []*ag_proto.Anomaly{mockAnomaly}}, nil)
	env.OnWorkflow(pinpoint.CatapultBisect, mock.Anything,
		&pinpoint.BisectParams{
			Request: &pinpoint_proto.ScheduleBisectRequest{
				ComparisonMode:       "performance",
				StartGitHash:         "test",
				EndGitHash:           "test",
				Configuration:        mockAnomaly.Paramset["bot"],
				Benchmark:            mockAnomaly.Paramset["benchmark"],
				Story:                mockAnomaly.Paramset["story"],
				Chart:                mockAnomaly.Paramset["measurement"],
				AggregationMethod:    mockAnomaly.Paramset["stat"],
				ImprovementDirection: mockAnomaly.ImprovementDirection,
			},
		}).Return(&pinpoint_proto.BisectExecution{
		JobId: "bisectionId",
	}, nil).Once()
	server.On("UpdateAnomalyGroup", mock.Anything, &ag_proto.UpdateAnomalyGroupRequest{
		BisectionId:    "bisectionId",
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&ag_proto.UpdateAnomalyGroupResponse{}, nil)

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
