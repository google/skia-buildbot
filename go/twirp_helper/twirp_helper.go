package twirp_helper

import (
	"context"
	"fmt"
	"net/http"

	"github.com/twitchtv/twirp"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/login"
)

/*
	Helpers for working with Twirp.
*/

// AuthHelper provides methods for authenticating users.
type AuthHelper struct {
	viewers allowed.Allow
	editors allowed.Allow
	admins  allowed.Allow
}

// NewAuthHelper returns an AuthHelper instance.
func NewAuthHelper(viewers, editors, admins allowed.Allow) *AuthHelper {
	return &AuthHelper{
		viewers: viewers,
		editors: editors,
		admins:  admins,
	}
}

// GetUser returns the email address of the logged-in user or the empty string
// if the user is not logged in.  No authorization check is performed.
func (h *AuthHelper) GetUser(ctx context.Context) string {
	session := login.GetSession(ctx)
	if session == nil {
		return ""
	}
	return session.Email
}

// GetViewer returns the email address of the logged-in user or an error if the
// user is not in the set of allowed viewers. Do not wrap the returned error, as
// it is used by Twirp.
func (h *AuthHelper) GetViewer(ctx context.Context) (string, error) {
	email := h.GetUser(ctx)
	if h.viewers.Member(email) {
		return email, nil
	}
	return "", twirp.NewError(twirp.PermissionDenied, fmt.Sprintf("%q is not an authorized viewer", email))
}

// GetEditor returns the email address of the logged-in user or an error if the
// user is not in the set of allowed editors. Do not wrap the returned error, as
// it is used by Twirp.
func (h *AuthHelper) GetEditor(ctx context.Context) (string, error) {
	email := h.GetUser(ctx)
	if h.editors.Member(email) {
		return email, nil
	}
	return "", twirp.NewError(twirp.PermissionDenied, fmt.Sprintf("%q is not an authorized editor", email))
}

// GetAdmin returns the email address of the logged-in user or an error if the
// user is not in the set of allowed admins. Do not wrap the returned error, as
// it is used by Twirp.
func (h *AuthHelper) GetAdmin(ctx context.Context) (string, error) {
	email := h.GetUser(ctx)
	if h.admins.Member(email) {
		return email, nil
	}
	return "", twirp.NewError(twirp.PermissionDenied, fmt.Sprintf("%q is not an authorized admin", email))
}

// Middleware wraps the given http.Handler with the login middleware required in
// order to use AuthHelper.
func Middleware(srv http.Handler) http.Handler {
	return login.SessionMiddleware(srv)
}
