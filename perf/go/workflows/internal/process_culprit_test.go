package internal

import (
	"net"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	culprit_proto "go.skia.org/infra/perf/go/culprit/proto/v1"
	culprit_mock "go.skia.org/infra/perf/go/culprit/proto/v1/mocks"
	"go.skia.org/infra/perf/go/workflows"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/grpc"
)

func setupCulpritService(t *testing.T) (string, *culprit_mock.CulpritServiceServer, func()) {
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
	addr, service, cleanup := setupCulpritService(t)
	defer cleanup()
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	csa := &CulpritServiceActivity{insecure_conn: true}
	env.RegisterActivity(csa)
	pp_commits := []*pinpoint_proto.Commit{
		{
			GitHash:    "123",
			Repository: "https://chromium.googlesource.com/chromium/src.git",
		},
		{
			GitHash:    "456",
			Repository: "https://chromium.googlesource.com/chromium/src.git",
		},
	}
	commits := []*culprit_proto.Commit{
		{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "",
			Revision: "123",
		}, {
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "",
			Revision: "456",
		},
	}

	anomalyGroupId := "111"
	mockCulpritIds := []string{"c1", "c2"}
	mockIssueIds := []string{"b1", "b2"}
	service.On("PersistCulprit", mock.Anything, &culprit_proto.PersistCulpritRequest{
		Commits:        commits,
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&culprit_proto.PersistCulpritResponse{
				CulpritIds: mockCulpritIds}, nil)
	service.On("NotifyUserOfCulprit", mock.Anything, &culprit_proto.NotifyUserOfCulpritRequest{
		CulpritIds:     mockCulpritIds,
		AnomalyGroupId: anomalyGroupId}).
		Return(
			&culprit_proto.NotifyUserOfCulpritResponse{
				IssueIds: mockIssueIds}, nil)

	env.ExecuteWorkflow(ProcessCulpritWorkflow, &workflows.ProcessCulpritParam{
		CulpritServiceUrl: addr,
		Commits:           pp_commits,
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

func TestProcessCulprit_HappyPath_InvalidCulpritRepo(t *testing.T) {
	addr, _, cleanup := setupCulpritService(t)
	defer cleanup()
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	csa := &CulpritServiceActivity{insecure_conn: true}
	env.RegisterActivity(csa)
	pp_commits := []*pinpoint_proto.Commit{
		{
			GitHash:    "123",
			Repository: "https://chromium.googlesource.com",
		},
	}

	env.ExecuteWorkflow(ProcessCulpritWorkflow, &workflows.ProcessCulpritParam{
		CulpritServiceUrl: addr,
		Commits:           pp_commits,
		AnomalyGroupId:    "111",
	})

	require.True(t, env.IsWorkflowCompleted())
	err := env.GetWorkflowError()
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid commit repository")
}

func TestParsePinpointCommit_Success(t *testing.T) {
	pinpoint_commit := &pinpoint_proto.Commit{
		Repository: "https://chromium.googlesource.com/v8/v8.git",
		GitHash:    "deadbeef1234",
	}
	culprit_commit, err := ParsePinpointCommit(pinpoint_commit)
	require.NoError(t, err)
	require.Equal(t, culprit_commit.Host, "chromium.googlesource.com")
	require.Equal(t, culprit_commit.Project, "v8/v8")
	require.Equal(t, culprit_commit.Revision, "deadbeef1234")
}

func TestParsePinpointCommit_ShortRepo(t *testing.T) {
	pinpoint_commit := &pinpoint_proto.Commit{
		Repository: "https://chromium",
		GitHash:    "deadbeef1234",
	}
	_, err := ParsePinpointCommit(pinpoint_commit)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid commit repository")
}

func TestParsePinpointCommit_EmptyProject(t *testing.T) {
	pinpoint_commit := &pinpoint_proto.Commit{
		Repository: "https://chromium.googlesource.com/",
		GitHash:    "deadbeef1234",
	}
	_, err := ParsePinpointCommit(pinpoint_commit)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Empty values parsed")
}

func TestParsePinpointCommit_EmptyHash(t *testing.T) {
	pinpoint_commit := &pinpoint_proto.Commit{
		Repository: "https://chromium.googlesource.com/v8/v8.git",
		GitHash:    "",
	}
	_, err := ParsePinpointCommit(pinpoint_commit)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Empty values parsed")
}
