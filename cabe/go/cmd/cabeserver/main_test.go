package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"google.golang.org/api/bigquery/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	cpb "go.skia.org/infra/cabe/go/proto"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/kube/go/authproxy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSetupAppWithBackends(t *testing.T) (context.Context, *App, func()) {
	ctx := context.Background()

	a := &App{
		port:       ":0",
		grpcPort:   ":0",
		promPort:   ":0",
		bqClient:   &bigquery.Service{},
		rbeClients: map[string]*rbeclient.Client{},
	}
	var w sync.WaitGroup
	w.Add(1)
	go func() {
		err := a.ConfigureAuthorization()
		require.NoError(t, err)
		err = a.Start(ctx)
		assert.NoError(t, err)
		w.Done()
	}()
	time.Sleep(time.Second)

	return ctx, a, func() {
		a.Cleanup()
		w.Wait()
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
	var w sync.WaitGroup
	w.Add(1)
	go func() {
		err := a.Start(ctx)
		assert.Error(t, err)
		w.Done()
	}()
	w.Wait()
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

	resp, err := analysisClient.GetAnalysis(ctx, &cpb.GetAnalysisRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGRPCAuthorizationPolicy_UserIsNotAuthorized_Fails(t *testing.T) {
	ctx, a, cleanup := testSetupAppWithBackends(t)
	defer cleanup()

	ctx, analysisClient := testSetupClientWithUserInRoles(t, ctx, a, roles.Roles{roles.Editor})

	resp, err := analysisClient.GetAnalysis(ctx, &cpb.GetAnalysisRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}
