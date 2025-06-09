package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

const (
	// WebAuthHeaderName is the name of the header sent to the application that
	// contains the users email address.
	WebAuthHeaderName = "X-WEBAUTH-USER"

	// WebAuthRoleHeaderName is the name of the header sent to the application
	// that contains the users Roles.
	WebAuthRoleHeaderName = "X-WEBAUTH-ROLES"
)

// authKey is a custom context key for storing the auth token.
type authKey struct{}

// AuthData provides a struct to store auth information passed on from
// the auth proxy.
type AuthData struct {
	UserEmail string
	UserRoles []string
}

// withAuthKey adds an auth key to the context.
func withAuthKey(ctx context.Context, authData AuthData) context.Context {
	return context.WithValue(ctx, authKey{}, authData)
}

// AuthFromRequest extracts the auth data from the request headers.
func AuthFromRequest(ctx context.Context, r *http.Request) context.Context {
	var email string
	var roles []string

	value := r.Header.Get(WebAuthHeaderName)
	if value != "" {
		email = strings.TrimSpace(value)
	}

	rolesStr := r.Header.Get(WebAuthRoleHeaderName)

	if rolesStr != "" {
		roles = strings.Split(rolesStr, ",")
	}
	authData := AuthData{
		UserEmail: email,
		UserRoles: roles,
	}

	return withAuthKey(ctx, authData)
}

// AuthDataFromContext extracts the auth information from the context.
func AuthDataFromContext(ctx context.Context) (AuthData, error) {
	authData, ok := ctx.Value(authKey{}).(AuthData)
	if !ok {
		return AuthData{}, fmt.Errorf("missing auth")
	}
	return authData, nil
}
