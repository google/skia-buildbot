package common

import (
	"context"
	"net/http"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2/google"
)

const (
	// Content type header application/json.
	ContentType = "application/json"
)

// DefaultHttpClient returns a HTTP client handler configured w/ default
// https://www.googleapis.com/auth/userinfo.email scope.
func DefaultHttpClient(ctx context.Context) (*http.Client, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create pinpoint client.")
	}

	return httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client(), nil
}
