package auth_steps

/*
   This package provides auth initialization. It is a wrapper around the
   go.skia.org/infra/go/auth package.
*/

import (
	"context"

	"go.skia.org/infra/go/auth"
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
			ts, err = auth.NewLUCIContextTokenSource(scopes...)
		}
		return err
	})
	return ts, err
}
