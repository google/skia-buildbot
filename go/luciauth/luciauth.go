package luciauth

import (
	"context"

	luci_auth "go.chromium.org/luci/auth"
	"golang.org/x/oauth2"
)

// NewLUCIContextTokenSource creates a new oauth2.TokenSource that uses
// LUCI_CONTEXT to generate tokens. This is the canonical way to obtain tokens
// for a service account tied to a Swarming task, ie. not the default GCE
// service account for a VM, but a service account specified in the task
// request. For more information, see:
// https://github.com/luci/luci-py/blob/master/client/LUCI_CONTEXT.md
//
// Individual scopes need to be specifically allowed by the LUCI token server.
// For this reason, it is recommended to use the compute.CloudPlatform scope.
func NewLUCIContextTokenSource(scopes ...string) (oauth2.TokenSource, error) {
	authenticator := luci_auth.NewAuthenticator(context.Background(), luci_auth.SilentLogin, luci_auth.Options{
		Method: luci_auth.LUCIContextMethod,
		Scopes: append(scopes, luci_auth.OAuthScopeEmail),
	})
	return authenticator.TokenSource()
}
