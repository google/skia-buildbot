// package mockedauth is intended for use with local development use cases. It tells auth-proxy
// to always set the same user identity when passing authentication information to
// the proxied service.
package mockedauth

import (
	"context"
	"net/http"

	"go.skia.org/infra/kube/go/authproxy/auth"
)

type mockedAuth struct {
	loggedInAs string
}

// New returns a new auth.Auth instance which always returns loggedInAs from calls to
// [auth.LoggedInAs].
func New(loggedInAs string) mockedAuth {
	return mockedAuth{loggedInAs: loggedInAs}
}

func (m mockedAuth) Init(ctx context.Context) error                         { return nil }
func (m mockedAuth) LoggedInAs(r *http.Request) string                      { return m.loggedInAs }
func (m mockedAuth) LoginURL(w http.ResponseWriter, r *http.Request) string { return "" }

// Confirm mockedAuth implements [auth.Auth].
var _ auth.Auth = mockedAuth{}
