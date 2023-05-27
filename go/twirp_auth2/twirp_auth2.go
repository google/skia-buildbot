package twirp_auth2

import (
	"context"
	"fmt"

	"github.com/twitchtv/twirp"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/roles"
)

/*
	Helpers for working with Twirp.
*/

// AuthHelper provides methods for authenticating users.
type AuthHelper struct {
}

// New returns an AuthHelper instance.
func New() *AuthHelper {
	return &AuthHelper{}
}

// GetViewer returns the email address of the logged-in user or an error if the
// user is not in the set of allowed viewers. Do not wrap the returned error, as
// it is used by Twirp.
func (h *AuthHelper) GetViewer(ctx context.Context) (string, error) {
	status := alogin.GetStatus(ctx)

	if (status.Roles.Has(roles.Viewer) || status.Roles.Has(roles.Editor)) && (status.EMail != alogin.NotLoggedIn) {
		return status.EMail.String(), nil
	}
	return "", twirp.NewError(twirp.PermissionDenied, fmt.Sprintf("%q is not an authorized viewer", status.EMail))
}

// GetEditor returns the email address of the logged-in user or an error if the
// user is not in the set of allowed editors. Do not wrap the returned error, as
// it is used by Twirp.
func (h *AuthHelper) GetEditor(ctx context.Context) (string, error) {
	status := alogin.GetStatus(ctx)

	if status.Roles.Has(roles.Editor) && (status.EMail != alogin.NotLoggedIn) {
		return status.EMail.String(), nil
	}
	return "", twirp.NewError(twirp.PermissionDenied, fmt.Sprintf("%q is not an authorized editor", status.EMail))
}
