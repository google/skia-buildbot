// Package roles are part of the Skia Infra Authorization system.
//
// See go/proxy-auth-skia.
package roles

import (
	"strings"
)

// Role for an authorized user.
type Role string

const (
	// Viewer has read-only access to the application.
	Viewer Role = "viewer"

	// Editor has read-write access to the application.
	Editor Role = "editor"

	// Admin has admin access to the application.
	Admin Role = "admin"

	// If the above roles are not fine grained enough for your application add
	// new Roles here and also remember to add them the AllValidRoles.

)

const (
	// InvalidRole signals an invalid Role.
	InvalidRole Role = ""
)

var (
	// AllValidRoles is all valid Roles.
	AllValidRoles Roles = []Role{Viewer, Editor, Admin}

	// AllRoles is all Roles including InvalidRole.
	AllRoles Roles = append(AllValidRoles, InvalidRole)
)

// RoleFromString converts a string to a Role, returning InvalidRole, which is the
// empty string, if the passed in value is not a valid role.
func RoleFromString(s string) Role {
	for _, role := range AllRoles {
		if string(role) == s {
			return role
		}
	}
	return InvalidRole
}

// Roles is a slice of Role.
type Roles []Role

// ToHeader converts Roles to a string, formatted for the X-ROLES header.
func (r Roles) ToHeader() string {
	var b strings.Builder
	last := len(r) - 1
	for i, role := range r {
		b.Write([]byte(role))
		if i != last {
			b.WriteString(",")
		}
	}
	return b.String()
}

// FromHeader parses the X-ROLES header value and returns Roles found.
func FromHeader(s string) Roles {
	var ret Roles
	for _, part := range strings.Split(s, ",") {
		if role := RoleFromString(strings.TrimSpace(part)); role != InvalidRole {
			ret = append(ret, role)
		}
	}
	return ret
}
