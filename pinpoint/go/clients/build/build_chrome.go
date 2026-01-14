package build

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"go.skia.org/infra/pinpoint/go/workflows"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

const (
	// ChromeProject refers to the "chrome" project.
	ChromeProject = "chrome"
	// ChromiumGitilesURL is the default Gitiles URL for chromium/src.
	ChromiumGitilesURL = "https://chromium.googlesource.com/chromium/src"
	// ChromiumGitilesHost is the default Gitiles host for chromium/src.
	ChromiumGitilesHost = "chromium.googlesource.com"
	// ChromiumGitilesProject is the default project name for chromium/src.
	ChromiumGitilesProject = "chromium/src"
	// ChromiumGitilesRefAtHead is the default ref used for Chromium builds.
	ChromiumGitilesRefAtHead = "refs/heads/main"

	// DefaultBucket is the Pinpoint bucket, equivalent to the "try" builds in Buildbucket.
	DefaultBucket = "try"
	// DefaultBuildsetKey is key tagged on builds for how commit information is tracked in Waterfall (CI) and Pinpoint.
	DefaultBuildsetKey = "buildset"
	// DefaultTagValue is the value format for the key above.
	DefaultBuildsetValue = "commit/gitiles/chromium.googlesource.com/chromium/src/+/%s"
	// DefaultCASInstance is the default CAS instance used by Pinpoint builds.
	//
	// TODO(b/315215756): Support other swarming instances. There are three known
	// swarming instances Pinpoint supports. The majority of Pinpoint builds are
	// this defaultInstance. Buildbucket API does not report the swarming instance
	// so our options are to:
	// - include the expected instance in the build tags
	// - try all 3 known swarming instances and brute force it
	DefaultCASInstance = "projects/chrome-swarming/instances/default_instance"
	// DepsOverrideKey is the key used to find any deps overrides in the input properties from a Buildbucket response.
	DepsOverrideKey = "deps_revision_overrides"
)

type buildChromeClient struct {
	backends.BuildbucketClient
}

func newBuildChromeClient(c *http.Client) *buildChromeClient {
	return &buildChromeClient{
		BuildbucketClient: backends.DefaultClientConfig().WithClient(c),
	}
}

type ChromeFindBuildRequest struct {
	Device  string
	Commit  string
	Deps    map[string]string
	Patches []*buildbucketpb.GerritChange
}

// CreateFindBuildRequest returns a request object with details needed by Chrome.
func (b *buildChromeClient) CreateFindBuildRequest(params workflows.BuildParams) (*FindBuildRequest, error) {
	if params.Device == "" || params.Commit == nil {
		return nil, skerr.Fmt("Missing required fields Commit and Device to create FindBuild request for Chrome")
	}
	chromeFindRequest := &ChromeFindBuildRequest{
		Device:  params.Device,
		Commit:  params.Commit.GetMainGitHash(),
		Deps:    params.Commit.DepsToMap(),
		Patches: params.Patch,
	}
	return &FindBuildRequest{
		Request: chromeFindRequest,
	}, nil
}

func (b *buildChromeClient) FindBuild(ctx context.Context, req *FindBuildRequest) (*FindBuildResponse, error) {
	findReq := req.Request.(*ChromeFindBuildRequest)
	builder, err := bot_configs.GetBotConfig(findReq.Device, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "Unsupported device value provided while searching for Chrome build")
	}

	build, err := b.BuildbucketClient.GetSingleBuild(ctx, builder.Builder, DefaultBucket, findReq.Commit, findReq.Deps, findReq.Patches)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to search for Chrome build")
	}

	if build != nil {
		return &FindBuildResponse{
			BuildID:  build.Id,
			Response: build,
		}, nil
	}

	// Do not check waterfall if there are gerrit patches or DEPS rolls.
	// Waterfall continuously builds commits along chromium/src and does not build
	// commits in other repos (i.e. DEPS) or user submitted gerrit patches.
	// If we checked the waterfall pool, we would find builds in chromium/src without
	// DEPS or the gerrit patch, finding the wrong build.
	if (findReq.Deps != nil && len(findReq.Deps) > 0) || (findReq.Patches != nil && len(findReq.Patches) > 0) {
		return &FindBuildResponse{
			Response: nil,
		}, nil
	}

	// For Chrome, search waterfall for build if there is an appropriate waterfall builder,
	// no gerrit patches, and no DEPS rolls.
	// Waterfall only builds chromium/src commits.
	// We search waterfall after Pinpoint, because waterfall builders
	// lag behind main. A user could try to request a build via Pinpoint before
	// waterfall has the chance to build the same commit.
	build, err = b.BuildbucketClient.GetBuildFromWaterfall(ctx, builder.Builder, findReq.Commit)
	if err != nil {
		return &FindBuildResponse{
			Response: nil,
		}, skerr.Wrapf(err, "Failed to find build with CI equivalent.")
	}
	// No matching waterfall build found.
	if build == nil {
		return &FindBuildResponse{
			Response: nil,
		}, nil
	}

	return &FindBuildResponse{
		BuildID:  build.Id,
		Response: build,
	}, nil
}

func (b *buildChromeClient) buildRequestProperties(commit string) *structpb.Struct {
	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"clobber": {
				Kind: &structpb.Value_BoolValue{
					BoolValue: false,
				},
			},
			"git_repo": {
				Kind: &structpb.Value_StringValue{
					StringValue: ChromiumGitilesURL,
				},
			},
			"revision": {
				Kind: &structpb.Value_StringValue{
					StringValue: commit,
				},
			},
			"staging": {
				Kind: &structpb.Value_BoolValue{
					BoolValue: false,
				},
			},
		},
	}
}

func (b *buildChromeClient) buildRequestGitilesCommit(commit string) *buildbucketpb.GitilesCommit {
	return &buildbucketpb.GitilesCommit{
		Host:    ChromiumGitilesHost,
		Project: ChromiumGitilesProject,
		Id:      commit,
		Ref:     ChromiumGitilesRefAtHead,
	}
}

func (b *buildChromeClient) buildRequestTags(pinpointJobID, commit string) []*buildbucketpb.StringPair {
	return []*buildbucketpb.StringPair{
		{
			Key:   "pinpoint_job_id",
			Value: pinpointJobID,
		},
		{
			Key:   "skia_pinpoint",
			Value: "true",
		},
		{
			Key:   DefaultBuildsetKey,
			Value: fmt.Sprintf(DefaultBuildsetValue, commit),
		},
	}
}

// Note: Patch parameter was removed because it was not in use, but will
// need to be re-added to support Try jobs with patch.
func (b *buildChromeClient) CreateStartBuildRequest(params workflows.BuildParams) (*StartBuildRequest, error) {
	if params.Device == "" || params.Commit == nil || params.WorkflowID == "" {
		return nil, skerr.Fmt("Missing required fields, one of [Commit, Device, WorkflowID] to create StartBuild request for Chrome")
	}
	commit := params.Commit.GetMainGitHash()
	deps := params.Commit.DepsToMap()
	builder, err := bot_configs.GetBotConfig(params.Device, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "Unsupported device value provided while creating Chrome build request")
	}

	properties := b.buildRequestProperties(commit)

	if deps != nil && len(deps) > 0 {
		fields := make(map[string]*structpb.Value, 0)
		for url, rev := range deps {
			fields[url] = &structpb.Value{
				Kind: &structpb.Value_StringValue{
					StringValue: rev,
				},
			}
		}
		properties.Fields[DepsOverrideKey] = &structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: &structpb.Struct{
					Fields: fields,
				},
			},
		}
	}

	// TODO(b/315215756): Implement createTags function to generalize across different job types
	scheduleReq := &buildbucketpb.ScheduleBuildRequest{
		RequestId: uuid.New().String(),
		Builder: &buildbucketpb.BuilderID{
			Project: ChromeProject,
			Bucket:  DefaultBucket,
			Builder: builder.Builder,
		},
		Properties:    properties,
		GitilesCommit: b.buildRequestGitilesCommit(commit),
		Tags:          b.buildRequestTags(params.WorkflowID, commit),
	}
	if len(params.Patch) > 0 {
		scheduleReq.GerritChanges = params.Patch
	}
	return &StartBuildRequest{
		Request: scheduleReq,
	}, nil
}

func (b *buildChromeClient) StartBuild(ctx context.Context, req *StartBuildRequest) (*StartBuildResponse, error) {
	build, err := b.BuildbucketClient.StartBuild(ctx, req.Request.(*buildbucketpb.ScheduleBuildRequest))
	return &StartBuildResponse{
		Response: build,
	}, err
}

func (b *buildChromeClient) GetStatus(ctx context.Context, id int64) (buildbucketpb.Status, error) {
	return b.BuildbucketClient.GetBuildStatus(ctx, id)
}

func (b *buildChromeClient) GetBuildArtifact(ctx context.Context, req *GetBuildArtifactRequest) (*GetBuildArtifactResponse, error) {
	cas, err := b.BuildbucketClient.GetCASReference(ctx, req.BuildID, req.Target)
	if err != nil {
		return nil, err
	}
	return &GetBuildArtifactResponse{
		Response: cas,
	}, nil
}

func (b *buildChromeClient) CancelBuild(ctx context.Context, req *CancelBuildRequest) error {
	return b.BuildbucketClient.CancelBuild(ctx, req.BuildID, req.Reason)
}

var _ BuildClient = (*buildChromeClient)(nil)
