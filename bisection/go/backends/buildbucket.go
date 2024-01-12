package backends

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"

	"go.chromium.org/luci/common/retry"
	"go.chromium.org/luci/grpc/prpc"

	"go.skia.org/infra/bisection/go/build_chrome"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/skerr"

	bpb "go.chromium.org/luci/buildbucket/proto"
)

const (
	// RBE CAS isolates expire after 32 days. We use 30 out of caution.
	CasExpiration = 30

	// ChromeProject refers to the "chrome" project.
	ChromeProject = "chrome"

	// DefaultBucket is the Pinpoint bucket, equivalent to the "try" builds in Buildbucket.
	DefaultBucket = "try"

	// DefaultPerRPCTimeout defines the default time permitted for each RPC call.
	DefaultPerRPCTimeout = 90 * time.Second

	// DefaultRetries is the default number of retries for Backoff logic to Buildbucket.
	DefaultRetries = 10

	// DefaultTagKey is key tagged on builds for how commit information is tracked in Waterfall (CI) and Pinpoint.
	DefaultTagKey = "buildset"

	// DefaultTagValue is the value format for the key above.
	DefaultTagValue = "commit/gitiles/chromium.googlesource.com/chromium/src/+/%s"

	// WaterfallBucket is equivalent to the "ci" bucket in Buildbucket.
	WaterfallBucket = "ci"
)

type Buildbucket interface {
	// GetBuilds calls Buildbucket to find existing builds for the given
	// builder and Chromium revision.
	GetBuilds(ctx context.Context, builderName, bucket, commit string, patches []*bpb.GerritChange) ([]*bpb.Build, error)

	// GetBuildWithPatches calls Buildbucket to find existing builds for the
	// given builder, Chromium revision and DEPS overrides combination.
	//
	// TODO(b/315215756): The current mechanism can be updated to utilize
	// tags, so that we aren't operating on O(len(builds) * len(deps_overrides))
	// to find the exact builds. This will require tagging scheduled builds with
	// new tags before it can be utilized.
	GetBuildWithPatches(ctx context.Context, builderName, bucket, commit string, patches []*bpb.GerritChange) (*bpb.Build, error)

	// GetBuildFromWaterfall searches for an existing build using its waterfall
	// (CI) counterpart.
	GetBuildFromWaterfall(ctx context.Context, builderName, commit string) (*bpb.Build, error)
}

// BuildbucketClient is an object used to interact with a single Buildbucket instance.
// This extends Skia's Buildbucket wrapper as our single use-case is to create
// builds at specific commits.
type BuildbucketClient struct {
	client bpb.BuildsClient
}

func NewBuildbucketClient(bc bpb.BuildsClient) *BuildbucketClient {
	return &BuildbucketClient{
		client: bc,
	}
}

// createSearchBuildRequest generates a SearchBuildsRequest.
func (b BuildbucketClient) createSearchBuildRequest(builderName, bucket, commit string, patches []*bpb.GerritChange) *bpb.SearchBuildsRequest {
	tags := []*bpb.StringPair{
		{
			Key:   DefaultTagKey,
			Value: fmt.Sprintf(DefaultTagValue, commit),
		},
	}

	// PageSize defaults to 100, with a maximum of 1000 builds.
	req := &bpb.SearchBuildsRequest{
		Predicate: &bpb.BuildPredicate{
			Builder: &bpb.BuilderID{
				Project: ChromeProject,
				Bucket:  bucket,
				Builder: builderName,
			},
			Tags:          tags,
			GerritChanges: patches,
		},
	}

	return req
}

// isBuildTooOld checks whether a terminated build is too old and no longer worth checking.
// Incomplete builds have default endtime of 1970-01-01 00:00 UTC.
func (b BuildbucketClient) isBuildTooOld(build *bpb.Build) bool {
	return (build.Status.Number() > bpb.Status_ENDED_MASK.Number() &&
		time.Now().Sub(build.EndTime.AsTime()).Hours()/24 > float64(CasExpiration))
}

// findMatchingBuild searches the list of builds to find a build in good status (Success, Started, Scheduled)
// with the correct number of patchsets.
func (b BuildbucketClient) findMatchingBuild(builds []*bpb.Build, patches []*bpb.GerritChange) *bpb.Build {
	statusOK := []bpb.Status{
		bpb.Status_SUCCESS,
		bpb.Status_STARTED,
		bpb.Status_SCHEDULED,
	}

	// SearchBuilds returns all builds that contain the GerritChange instead of an exact match,
	// so this logic loops through to ensure we have an identical match.
	// Because of the sorted response (latest -> oldest), this returns the latest matched entry.
	for _, build := range builds {
		// If a completed build is past the expiration point, then all remaining
		// builds are too old, since builds are returned ordered by build number
		// and thus, newest to oldest.
		if b.isBuildTooOld(build) {
			return nil
		}

		if slices.Contains(statusOK, build.GetStatus()) && len(patches) == len(build.GetInput().GetGerritChanges()) {
			return build
		}
	}

	return nil
}

// GetBuilds calls Buildbucket's SearchBuilds.
func (b BuildbucketClient) GetBuilds(ctx context.Context, builderName, bucket, commit string, patches []*bpb.GerritChange) ([]*bpb.Build, error) {
	req := b.createSearchBuildRequest(builderName, bucket, commit, patches)
	resp, err := b.client.SearchBuilds(ctx, req)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to call Buildbucket with request %v: ", req)
	}

	// Note: This assumes that the result we're looking for is within the first
	// 100 builds, since it's ordered from newest to oldest. Utilize NextPageToken
	// from the response to fetch more responses, or increase the PageSize up to
	// 1000.
	return resp.Builds, nil
}

// GetBuildWithPatches utilizes GetBuilds() and filters to find an exactly matching build, meaning
// that the GerritChanges and base Chromium build commit hash are the same.
func (b BuildbucketClient) GetBuildWithPatches(ctx context.Context, builderName, bucket, commit string, patches []*bpb.GerritChange) (*bpb.Build, error) {
	builds, err := b.GetBuilds(ctx, builderName, bucket, commit, patches)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to call Buildbucket to find a single matching build.")
	}

	return b.findMatchingBuild(builds, patches), nil
}

// GetBuildFromWaterfall searches for an exactly matching Buildbucket build using information
// from the builderName's CI counterpart.
func (b BuildbucketClient) GetBuildFromWaterfall(ctx context.Context, builderName, commit string) (*bpb.Build, error) {
	mirror, ok := build_chrome.PinpointWaterfall[builderName]
	if !ok {
		return nil, skerr.Fmt("%s has no supported CI waterfall builder.", builderName)
	}

	builds, err := b.GetBuilds(ctx, mirror, WaterfallBucket, commit, nil)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to find build with using CI counterpart for %s", builderName)
	}

	// We pass an empty list of len == 0 so that Builds with GerritChanges specified
	// are ignored.
	return b.findMatchingBuild(builds, make([]*bpb.GerritChange, 0)), nil
}

// BuildbucketClientConfig represents options for the behavior of the Buildbucket client.
//
// Example:
// bc := DefaultClientConfig().WithClient(c)
// bc.GetBuilds(...)
type BuildbucketClientConfig struct {
	// The buildbucket host to target. See "go.skia.org/infra/go/buildbucket"
	// for the default value.
	Host string

	// Retries, if >= 0, is the number of remaining retries. If <0, no retry
	// count will be applied.
	Retries int

	// Delay is the next generated delay.
	Delay time.Duration

	// MaxDelay is the maximum duration. If <= zero, no maximum will be enforced.
	MaxDelay time.Duration

	// PerRPCTimeout, if > 0, is a timeout that is applied to each call attempt.
	PerRPCTimeout time.Duration
}

// DefaultClientConfig returns a BuildbucketClientConfig with defaults:
//   - Host: cr-buildbucket.appspot.com
//   - Exponential backoff with 10 retries
//   - PerRPCTimeout of 90 seconds. Swarming servers have an internal 60-second
//     deadline to respond to requests.
func DefaultClientConfig() BuildbucketClientConfig {
	return BuildbucketClientConfig{
		Host:          buildbucket.DEFAULT_HOST,
		Retries:       DefaultRetries,
		Delay:         time.Second,
		MaxDelay:      time.Minute,
		PerRPCTimeout: DefaultPerRPCTimeout,
	}
}

// WithClient returns a BuildbucketClient as configured by the ClientConfig
func (bc BuildbucketClientConfig) WithClient(c *http.Client) *BuildbucketClient {
	return &BuildbucketClient{
		client: bpb.NewBuildsPRPCClient(
			&prpc.Client{
				C:    c,
				Host: bc.Host,
				Options: &prpc.Options{
					Retry: func() retry.Iterator {
						return &retry.ExponentialBackoff{
							MaxDelay: bc.MaxDelay,
							Limited: retry.Limited{
								Delay:   bc.Delay,
								Retries: bc.Retries,
							},
						}
					},
					PerRPCTimeout: 90 * time.Second,
				},
			},
		),
	}
}
