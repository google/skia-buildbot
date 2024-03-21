package internal

import (
	"net"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	pbmock "go.skia.org/infra/perf/go/culprit/proto/mocks"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/workflows"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/grpc"
)

func setup(t *testing.T) (string, *pbmock.CulpritServiceServer, func()) {
	lis, err := net.Listen("tcp", "localhost:9000")
	require.NoError(t, err)
	s := grpc.NewServer()
	service := pbmock.NewCulpritServiceServer(t)
	pb.RegisterCulpritServiceServer(s, service)
	go func() {
		require.NoError(t, s.Serve(lis))
	}()
	return lis.Addr().String(), service, func() {
		s.Stop()
	}
}

func TestProcessCulprit_HappyPath_ShouldInvokeCulpritService(t *testing.T) {
	addr, server, cleanup := setup(t)
	defer cleanup()
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	csa := &CulpritServiceActivity{}
	env.RegisterActivity(csa)
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
	anomalyGroupId := "111"
	mockCulpritIds := []string{"c1", "c2"}
	mockIssueIds := []string{"b1", "b2"}
	server.On("PersistCulprit", mock.Anything, &pb.PersistCulpritRequest{
		Culprits:       culprits,
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&pb.PersistCulpritResponse{
				CulpritIds: mockCulpritIds}, nil)
	server.On("NotifyUser", mock.Anything, &pb.NotifyUserRequest{
		CulpritIds:     mockCulpritIds,
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&pb.NotifyUserResponse{
				IssueIds: mockIssueIds}, nil)

	env.ExecuteWorkflow(ProcessCulpritWorkflow, &workflows.ProcessCulpritParam{
		CulpritServiceUrl: addr,
		Culprits:          culprits,
		AnomalyGroupId:    anomalyGroupId,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var resp *workflows.ProcessCulpritResult
	require.NoError(t, env.GetWorkflowResult(&resp))
	require.Equal(t, resp.CulpritIds, mockCulpritIds)
	require.Equal(t, resp.IssueIds, mockIssueIds)
	env.AssertExpectations(t)
}
