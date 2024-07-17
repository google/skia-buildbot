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

	// Bisecter can request a bisection.
	Bisecter Role = "bisecter"

	// Buildbucket represents the Buildbucket service.
	Buildbucket Role = "buildbucket"

	// LuciConfig represents the LUCI Config service account.
	LuciConfig Role = "luci_config"

	// If the above roles are not fine grained enough for your application add
	// new Roles here and also remember to add them the AllValidRoles.

)

const (
	// InvalidRole signals an invalid Role.
	InvalidRole Role = ""
)

var (
	// AllValidRoles is all valid Roles.
	AllValidRoles Roles = []Role{Viewer, Editor, Admin, Bisecter, Buildbucket, LuciConfig}

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

// ToHeader converts Roles to a string, formatted for an HTTP header.
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

// Has returns true if the give Role appears in Roles.
func (r Roles) Has(role Role) bool {
	for _, x := range r {
		if x == role {
			return true
		}
	}
	return false
}

// IsAuthorized returns true if r and in contain any of the same roles.
// Note that if either r or in is empty, this will return false.
func (r Roles) IsAuthorized(in Roles) bool {
	for _, role := range in {
		if r.Has(role) {
			return true
		}
	}
	return false
}

// RolesFromStrings parses multiple string values and returns Roles found.
func RolesFromStrings(s ...string) Roles {
	var ret Roles
	for _, r := range s {
		if role := RoleFromString(strings.TrimSpace(r)); role != InvalidRole {
			ret = append(ret, role)
		}
	}
	return ret

}

// FromHeader parses a Roles header value and returns Roles found.
func FromHeader(s string) Roles {
	return RolesFromStrings(strings.Split(s, ",")...)
}
