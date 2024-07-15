package internal

import (
	"context"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/midpoint"

	"golang.org/x/oauth2/google"
)

type MidpointHandlerKey struct{}

var MidpointHandlerContextKey = &MidpointHandlerKey{}

// FindMidCommitActivity is an Activity that finds the middle point of two commits.
func FindMidCommitActivity(ctx context.Context, lower, higher *common.CombinedCommit) (*common.CombinedCommit, error) {
	handler, ok := ctx.Value(MidpointHandlerContextKey).(*midpoint.MidpointHandler)
	if !ok {
		httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
		if err != nil {
			return nil, skerr.Wrapf(err, "problem setting up default token source")
		}
		c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).Client()
		handler = midpoint.New(ctx, c)
	}

	m, err := handler.FindMidCombinedCommit(ctx, lower, higher)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to identify midpoint")
	}
	return m, nil
}

// CheckCombinedCommitEqualActivity checks whether two combined commits are equal.
func CheckCombinedCommitEqualActivity(ctx context.Context, lower, higher *common.CombinedCommit) (bool, error) {
	handler, ok := ctx.Value(MidpointHandlerContextKey).(*midpoint.MidpointHandler)
	if !ok {
		httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
		if err != nil {
			return false, skerr.Wrapf(err, "Problem setting up default token source")
		}
		c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).Client()
		handler = midpoint.New(ctx, c)
	}
	equal, err := handler.Equal(ctx, lower, higher)
	if err != nil {
		return equal, skerr.Wrapf(err, "failed to determine combined commit equality")
	}

	return equal, nil
}
