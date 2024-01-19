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

	"go.skia.org/infra/bisection/go/backends"
	"go.skia.org/infra/bisection/go/bot_configs"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

// SwarmingBuildChrome is a swarming task that builds Chrome.
type SwarmingBuildChrome interface {
	// SearchOrBuild starts a new Build if it doesn't exist, or it will fetch
	// the existing one that matches the build parameters.
	//
	// Note even if the build exists, it doesn't mean it is completed.
	SearchOrBuild(ctx context.Context) (int64, error)

	// GetStatus returns the Build status.
	GetStatus(context.Context) (buildbucketpb.Status, error)

	// RetrieveCAS retrieves CAS from the build.
	RetrieveCAS(context.Context) (*swarmingV1.SwarmingRpcsCASReference, error)
}

// BuildChrome stores all of the parameters
// used by the build Chrome workflow
type BuildChrome struct {
	// Client is the buildbucket client.
	Client backends.Buildbucket
	// Commit is the chromium commit hash.
	Commit string
	// Device is the name of the device, e.g. "linux-perf".
	Device string
	// The corresponding Chrome builder associated with
	// the device. Can get the builder from package
	// bot_configs.
	Builder string
	// Target is name of the build isolate target
	// e.g. "performance_test_suite".
	Target string
	// Patch is the Gerrit patch included in the build.
	Patch []*buildbucketpb.GerritChange
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

// TODO(haowoo):
// New should return SwarmingBuildChrome however that requires a larger change,
// it is done so only to break up into small changes.
func New(client backends.Buildbucket, commit, device, builder, target string, patch []*buildbucketpb.GerritChange) (*BuildChrome, error) {
	if client == nil {
		buildClient, err := dialBuildbucketClient(context.Background())
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to create build client.")
		}

		client = *buildClient
	}

	// BuildChrome has not implemented SwarmingBuildChrome yet.
	return &BuildChrome{
		Client:  client,
		Commit:  commit,
		Device:  device,
		Builder: builder,
		Target:  target,
		Patch:   patch,
	}, nil
}

// dialBuildbucketClient returns an authenticated LUCI Buildbucket client instance.
//
// Although skia has their own buildbucket wrapper type, it cannot build Chrome
// at a specific commit.
func dialBuildbucketClient(ctx context.Context) (*backends.BuildbucketClient, error) {
	// Create authenticated HTTP client.
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, fmt.Errorf("Problem setting up default token source: %s", err)
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).With2xxOnly().Client()

	bc := backends.DefaultClientConfig().WithClient(c)
	return bc, nil
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
func (b *BuildChrome) searchBuild(ctx context.Context, builder string) (int64, error) {
	// search Pinpoint for build
	build, err := b.Client.GetBuildWithPatches(ctx, builder, backends.DefaultBucket, b.Commit, b.Patch)
	if err != nil {
		return 0, skerr.Wrapf(err, "Error searching buildbucket.")
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
	build, err = b.Client.GetBuildFromWaterfall(ctx, builder, b.Commit)
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

// Run searches buildbucket for an old Chrome build and will schedule a new
// Chrome build if it cannot find one.
// Run returns the build ID of an existing, ongoing, or new build.
func (b *BuildChrome) Run(ctx context.Context, pinpointJobID string) (int64, error) {
	builder, err := bot_configs.GetBotConfig(b.Device, false)
	if err != nil {
		return 0, err
	}

	buildId, err := b.searchBuild(ctx, builder.Builder)
	if err != nil {
		return 0, err
	}
	// found a build
	if buildId != 0 {
		return buildId, nil
	}

	// if the ongoing build failed or the build was not found, start new build
	requestID := uuid.New().String()
	build, err := b.Client.StartChromeBuild(ctx, pinpointJobID, requestID, builder.Builder, b.Commit, b.Patch)
	if err != nil {
		return 0, err
	}

	return build.Id, nil
}

// CheckBuildStatus returns the builds status given a buildId
// TODO(b/315215756): switch from polling to pub sub
func (b *BuildChrome) CheckBuildStatus(ctx context.Context, buildID int64) (buildbucketpb.Status, error) {
	status, err := b.Client.GetBuildStatus(ctx, buildID)
	if err != nil {
		return buildbucketpb.Status_STATUS_UNSPECIFIED, err

	}

	return status, nil
}

// RetrieveCAS returns the CAS location of the build given buildId
func (b *BuildChrome) RetrieveCAS(ctx context.Context, buildID int64) (*swarmingV1.SwarmingRpcsCASReference, error) {
	ref, err := b.Client.GetCASReference(ctx, buildID, b.Target)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not find the CAS outputs to build %d", buildID)
	}
	return ref, nil
}

func (b *BuildChrome) Cancel(ctx context.Context, buildID int64, reason string) error {
	err := b.Client.CancelBuild(ctx, buildID, reason)
	if err != nil {
		return err
	}
	return nil
}
