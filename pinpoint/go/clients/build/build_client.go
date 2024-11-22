package build

import (
	"context"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/workflows"
	"golang.org/x/oauth2/google"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
)

type BuildClientKey struct{}

var BuildClientContextKey = &BuildClientKey{}

// BuildClient is the interface for all build clients to implement.
type BuildClient interface {
	// CreateFindBuildRequest returns a request for FindBuild.
	CreateFindBuildRequest(params workflows.BuildParams) (*FindBuildRequest, error)

	// FindBuild looks for an existing build that matches the build parameters.
	FindBuild(ctx context.Context, req *FindBuildRequest) (*FindBuildResponse, error)

	// CreateStartBuildRequest returns a request for StartBuild
	CreateStartBuildRequest(params workflows.BuildParams) (*StartBuildRequest, error)

	// FindBuild starts a new build request.
	StartBuild(ctx context.Context, req *StartBuildRequest) (*StartBuildResponse, error)

	// GetStatus returns the Build status.
	//
	// Note: The status should be generalized, but the Buildbucket statuses do a
	// good job of defining states, so we'll leave it for now.
	GetStatus(ctx context.Context, id int64) (buildbucketpb.Status, error)

	// GetBuildArtifact fetches the information pointing to build artifacts.
	GetBuildArtifact(ctx context.Context, req *GetBuildArtifactRequest) (*GetBuildArtifactResponse, error)

	// CancelBuild cancels an existing ongoing build.
	CancelBuild(ctx context.Context, req *CancelBuildRequest) error
}

func NewBuildClient(ctx context.Context, project string) (BuildClient, error) {
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "problem setting up default token source")
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).Client()

	// TODO(jeffyoon@) - switch this to a switch statenent and change the type of client
	// being returned based on the project. Default should be build_chrome.
	return newBuildChromeClient(c), nil
}
