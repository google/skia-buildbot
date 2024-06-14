package main

import (
	"context"
	"fmt"
	"testing"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	cpb "go.skia.org/infra/cabe/go/proto"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/swarming/v2/mocks"
	"go.skia.org/infra/kube/go/authproxy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testSetupAppWithBackends(t *testing.T) (context.Context, *App, func()) {
	ctx := context.Background()
	swarmingClient := &mocks.SwarmingV2Client{}
	anything := mock.MatchedBy(func(any) bool { return true })
	swarmingClient.On("ListTasks",
		anything, anything, anything, anything, anything).Return(&apipb.TaskListResponse{}, nil).Maybe()
	a := &App{
		port:           ":0",
		grpcPort:       ":0",
		promPort:       ":0",
		rbeClients:     map[string]*rbeclient.Client{},
		swarmingClient: swarmingClient,
	}
	ch := make(chan interface{})

	err := a.ConfigureAuthorization()
	require.NoError(t, err)
	err = a.Init(ctx)
	assert.NoError(t, err)
	go func() {
		err := a.ServeGRPC(ctx)
		assert.NoError(t, err)
		ch <- nil
	}()
	go func() {
		err := a.ServeHTTP()
		assert.NoError(t, err)
		ch <- nil
	}()

	return ctx, a, func() {
		a.Cleanup()
		<-ch
		<-ch
	}
}

func testSetupClientWithUserInRoles(t *testing.T, ctx context.Context, a *App, roles roles.Roles) (context.Context, cpb.AnalysisClient) {
	conn, err := grpc.Dial(a.grpcPort, grpc.WithTransportCredentials(insecure.NewCredentials()))

	require.NoError(t, err)

	analysisClient := cpb.NewAnalysisClient(conn)

	md := metadata.MD{}
	md.Set(authproxy.WebAuthHeaderName, "user@google.com")
	md.Set(authproxy.WebAuthRoleHeaderName, roles.ToHeader())
	return metadata.NewOutgoingContext(ctx, md), analysisClient
}

func TestApp_StartNoBackends_Fails(t *testing.T) {
	a := &App{
		port:     ":0",
		grpcPort: ":0",
		promPort: ":0",
	}
	ctx := context.Background()

	err := a.Init(ctx)
	assert.Error(t, err)
}

func TestApp_StartWithBackendsAndHTTPHealthCheck_Succeeds(t *testing.T) {
	_, a, cleanup := testSetupAppWithBackends(t)
	defer cleanup()
	// Make a health check http request.
	client := httputils.NewFastTimeoutClient()
	healthz := fmt.Sprintf("http://%s/healthz", a.port)

	resp, err := client.Get(healthz)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestApp_StartWithBackendsAndGRPCHealthCheck_Succeeds(t *testing.T) {
	ctx, a, cleanup := testSetupAppWithBackends(t)
	defer cleanup()

	// Make a health check rpc.
	conn, err := grpc.Dial(a.grpcPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	healthClient := grpc_health_v1.NewHealthClient(conn)
	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGRPCAuthorizationPolicy_UserIsAuthorized_Succeeds(t *testing.T) {
	ctx, a, cleanup := testSetupAppWithBackends(t)
	defer cleanup()

	ctx, analysisClient := testSetupClientWithUserInRoles(t, ctx, a, roles.Roles{roles.Viewer})

	// Since this test server doesn't use replaybackends, just the swarming mock API, there's no
	// concise way to have this return a set of cpb.AnalysisResult values to assert against.
	// So this test just makes sure that if there's an error then it isn't "PermissionDenied",
	// at least.
	_, err := analysisClient.GetAnalysis(ctx, &cpb.GetAnalysisRequest{
		PinpointJobId: "123",
	})
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.NotEqual(t, st.Code(), codes.PermissionDenied)
}

func TestGRPCAuthorizationPolicy_UserIsNotAuthorized_Fails(t *testing.T) {
	ctx, a, cleanup := testSetupAppWithBackends(t)
	defer cleanup()

	ctx, analysisClient := testSetupClientWithUserInRoles(t, ctx, a, roles.Roles{roles.Editor})

	resp, err := analysisClient.GetAnalysis(ctx, &cpb.GetAnalysisRequest{
		PinpointJobId: "123",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, st.Code(), codes.PermissionDenied)
	assert.Nil(t, resp)
}
