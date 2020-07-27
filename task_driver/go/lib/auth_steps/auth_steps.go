package auth_steps

/*
   This package provides auth initialization. It is a wrapper around the
   go.skia.org/infra/go/auth package.
*/

import (
	"context"
	"net/http"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/luciauth"
	"go.skia.org/infra/task_driver/go/td"
	"golang.org/x/oauth2"
)

func Init(ctx context.Context, local bool, scopes ...string) (oauth2.TokenSource, error) {
	var ts oauth2.TokenSource
	err := td.Do(ctx, td.Props("Auth Init").Infra(), func(context.Context) error {
		var err error
		if local {
			ts, err = auth.NewDefaultTokenSource(true, scopes...)
		} else {
			ts, err = luciauth.NewLUCIContextTokenSource(scopes...)
		}
		return err
	})
	return ts, err
}

func HttpClient(ctx context.Context, ts oauth2.TokenSource) *http.Client {
	return td.HttpClient(ctx, httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client())
}

func InitHttpClient(ctx context.Context, local bool, scopes ...string) (*http.Client, error) {
	ts, err := Init(ctx, local, scopes...)
	if err != nil {
		return nil, err
	}
	return HttpClient(ctx, ts), nil
}
