package grpcsp

import (
	"context"
	"testing"

	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/kube/go/authproxy"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	MethodOneName = "Foo"
	MethodTwoName = "Bar"
	ServiceName   = "foo.service.Example"
)

type mockServer struct {
	handleCalled bool
}

func (ts *mockServer) handle(ctx context.Context, req interface{}) (interface{}, error) {
	ts.handleCalled = true
	return nil, nil
}

func testSetupEmpty(t *testing.T) *ServicePolicy {
	desc := grpc.ServiceDesc{}
	serverPolicy := Server()
	servicePolicy, err := serverPolicy.Service(desc)
	require.NoError(t, err)
	require.NotNil(t, servicePolicy)
	return servicePolicy
}

func testSetupOneMethod(t *testing.T) (grpc.ServiceDesc, *ServerPolicy, *ServicePolicy) {
	desc := grpc.ServiceDesc{
		ServiceName: ServiceName,
		Methods: []grpc.MethodDesc{
			{
				MethodName: MethodOneName,
			},
		},
	}
	serverPolicy := Server()
	servicePolicy, err := serverPolicy.Service(desc)
	require.NoError(t, err)
	require.NotNil(t, servicePolicy)
	return desc, serverPolicy, servicePolicy
}

func testSetupTwoMethods(t *testing.T) (grpc.ServiceDesc, *ServerPolicy, *ServicePolicy) {
	desc := grpc.ServiceDesc{
		ServiceName: ServiceName,
		Methods: []grpc.MethodDesc{
			{
				MethodName: MethodOneName,
			},
			{
				MethodName: MethodTwoName,
			},
		},
	}
	serverPolicy := Server()
	servicePolicy, err := serverPolicy.Service(desc)
	require.NoError(t, err)
	require.NotNil(t, servicePolicy)
	return desc, serverPolicy, servicePolicy
}

func testSetupUnaryInterceptorWithCallerRoles(t *testing.T, desc grpc.ServiceDesc, policy *ServerPolicy, callerRoles roles.Roles) (context.Context, grpc.UnaryServerInterceptor) {
	ctx := context.Background()
	md := metadata.New(map[string]string{
		authproxy.WebAuthHeaderName:     "user@domain.com",
		authproxy.WebAuthRoleHeaderName: callerRoles.ToHeader(),
	})
	ctx = metadata.NewIncomingContext(ctx, md)

	interceptor := policy.UnaryInterceptor()
	require.NotNil(t, interceptor)
	return ctx, interceptor
}

func testMockServerCall(t *testing.T, ctx context.Context, serviceName, methodName string, interceptor grpc.UnaryServerInterceptor) (bool, error) {
	info := &grpc.UnaryServerInfo{FullMethod: "/" + serviceName + "/" + methodName}
	srv := &mockServer{}
	_, err := interceptor(ctx, nil, info, srv.handle)
	return srv.handleCalled, err
}

func TestAuthorizeRoles(t *testing.T) {
	policy := testSetupEmpty(t)

	assert.NoError(t, policy.AuthorizeRoles(roles.Roles{roles.Viewer}))
	assert.Error(t, policy.AuthorizeRoles(roles.Roles{roles.Viewer}), "calling AuthorizeRoles more than once returns an error")
}

func TestAuthorizeMethodForRoles_UnknownMethod_ReturnsError(t *testing.T) {
	policy := testSetupEmpty(t)

	assert.Error(t, policy.AuthorizeMethodForRoles(MethodOneName, roles.Roles{roles.Viewer}), "cannot authorize an unrecognized method")
}

func TestAuthorizeMethodForRoles_DuplicateMethod_ReturnsError(t *testing.T) {
	_, _, servicePolicy := testSetupOneMethod(t)
	assert.NoError(t, servicePolicy.AuthorizeMethodForRoles(MethodOneName, roles.Roles{roles.Viewer}))
	assert.Error(t, servicePolicy.AuthorizeMethodForRoles(MethodOneName, roles.Roles{roles.Editor}),
		"calling AuthorizeMethodForRoles more than once for the same method returns an error")
}

func TestUnaryInterceptor_UserHasSufficientServiceWideRoles_Succeed(t *testing.T) {
	desc, serverPolicy, servicePolicy := testSetupTwoMethods(t)
	require.NoError(t, servicePolicy.AuthorizeRoles(roles.Roles{roles.Admin}))

	ctx, interceptor := testSetupUnaryInterceptorWithCallerRoles(t, desc, serverPolicy, roles.Roles{roles.Admin})

	handleCalled, err := testMockServerCall(t, ctx, desc.ServiceName, MethodOneName, interceptor)
	assert.NoError(t, err, "users with service-wide authorized roles may call any method")
	assert.True(t, handleCalled, "control passed on to server handler")

	handleCalled, err = testMockServerCall(t, ctx, desc.ServiceName, MethodTwoName, interceptor)
	require.NoError(t, err, "users with service-wide authorized roles may call any method")
	assert.True(t, handleCalled, "control passed on to server handler")
}

func TestUnaryInterceptor_ServiceHasServiceWideRolesUserHasInsufficientRoles_Fail(t *testing.T) {
	desc, serverPolicy, servicePolicy := testSetupTwoMethods(t)
	require.NoError(t, servicePolicy.AuthorizeRoles(roles.Roles{roles.Admin}))

	ctx, interceptor := testSetupUnaryInterceptorWithCallerRoles(t, desc, serverPolicy, roles.Roles{roles.Viewer})

	handleCalled, err := testMockServerCall(t, ctx, desc.ServiceName, MethodOneName, interceptor)
	require.Error(t, err, "users with insufficient service-wide roles may not call methods when the policy has no method-specific roles")
	assert.False(t, handleCalled, "control is not passed on to server handler")

	handleCalled, err = testMockServerCall(t, ctx, desc.ServiceName, MethodTwoName, interceptor)
	require.Error(t, err, "users with insufficient service-wide roles may not call methods when the policy has no method-specific roles")
	assert.False(t, handleCalled, "control is not passed on to server handler")
}

func TestUnaryInterceptor_UserHasSufficientMethodRoles_Succeed(t *testing.T) {
	desc, serverPolicy, servicePolicy := testSetupTwoMethods(t)
	require.NoError(t, servicePolicy.AuthorizeMethodForRoles(MethodOneName, roles.Roles{roles.Viewer}))
	require.NoError(t, servicePolicy.AuthorizeMethodForRoles(MethodTwoName, roles.Roles{roles.Editor}))

	ctx, interceptor := testSetupUnaryInterceptorWithCallerRoles(t, desc, serverPolicy, roles.Roles{roles.Viewer})

	handleCalled, err := testMockServerCall(t, ctx, desc.ServiceName, MethodOneName, interceptor)
	assert.NoError(t, err, "viewer is authorized to call /"+MethodOneName)
	assert.True(t, handleCalled, "control passed on to server handler")
}

func TestUnaryInterceptor_UserHasInsufficientMethodRoles_Fail(t *testing.T) {
	desc, serverPolicy, servicePolicy := testSetupTwoMethods(t)
	require.NoError(t, servicePolicy.AuthorizeMethodForRoles(MethodOneName, roles.Roles{roles.Viewer}))
	require.NoError(t, servicePolicy.AuthorizeMethodForRoles(MethodTwoName, roles.Roles{roles.Editor}))

	ctx, interceptor := testSetupUnaryInterceptorWithCallerRoles(t, desc, serverPolicy, roles.Roles{roles.Viewer})

	handleCalled, err := testMockServerCall(t, ctx, desc.ServiceName, MethodTwoName, interceptor)
	assert.Error(t, err, "viewer isn't authorized to /"+MethodTwoName)
	assert.False(t, handleCalled, "control is not passed on to server handler")
}
