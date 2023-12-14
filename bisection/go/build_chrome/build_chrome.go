// Package build_chrome builds Chrome browser given a chromium
// commit and a device target.
//
// build_chrome also supports gerrit patches and non-chromium
// commits.
package build_chrome

import (
	"context"
	"fmt"

	"go.chromium.org/luci/grpc/prpc"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
)

// pinpointWaterfall maps builder names from Pinpoint to Waterfall
// Builds from waterfall can also be recycled for bisection
//
// As part of our anomaly detection system, Waterfall builders
// will continuously build Chrome near the tip of main. Sometimes,
// Pinpoint jobs will attempt to build a CL Waterfall has already
// built i.e. verifying a regression. These builds are automatic.
// Pinpoint builders will only build Chrome on demand. The Waterfall
// and Pinpoint builders are maintained in separate pools.
//
// The map is maintained here:
// https://chromium.googlesource.com/chromium/tools/build/+/986f23767a01508ad1eb39194ffdb5fec4f00d7b/recipes/recipes/pinpoint/builder.py#22
// TODO(b/316207255): move this builder map to a more stable config file
var pinpointWaterfall = map[string]string{
	"Android Compile Perf":                       "android-builder-perf",
	"Android Compile Perf PGO":                   "android-builder-perf-pgo",
	"Android arm64 Compile Perf":                 "android_arm64-builder-perf",
	"Android arm64 Compile Perf PGO":             "android_arm64-builder-perf-pgo",
	"Android arm64 High End Compile Perf":        "android_arm64_high_end-builder-perf",
	"Android arm64 High End Compile Perf PGO":    "android_arm64_high_end-builder-perf-pgo",
	"Chromecast Linux Builder Perf":              "chromecast-linux-builder-perf",
	"Chromeos Amd64 Generic Lacros Builder Perf": "chromeos-amd64-generic-lacros-builder-perf",
	"Fuchsia Builder Perf":                       "fuchsia-builder-perf-arm64",
	"Linux Builder Perf":                         "linux-builder-perf",
	"Linux Builder Perf PGO":                     "linux-builder-perf-pgo",
	"Mac Builder Perf":                           "mac-builder-perf",
	"Mac Builder Perf PGO":                       "mac-builder-perf-pgo",
	"Mac arm Builder Perf":                       "mac-arm-builder-perf",
	"Mac arm Builder Perf PGO":                   "mac-arm-builder-perf-pgo",
	"mac-laptop_high_end-perf":                   "mac-laptop_high_end-perf",
	"Win x64 Builder Perf":                       "win64-builder-perf",
	"Win x64 Builder Perf PGO":                   "win64-builder-perf-pgo",
}

// DialBuildClient returns an authenticated LUCI Buildbucket client instance.
//
// Although skia has their own buildbucket wrapper type, it cannot build Chrome
// at a specific commit.
// TODO(b/315215756): Move this client dial to a backends/ folder
func DialBuildClient(ctx context.Context) (buildbucketpb.BuildsClient, error) {
	// Create authenticated HTTP client.
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, fmt.Errorf("Problem setting up default token source: %s", err)
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).With2xxOnly().Client()

	host := buildbucket.DEFAULT_HOST
	return buildbucketpb.NewBuildsPRPCClient(&prpc.Client{
		C:    c,
		Host: host,
	}), nil
}

func createSearchBuildRequest(bucket string, builder string, commit string,
	patch []*buildbucketpb.GerritChange) *buildbucketpb.SearchBuildsRequest {
	tags := []*buildbucketpb.StringPair{
		{
			// buildset is how commit information is tracked in waterfall and Pinpoint.
			Key:   "buildset",
			Value: fmt.Sprintf("commit/gitiles/chromium.googlesource.com/chromium/src/+/%s", commit),
		},
	}
	req := &buildbucketpb.SearchBuildsRequest{
		Predicate: &buildbucketpb.BuildPredicate{
			Builder: &buildbucketpb.BuilderID{
				Project: "chrome",
				Bucket:  bucket,
				Builder: builder,
			},
			Tags: tags,
		},
	}
	// patch is always nil for waterfall builds
	if patch != nil {
		req.Predicate.GerritChanges = patch
	}
	return req
}

func search(ctx context.Context, client buildbucketpb.BuildsClient,
	req *buildbucketpb.SearchBuildsRequest) (int64, buildbucketpb.Status, error) {
	resp, err := client.SearchBuilds(ctx, req)
	if err != nil {
		return 0, 0, fmt.Errorf("error searching buildbucket builds with request %v\n and error: %s", req, err)
	}
	// builds are returned from newest to oldest
	for _, build := range resp.Builds {
		// client.SearchBuilds will find all builds that include the GerritChanges
		// rather than matching builds that have the exact same GerritChanges
		// so we need to verify there are an equal number of gerrit patches
		if len(req.Predicate.GerritChanges) == len(build.Input.GerritChanges) {
			return build.Id, build.Status, nil
		}
	}
	return 0, 0, nil
}

// SearchBuild looks for an existing buildbucket build using the
// builder and the commit and returns the build ID and status of the build
// TODO(b/315215756): add support for non-chromium commits. A non-chromium build, such
// as this one: https://ci.chromium.org/ui/p/chrome/builders/try/Android%20arm64%20Compile%20Perf/117084/overview,
// is not easily searchable with the existing buildbucket API. The key commit information
// is written in deps_revision_overrides under Input properties. A working solution would
// be to fetch all the builds under the chromium buildset and hash, and then iterate
// through each build for the correct deps_revision_overrides. A better solution
// would be to add the non-chromium commit info to the tags and query the tags.
func SearchBuild(ctx context.Context, client buildbucketpb.BuildsClient, builder string,
	commit string, patch []*buildbucketpb.GerritChange) (int64, buildbucketpb.Status, error) {
	req := &buildbucketpb.SearchBuildsRequest{}

	// search Pinpoint for build
	req = createSearchBuildRequest("try", builder, commit, patch)
	buildID, status, err := search(ctx, client, req)
	if err != nil {
		return 0, 0, fmt.Errorf("error searching buildbucket builds with request %v\n and error: %s", req, err)
	}
	if buildID != 0 {
		sklog.Debugf("build %d found in Pinpoint builder with status %v\n", buildID, status)
		return buildID, status, nil
	}

	// Search waterfall for build if there is an appropriate waterfall
	// builder and no gerrit patches. We search waterfall after pinpoint,
	// because waterfall builders lag behind main. A user could try to
	// request a build via Pinpoint before waterfall has the chance to
	// build the same commit.
	if pinpointWaterfall[builder] != "" && len(patch) == 0 {
		sklog.Debugf("search waterfall builder %s for build", pinpointWaterfall[builder])
		req = createSearchBuildRequest("ci", pinpointWaterfall[builder], commit, nil)
		buildID, status, err := search(ctx, client, req)
		if err != nil {
			return 0, 0, fmt.Errorf("error searching buildbucket builds with request %v\n and error: %s", req, err)
		}
		if buildID != 0 {
			sklog.Debugf("build %d found in Waterfall builder with status %v", buildID, status)
			return buildID, status, nil
		}
	}

	sklog.Debug("build could not be found")
	return 0, 0, nil
}
