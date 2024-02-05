// Package build_chrome builds Chrome browser given a chromium
// commit and a device target.
//
// build_chrome also supports gerrit patches and non-chromium
// commits.
package build_chrome

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"golang.org/x/oauth2/google"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

// BuildChromeClient is a buildbucket client to build Chrome.
type BuildChromeClient interface {
	// SearchOrBuild starts a new Build if it doesn't exist, or it will fetch
	// the existing one that matches the build parameters.
	SearchOrBuild(ctx context.Context, pinpointJobID, commit, device string, deps map[string]interface{}, patches []*buildbucketpb.GerritChange) (int64, error)

	// GetStatus returns the Build status.
	GetStatus(context.Context, int64) (buildbucketpb.Status, error)

	// RetrieveCAS retrieves CAS from the build.
	RetrieveCAS(context.Context, int64, string) (*swarmingV1.SwarmingRpcsCASReference, error)

	// CancelBuild cancels the ongoing build.
	CancelBuild(context.Context, int64, string) error
}

// These constants define default fields used in a buildbucket
// ScheduleBuildRequest for Pinpoint Chrome builds
const (
	ScheduleReqClobber string = "clobber"
	ScheduleReqDeps    string = "deps_revision_overrides"
	ScheduleReqGit     string = "git_repo"
	ScheduleReqRev     string = "revision"
	ScheduleReqStage   string = "staging"
)

// buildChromeImpl implements BuildChromeClient to build Chrome.
type buildChromeImpl struct {
	backends.BuildbucketClient
}

// New returns buildChromeImpl.
//
// buildChromeImpl is an authenticated LUCI Buildbucket client instance. Although skia has their
// own buildbucket wrapper type, it cannot build Chrome at a specific commit.
func New(ctx context.Context) (*buildChromeImpl, error) {
	// Create authenticated HTTP client.
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, fmt.Errorf("Problem setting up default token source: %s", err)
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).With2xxOnly().Client()

	bc := backends.DefaultClientConfig().WithClient(c)
	return &buildChromeImpl{
		BuildbucketClient: bc,
	}, nil
}

// searchBuild looks for an existing buildbucket build using the
// builder and the commit and returns the build ID and status of the build
// TODO(b/315215756): add support for non-chromium commits. A non-chromium build, such
// as this one: https://ci.chromium.org/ui/p/chrome/builders/try/Android%20arm64%20Compile%20Perf/117084/overview,
// is not easily searchable with the existing buildbucket API. The key commit information
// is written in deps_revision_overrides under Input properties. A working solution would
// be to fetch all the builds under the chromium buildset and hash, and then iterate
// through each build for the correct deps_revision_overrides. A better solution
// would be to add the non-chromium commit info to the tags and query the tags.
func (bci *buildChromeImpl) searchBuild(ctx context.Context, builder, commit string, deps map[string]interface{}, patches []*buildbucketpb.GerritChange) (int64, error) {
	// search Pinpoint for build
	build, err := bci.GetSingleBuild(ctx, builder, backends.DefaultBucket, commit, deps, patches)
	if err != nil {
		return 0, skerr.Wrapf(err, "Error searching buildbucket")
	}

	if build != nil {
		return build.Id, nil
	}

	// Search waterfall for build if there is an appropriate waterfall
	// builder and no gerrit patches. We search waterfall after Pinpoint,
	// because waterfall builders lag behind main. A user could try to
	// request a build via Pinpoint before waterfall has the chance to
	// build the same commit.
	sklog.Debugf("SearchBuild: search waterfall builder %s for build", backends.PinpointWaterfall[builder])
	build, err = bci.GetBuildFromWaterfall(ctx, builder, commit)
	if err != nil {
		return 0, skerr.Wrapf(err, "Failed to find build with CI equivalent.")
	}

	if len(build.GetInput().GetGerritChanges()) > 0 {
		return 0, nil
	}

	if build != nil {
		sklog.Debugf("SearchBuild: build %d found in Waterfall builder", build.Id)
		return build.Id, nil
	}

	sklog.Debug("SearchBuild: build could not be found")
	return 0, nil
}

// SearchOrBuild implements BuildChromeClient interface
func (bci *buildChromeImpl) SearchOrBuild(ctx context.Context, pinpointJobID, commit, device string, deps map[string]interface{}, patches []*buildbucketpb.GerritChange) (int64, error) {
	builder, err := bot_configs.GetBotConfig(device, false)
	if err != nil {
		return 0, err
	}

	buildId, err := bci.searchBuild(ctx, builder.Builder, commit, deps, patches)
	// We can ignore the error here since we only need to know if there is an existing build.
	if err == nil && buildId != 0 {
		return buildId, nil
	}

	// if the ongoing build failed or the build was not found, start new build
	requestID := uuid.New().String()
	build, err := bci.StartChromeBuild(ctx, pinpointJobID, requestID, builder.Builder, commit, deps, patches)
	if err != nil {
		return 0, skerr.Wrapf(err, "Failed to start a build")
	}

	return build.Id, nil
}

// RetrieveCAS implements BuildChromeClient interface
func (bci *buildChromeImpl) RetrieveCAS(ctx context.Context, buildID int64, target string) (*swarmingV1.SwarmingRpcsCASReference, error) {
	ref, err := bci.GetCASReference(ctx, buildID, target)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not find the CAS outputs to build %d", buildID)
	}
	return ref, nil
}

// CancelBuild implements BuildChromeClient interface
func (bci *buildChromeImpl) CancelBuild(ctx context.Context, buildID int64, summary string) error {
	return bci.CancelBuild(ctx, buildID, summary)
}

// GetStatus implements BuildChromeClient interface
// TODO(b/315215756): switch from polling to pub sub
func (bci *buildChromeImpl) GetStatus(ctx context.Context, buildID int64) (buildbucketpb.Status, error) {
	return bci.GetBuildStatus(ctx, buildID)
}
