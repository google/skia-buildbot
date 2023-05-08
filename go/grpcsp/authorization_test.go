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
)

type mockServer struct {
	handleCalled bool
}

func (ts *mockServer) handle(ctx context.Context, req interface{}) (interface{}, error) {
	ts.handleCalled = true
	return nil, nil
}

func testSetupOneMethod(t *testing.T) (grpc.ServiceDesc, *policy) {
	desc := grpc.ServiceDesc{
		ServiceName: "foo.service.Example",
		Methods: []grpc.MethodDesc{
			{
				MethodName: MethodOneName,
			},
		},
	}
	policy := Authorization(desc)
	require.NotNil(t, policy)
	return desc, policy
}

func testSetupTwoMethods(t *testing.T) (grpc.ServiceDesc, *policy) {
	desc := grpc.ServiceDesc{
		ServiceName: "foo.service.Example",
		Methods: []grpc.MethodDesc{
			{
				MethodName: MethodOneName,
			},
			{
				MethodName: MethodTwoName,
			},
		},
	}
	policy := Authorization(desc)
	require.NotNil(t, policy)
	return desc, policy
}

func testSetupUnaryInterceptor(t *testing.T, desc grpc.ServiceDesc, policy *policy, callerRoles roles.Roles) (context.Context, grpc.UnaryServerInterceptor) {
	ctx := context.Background()
	md := metadata.New(map[string]string{
		authproxy.WebAuthHeaderName:     "user@domain.com",
		authproxy.WebAuthRoleHeaderName: callerRoles.ToHeader(),
	})
	ctx = metadata.NewIncomingContext(ctx, md)

	interceptor := policy.UnaryInterceptor()
	assert.NotNil(t, interceptor)
	return ctx, interceptor
}

func TestAuthorizeRoles(t *testing.T) {
	desc := grpc.ServiceDesc{}
	policy := Authorization(desc)
	require.NotNil(t, policy)

	err := policy.AuthorizeRoles(roles.Roles{roles.Viewer})
	assert.NoError(t, err)

	err = policy.AuthorizeRoles(roles.Roles{roles.Viewer})
	assert.Error(t, err, "calling AuthorizeRoles more than once returns an error")
}

func TestAuthorizeMethodForRoles_UnknownMethod_ReturnsError(t *testing.T) {
	desc := grpc.ServiceDesc{}
	policy := Authorization(desc)
	require.NotNil(t, policy)
	err := policy.AuthorizeMethodForRoles(MethodOneName, roles.Roles{roles.Viewer})
	assert.Error(t, err, "cannot authorize an unrecognized method")
}

func TestAuthorizeMethodForRoles_DuplicateMethod_ReturnsError(t *testing.T) {
	_, policy := testSetupOneMethod(t)
	err := policy.AuthorizeMethodForRoles(MethodOneName, roles.Roles{roles.Viewer})
	assert.NoError(t, err)
	err = policy.AuthorizeMethodForRoles(MethodOneName, roles.Roles{roles.Editor})
	assert.Error(t, err, "calling AuthorizeMethodForRoles more than once for the same method returns an error")
}

func TestUnaryInterceptor_UserHasSufficientServiceWideRoles_Succeed(t *testing.T) {
	desc, policy := testSetupTwoMethods(t)
	err := policy.AuthorizeRoles(roles.Roles{roles.Admin})
	assert.NoError(t, err)

	ctx, interceptor := testSetupUnaryInterceptor(t, desc, policy, roles.Roles{roles.Admin})

	var info *grpc.UnaryServerInfo
	var srv *mockServer

	srv = &mockServer{}
	info = &grpc.UnaryServerInfo{FullMethod: "/" + desc.ServiceName + "/" + MethodOneName}
	_, err = interceptor(ctx, nil, info, srv.handle)
	require.NoError(t, err, "users with service-wide authorized roles may call any method")
	assert.True(t, srv.handleCalled, "control passed on to server handler")

	srv = &mockServer{}
	info = &grpc.UnaryServerInfo{FullMethod: "/" + desc.ServiceName + "/" + MethodTwoName}
	_, err = interceptor(ctx, nil, info, srv.handle)
	require.NoError(t, err, "users with service-wide authorized roles may call any method")
	assert.True(t, srv.handleCalled, "control passed on to server handler")
}

func TestUnaryInterceptor_ServiceHasServiceWideRolesUserHasInsufficientRoles_Fail(t *testing.T) {
	desc, policy := testSetupTwoMethods(t)
	err := policy.AuthorizeRoles(roles.Roles{roles.Admin})
	assert.NoError(t, err)

	ctx, interceptor := testSetupUnaryInterceptor(t, desc, policy, roles.Roles{roles.Viewer})

	var info *grpc.UnaryServerInfo
	var srv *mockServer

	srv = &mockServer{}
	info = &grpc.UnaryServerInfo{FullMethod: "/" + desc.ServiceName + "/" + MethodOneName}
	_, err = interceptor(ctx, nil, info, srv.handle)
	require.Error(t, err, "users with insufficient service-wide roles may not call methods when the policy has no method-specific roles")
	assert.False(t, srv.handleCalled, "control is not passed on to server handler")

	srv = &mockServer{}
	info = &grpc.UnaryServerInfo{FullMethod: "/" + desc.ServiceName + "/" + MethodTwoName}
	_, err = interceptor(ctx, nil, info, srv.handle)
	require.Error(t, err, "users with insufficient service-wide roles may not call methods when the policy has no method-specific roles")
	assert.False(t, srv.handleCalled, "control is not passed on to server handler")
}

func TestUnaryInterceptor_UserHasSufficientMethodRoles_Succeed(t *testing.T) {
	desc, policy := testSetupTwoMethods(t)

	err := policy.AuthorizeMethodForRoles(MethodOneName, roles.Roles{roles.Viewer})
	require.NoError(t, err)
	err = policy.AuthorizeMethodForRoles(MethodTwoName, roles.Roles{roles.Editor})
	require.NoError(t, err)

	ctx, interceptor := testSetupUnaryInterceptor(t, desc, policy, roles.Roles{roles.Viewer})

	var info *grpc.UnaryServerInfo
	var srv *mockServer

	info = &grpc.UnaryServerInfo{FullMethod: "/" + desc.ServiceName + "/" + MethodOneName}
	srv = &mockServer{}
	_, err = interceptor(ctx, nil, info, srv.handle)
	assert.NoError(t, err, "viewer is authorized to /"+MethodOneName)
	assert.True(t, srv.handleCalled, "control passed on to server handler")
}

func TestUnaryInterceptor_UserHasInsufficientMethodRoles_Fail(t *testing.T) {
	desc, policy := testSetupTwoMethods(t)

	err := policy.AuthorizeMethodForRoles(MethodOneName, roles.Roles{roles.Viewer})
	require.NoError(t, err)
	err = policy.AuthorizeMethodForRoles(MethodTwoName, roles.Roles{roles.Editor})
	require.NoError(t, err)

	ctx, interceptor := testSetupUnaryInterceptor(t, desc, policy, roles.Roles{roles.Viewer})

	var info *grpc.UnaryServerInfo
	var srv *mockServer
	info = &grpc.UnaryServerInfo{FullMethod: "/" + desc.ServiceName + "/" + MethodTwoName}
	srv = &mockServer{}
	_, err = interceptor(ctx, nil, info, srv.handle)
	assert.Error(t, err, "viewer isn't authorized to /"+MethodTwoName)
	assert.False(t, srv.handleCalled, "control is not passed on to server handler")
}
