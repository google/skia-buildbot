package shared

import "go.skia.org/infra/go/roles"

// AuthorizationPolicy provides a struct to define authz policy for a service.
type AuthorizationPolicy struct {
	AllowUnauthenticated  bool
	AuthorizedRoles       []roles.Role
	MethodAuthorizedRoles map[string][]roles.Role
}
