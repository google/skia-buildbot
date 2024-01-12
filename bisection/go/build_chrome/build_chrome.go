// Package build_chrome builds Chrome browser given a chromium
// commit and a device target.
//
// build_chrome also supports gerrit patches and non-chromium
// commits.
package build_chrome

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.chromium.org/luci/grpc/prpc"
	"go.skia.org/infra/bisection/go/bot_configs"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

// BuildChrome stores all of the parameters
// used by the build Chrome workflow
type BuildChrome struct {
	// Client is the buildbucket client.
	Client buildbucketpb.BuildsClient
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
var PinpointWaterfall = map[string]string{
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

// name of the bucket used by Pinpoint builders in LUCI
const pinpointBucket = "try"

// chromium gitiles links
const (
	chromiumGitilesUrl     = "https://chromium.googlesource.com/chromium/src"
	chromiumGitilesHost    = "chromium.googlesource.com"
	chromiumGitilesProject = "chromium/src"
	chromiumGitilesRef     = "refs/heads/main"
)

// The swarming instance the build CAS isolate belongs to
// TODO(b/315215756): Support other swarming instances. There are three known
// swarming instances Pinpoint supports. The majority of Pinpoint builds are
// this defaultInstance. Buildbucket API does not report the swarming instance
// so our options are to:
// - include the expected instance in the build tags
// - try all 3 known swarming instances and brute force it
const defaultInstance string = "projects/chrome-swarming/instances/default_instance"

// RBE CAS isolates expire after 32 days. We use 30 out of caution.
const casExpiration int = 30

// These constants define default fields used in a buildbucket
// ScheduleBuildRequest for Pinpoint Chrome builds
const (
	ScheduleReqClobber string = "clobber"
	ScheduleReqDeps    string = "deps_revision_overrides"
	ScheduleReqGit     string = "git_repo"
	ScheduleReqRev     string = "revision"
	ScheduleReqStage   string = "staging"
)

// DialBuildClient returns an authenticated LUCI Buildbucket client instance.
//
// Although skia has their own buildbucket wrapper type, it cannot build Chrome
// at a specific commit.
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

func (b *BuildChrome) createSearchBuildRequest(bucket string, builder string) *buildbucketpb.SearchBuildsRequest {
	tags := []*buildbucketpb.StringPair{
		{
			// buildset is how commit information is tracked in waterfall and Pinpoint.
			Key:   "buildset",
			Value: fmt.Sprintf("commit/gitiles/chromium.googlesource.com/chromium/src/+/%s", b.Commit),
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
	if b.Patch != nil {
		req.Predicate.GerritChanges = b.Patch
	}
	return req
}

func (b *BuildChrome) search(ctx context.Context, req *buildbucketpb.SearchBuildsRequest) (int64, error) {
	resp, err := b.Client.SearchBuilds(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("error searching buildbucket builds with request %v\n and error: %s", req, err)
	}
	if resp == nil {
		return 0, nil
	}

	for _, build := range resp.Builds {
		// builds are returned from newest to oldest
		// if a completed build is past the expiration point,
		// then all remaining builds are too old
		// incomplete builds have default endtime of 1970-01-01 00:00 UTC
		if build.Status.Number() > buildbucketpb.Status_ENDED_MASK.Number() &&
			time.Now().Sub(build.EndTime.AsTime()).Hours()/24 > float64(casExpiration) {
			return 0, nil
		}
		// verify successful or ongoing build with correct gerrit changes
		// client.SearchBuilds will find all builds that include the GerritChanges
		// rather than matching builds that have the exact same GerritChanges
		// so we need to verify there are an equal number of gerrit patches
		if (build.Status == buildbucketpb.Status_SUCCESS ||
			build.Status == buildbucketpb.Status_STARTED ||
			build.Status == buildbucketpb.Status_SCHEDULED) &&
			len(req.Predicate.GerritChanges) == len(build.Input.GerritChanges) {
			return build.Id, nil
		}
	}
	return 0, nil
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
	req := &buildbucketpb.SearchBuildsRequest{}

	// search Pinpoint for build
	req = b.createSearchBuildRequest(pinpointBucket, builder)
	sklog.Debugf("SearchBuild: request %v", req)
	buildId, err := b.search(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("error searching buildbucket builds with request %v\n and error: %s", req, err)
	}
	if buildId != 0 {
		sklog.Debugf("SearchBuild: build %d found in Pinpoint builder\n", buildId)
		return buildId, nil
	}

	// Search waterfall for build if there is an appropriate waterfall
	// builder and no gerrit patches. We search waterfall after pinpoint,
	// because waterfall builders lag behind main. A user could try to
	// request a build via Pinpoint before waterfall has the chance to
	// build the same commit.
	if PinpointWaterfall[builder] != "" && len(b.Patch) == 0 {
		sklog.Debugf("SearchBuild: search waterfall builder %s for build", PinpointWaterfall[builder])

		req = b.createSearchBuildRequest("ci", PinpointWaterfall[builder])
		buildId, err := b.search(ctx, req)

		if err != nil {
			return 0, skerr.Fmt("error searching buildbucket builds with request %v\n and error: %s", req, err)
		}
		if buildId != 0 {
			sklog.Debugf("SearchBuild: build %d found in Waterfall builder", buildId)
			return buildId, nil
		}
	}

	sklog.Debug("SearchBuild: build could not be found")
	return 0, nil
}

// TODO(b/315215756): Add support for non-chromium commits and gerrit patches
func (b *BuildChrome) createScheduleRequest(builder string) (
	*buildbucketpb.ScheduleBuildRequest, error) {
	req := &buildbucketpb.ScheduleBuildRequest{
		RequestId: uuid.New().String(),
		Builder: &buildbucketpb.BuilderID{
			Project: "chrome",
			Bucket:  pinpointBucket,
			Builder: builder,
		},
		Properties: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				ScheduleReqClobber: {
					Kind: &structpb.Value_BoolValue{
						BoolValue: false,
					},
				},
				ScheduleReqGit: {
					Kind: &structpb.Value_StringValue{
						StringValue: chromiumGitilesUrl,
					},
				},
				ScheduleReqRev: {
					Kind: &structpb.Value_StringValue{
						StringValue: b.Commit,
					},
				},
				ScheduleReqStage: {
					Kind: &structpb.Value_BoolValue{
						BoolValue: false,
					},
				},
			},
		},
		GitilesCommit: &buildbucketpb.GitilesCommit{
			Host:    chromiumGitilesHost,
			Project: chromiumGitilesProject,
			Id:      b.Commit,
			Ref:     chromiumGitilesRef,
		},
		GerritChanges: b.Patch,
		// TODO(b/315215756): Implement createTags function to
		// generalize across different job types
		Tags: []*buildbucketpb.StringPair{
			{
				Key:   "pinpoint_job_id",
				Value: "1",
			},
			{
				Key:   "skia_pinpoint_bisect",
				Value: "true",
			},
			{
				Key:   "buildset",
				Value: fmt.Sprintf("commit/gitiles/chromium.googlesource.com/chromium/src/+/%s", b.Commit),
			},
		},
	}

	return req, nil
}

// TODO(b/315215756): implement cancel build

// Run searches buildbucket for an old Chrome build
// and will schedule a new Chrome build if it cannot
// find one.
// Run returns the build ID of an existing, ongoing,
// or new build.
func (b *BuildChrome) Run(ctx context.Context) (
	int64, error) {

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
	newBuildReq, err := b.createScheduleRequest(builder.Builder)
	if err != nil {
		return 0, err
	}

	build, err := b.Client.ScheduleBuild(ctx, newBuildReq)
	if err != nil {
		return 0, err
	}
	return build.Id, nil
}

// CheckBuildStatus returns the builds status given a buildId
// TODO(b/315215756): switch from polling to pub sub
func (b *BuildChrome) CheckBuildStatus(ctx context.Context, buildId int64) (buildbucketpb.Status, error) {
	statusReq := &buildbucketpb.GetBuildStatusRequest{
		Id: buildId,
	}
	build, err := b.Client.GetBuildStatus(ctx, statusReq)
	if err != nil {
		return buildbucketpb.Status_STATUS_UNSPECIFIED, err
	}

	return build.Status, nil
}

// RetrieveCAS returns the CAS location of the build given buildId
func (b *BuildChrome) RetrieveCas(ctx context.Context, buildId int64) (
	*swarmingV1.SwarmingRpcsCASReference, error) {
	req := buildbucketpb.GetBuildRequest{
		Id: buildId,
		Mask: &buildbucketpb.BuildMask{
			Fields: &fieldmaskpb.FieldMask{
				Paths: []string{"output.properties"},
			},
		},
	}
	build, err := b.Client.GetBuild(ctx, &req)
	if err != nil {
		return nil, err
	}
	if build.Status != buildbucketpb.Status_SUCCESS {
		return nil, skerr.Fmt("Cannot retrieve CAS from build %d with status %v",
			buildId, build.Status)
	}
	for k, v := range build.Output.Properties.Fields {
		if strings.Contains(k, "swarm_hashes_refs") {
			cas := fmt.Sprintf("%v", v.GetStructValue().AsMap()[b.Target])
			bytes, err := strconv.ParseInt(strings.Split(cas, "/")[1], 10, 64)
			if err != nil {
				return nil, err
			}
			return &swarmingV1.SwarmingRpcsCASReference{
				CasInstance: defaultInstance,
				Digest: &swarmingV1.SwarmingRpcsDigest{
					Hash:      strings.Split(cas, "/")[0],
					SizeBytes: bytes,
				},
			}, nil
		}
	}
	return nil, skerr.Fmt("Could not find the CAS outputs to build %d", buildId)
}
