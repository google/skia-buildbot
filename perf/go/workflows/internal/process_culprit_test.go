package internal

import (
	"net"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	culprit_proto "go.skia.org/infra/perf/go/culprit/proto/v1"
	culprit_mock "go.skia.org/infra/perf/go/culprit/proto/v1/mocks"
	"go.skia.org/infra/perf/go/workflows"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/grpc"
)

func setupProcessCulprit(t *testing.T) (string, *culprit_mock.CulpritServiceServer, func()) {
	lis, err := net.Listen("tcp", "localhost:9000")
	require.NoError(t, err)
	s := grpc.NewServer()
	service := culprit_mock.NewCulpritServiceServer(t)
	culprit_proto.RegisterCulpritServiceServer(s, service)
	go func() {
		require.NoError(t, s.Serve(lis))
	}()
	return lis.Addr().String(), service, func() {
		s.Stop()
	}
}

func TestProcessCulprit_HappyPath_ShouldInvokeCulpritService(t *testing.T) {
	addr, server, cleanup := setupProcessCulprit(t)
	defer cleanup()
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	csa := &CulpritServiceActivity{insecure_conn: true}
	env.RegisterActivity(csa)
	commits := []*culprit_proto.Commit{
		{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main",
			Revision: "123",
		}, {
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main1",
			Revision: "456",
		},
	}

	anomalyGroupId := "111"
	mockCulpritIds := []string{"c1", "c2"}
	mockIssueIds := []string{"b1", "b2"}
	server.On("PersistCulprit", mock.Anything, &culprit_proto.PersistCulpritRequest{
		Commits:        commits,
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&culprit_proto.PersistCulpritResponse{
				CulpritIds: mockCulpritIds}, nil)
	server.On("NotifyUser", mock.Anything, &culprit_proto.NotifyUserRequest{
		CulpritIds:     mockCulpritIds,
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&culprit_proto.NotifyUserResponse{
				IssueIds: mockIssueIds}, nil)

	env.ExecuteWorkflow(ProcessCulpritWorkflow, &workflows.ProcessCulpritParam{
		CulpritServiceUrl: addr,
		Commits:           commits,
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
