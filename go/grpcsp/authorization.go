// Package grpcsp implements grpc server interceptors to apply role-based
// access control to a grpc service. It is intended to work with headers
// set by [go.skia.org/infra/kube/go/authproxy] on incoming requests.
package grpcsp

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/kube/go/authproxy"
)

type policy struct {
	desc           grpc.ServiceDesc
	allowRoles     roles.Roles
	rolesForMethod map[string]roles.Roles
}

// Authorization returns a new configurable server authorization policy.
// The policy is conservative in that anything that isn't explicitly allowed
// by the policy is denied.
func Authorization(desc grpc.ServiceDesc) *policy {
	ret := &policy{
		desc:           desc,
		allowRoles:     nil,
		rolesForMethod: map[string]roles.Roles{},
	}
	for _, m := range desc.Methods {
		fullPath := "/" + desc.ServiceName + "/" + m.MethodName
		ret.rolesForMethod[fullPath] = nil
	}
	return ret
}

// AuthorizeRoles configures the policy to allow users with any of the given [role]
// values to make calls to any method. Authorize multiple roles by passing multiple role
// values. Calling this more than once results in an error.
func (p *policy) AuthorizeRoles(r roles.Roles) error {
	if p.allowRoles != nil {
		return fmt.Errorf("allowed roles for %q have already been set", p.desc.ServiceName)
	}
	p.allowRoles = r
	return nil
}

// AuthorizeMethodForRoles configures the policy to allow users with any of the given [role]
// values to make calls to [method]. Authorize multiple roles by passing multiple role
// values. Calling this more than once with the same [method] results in an error. Calling
// this with a method not included in the service description results in an error.
func (p *policy) AuthorizeMethodForRoles(method string, r roles.Roles) error {
	fullPath := "/" + p.desc.ServiceName + "/" + method
	rfm, ok := p.rolesForMethod[fullPath]
	if !ok {
		return fmt.Errorf("unknown grpc method: %q for service", method, p.desc.ServiceName)
	}
	if rfm != nil {
		return fmt.Errorf("already have roles set for method: %q", method)
	}

	p.rolesForMethod[fullPath] = r
	return nil
}

// rolesFromContext is dependent on specific headers set by skia's auth-proxy implementation.
func rolesFromContext(ctx context.Context) (roles.Roles, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "could not authorize without metadata from incoming context")
	}
	user := md.Get(authproxy.WebAuthHeaderName)
	if len(user) == 0 {
		return nil, status.Error(codes.PermissionDenied, "could not authorize without user identity from incoming context")
	}
	return roles.RolesFromStrings(md.Get(authproxy.WebAuthRoleHeaderName)...), nil
}

// UnaryInterceptor returns a [grpc.UnaryServerInterceptor] that applies role checks defined
// by the policy to incoming requests. Requests that do not satisfy the policy result in
// a [codes.PermissionDenied] response code returned to the caller.
func (p *policy) UnaryInterceptor() grpc.UnaryServerInterceptor {
	ret := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		roleSet, err := rolesFromContext(ctx)
		if err != nil {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		if p.allowRoles.IsAuthorized(roleSet) {
			return handler(ctx, req)
		}

		allowedRoleSet, ok := p.rolesForMethod[info.FullMethod]
		if !ok {
			return nil, status.Error(codes.PermissionDenied, "unrecognized method")
		}
		if !allowedRoleSet.IsAuthorized(roleSet) {
			return nil, status.Error(codes.PermissionDenied, "user does not have required role(s)")
		}
		return handler(ctx, req)
	}
	return ret
}
