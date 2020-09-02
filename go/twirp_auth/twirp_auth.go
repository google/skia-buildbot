package twirp_auth

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
	getUser func(context.Context) string
}

// NewAuthHelper returns an AuthHelper instance which uses the given allow lists
// to control access.  If the viewers list is nil, anyone is allowed to view.
// If the editors or admins lists are nil, nobody has permission for those
// actions.
func NewAuthHelper(viewers, editors, admins allowed.Allow) *AuthHelper {
	return &AuthHelper{
		viewers: viewers,
		editors: editors,
		admins:  admins,
		getUser: func(ctx context.Context) string {
			session := login.GetSession(ctx)
			if session == nil {
				return ""
			}
			return session.Email
		},
	}
}

// GetViewer returns the email address of the logged-in user or an error if the
// user is not in the set of allowed viewers. Do not wrap the returned error, as
// it is used by Twirp.
func (h *AuthHelper) GetViewer(ctx context.Context) (string, error) {
	email := h.getUser(ctx)
	// Special case for viewers: if nil, anyone can view.
	if h.viewers == nil || h.viewers.Member(email) {
		return email, nil
	}
	return "", twirp.NewError(twirp.PermissionDenied, fmt.Sprintf("%q is not an authorized viewer", email))
}

// GetEditor returns the email address of the logged-in user or an error if the
// user is not in the set of allowed editors. Do not wrap the returned error, as
// it is used by Twirp.
func (h *AuthHelper) GetEditor(ctx context.Context) (string, error) {
	email := h.getUser(ctx)
	if h.editors != nil && h.editors.Member(email) {
		return email, nil
	}
	return "", twirp.NewError(twirp.PermissionDenied, fmt.Sprintf("%q is not an authorized editor", email))
}

// GetAdmin returns the email address of the logged-in user or an error if the
// user is not in the set of allowed admins. Do not wrap the returned error, as
// it is used by Twirp.
func (h *AuthHelper) GetAdmin(ctx context.Context) (string, error) {
	email := h.getUser(ctx)
	if h.admins != nil && h.admins.Member(email) {
		return email, nil
	}
	return "", twirp.NewError(twirp.PermissionDenied, fmt.Sprintf("%q is not an authorized admin", email))
}

// MockGetUserForTesting sets the function used to retrieve the logged-in user
// from the request context. This is intended to be used for testing only.
func (h *AuthHelper) MockGetUserForTesting(getUser func(context.Context) string) {
	h.getUser = getUser
}

// Middleware wraps the given http.Handler with the login middleware required in
// order to use AuthHelper.
func Middleware(srv http.Handler) http.Handler {
	return login.SessionMiddleware(srv)
}
