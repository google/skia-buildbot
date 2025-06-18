package buildbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"

	fmpb "google.golang.org/protobuf/types/known/fieldmaskpb"
	spb "google.golang.org/protobuf/types/known/structpb"
	tspb "google.golang.org/protobuf/types/known/timestamppb"

	// The main Buildbucket v2 client library.
	bbpb "go.chromium.org/luci/buildbucket/proto"
	// The PRPC client is used to communicate with LUCI services.
	"go.chromium.org/luci/grpc/prpc"
)

type BuildStatus struct {
	Status string
	URL    string
}

type ReleaseStatus map[string]BuildStatus

type BuildbucketClient struct {
	BuildsClient bbpb.BuildsClient
}

const (
	buildbucketDefaultHost = "cr-buildbucket.appspot.com"

	chromeVersionPattern = `\d+\.0\.\d+\.\d+`
	ReleaseTagPattern    = `refs/tags/(` + chromeVersionPattern + `)`
)

var builderBuckets = map[string]string{
	// Official release builders:
	"android-arm-official":        "official",
	"android-arm64-high-official": "official",
	"android-arm64-official":      "official",
	"android-x86-official":        "official",
	"android-x86_64-official":     "official",
	"linux64":                     "official",
	"mac-arm64":                   "official",
	"mac64":                       "official",
	"win-arm64-clang":             "official",
	"win-clang":                   "official",
	"win64-clang":                 "official",
	// Official infra builders:
	"chrome-release":                  "official.infra",
	"chrome-best-revision-continuous": "official.infra",
}

func genBuildID(builder string) (*bbpb.BuilderID, error) {
	bucket, ok := builderBuckets[builder]
	if !ok {
		return nil, skerr.Fmt("Unknown builder: %s", builder)
	}

	return &bbpb.BuilderID{
		Project: "chrome",
		Bucket:  bucket,
		Builder: builder,
	}, nil
}

func NewBuildbucketClient(httpClient *http.Client) BuildbucketClient {
	return BuildbucketClient{
		BuildsClient: bbpb.NewBuildsPRPCClient(&prpc.Client{
			C:       httpClient,
			Host:    buildbucketDefaultHost,
			Options: &prpc.Options{Insecure: false}, // Always use TLS/SSL
		}),
	}
}

func (bbc BuildbucketClient) GetBestRevisionHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	builderID, err := genBuildID("chrome-best-revision-continuous")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	req := &bbpb.SearchBuildsRequest{
		Predicate: &bbpb.BuildPredicate{
			Builder: builderID,
		},
		PageSize: 8, // 8 builds + 4-hour lookback window in each build, gives 12 hours of coverage.
		// Specify which fields you want back for each build.
		Mask: &bbpb.BuildMask{
			Fields: &fmpb.FieldMask{
				Paths: []string{"id", "builder", "number", "status", "summary_markdown", "create_time", "input", "output"},
			},
		},
	}

	builds, err := bbc.SearchBuilds(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get best revision builds: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found best revision builds: %v", builds)), nil
}

func (bbc BuildbucketClient) GetMilestoneStatusHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	milestone, err := request.RequireString(argMilestone)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	integerRegex := regexp.MustCompile(`\d+`)
	if !integerRegex.MatchString(milestone) {
		return mcp.NewToolResultError("Milestone number has to be an integer"), nil
	}

	builderID, _ := genBuildID("chrome-release")
	now := time.Now()
	req := &bbpb.SearchBuildsRequest{
		Predicate: &bbpb.BuildPredicate{
			Builder: builderID,
			CreateTime: &bbpb.TimeRange{
				StartTime: &tspb.Timestamp{
					// Search for builds in the past 30 days.
					Seconds: now.Add(-30 * 24 * time.Hour).Unix(),
				},
			},
		},
		PageSize: 100,
		// Specify which fields you want back for each build.
		Mask: &bbpb.BuildMask{
			Fields: &fmpb.FieldMask{
				Paths: []string{"id", "builder", "number", "status", "summary_markdown", "create_time", "input", "output"},
			},
		},
	}

	builds, err := bbc.SearchBuilds(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get chrome-release builds: %v", err)), nil
	}
	if len(builds) == 0 {
		return mcp.NewToolResultError("No builds found for the chrome-release builder."), nil
	}

	var version string
	var outputProperties *spb.Struct
	tagRegex := regexp.MustCompile(ReleaseTagPattern)
	for _, build := range builds {
		if build.Status != bbpb.Status_SUCCESS {
			continue
		}
		vMatches := tagRegex.FindStringSubmatch(build.Output.GitilesCommit.Ref)
		if len(vMatches) > 1 {
			// If len(matches) is > 1, it means the regex matched, and
			// the first capture group (matches[1]) contains the desired chunk.
			triggeredMilestone := integerRegex.FindString(vMatches[1])
			sklog.Debugf("Found chrome-release build for M%s", triggeredMilestone)
			if triggeredMilestone == milestone {
				outputProperties = build.Output.Properties
				version = vMatches[1]
				break
			}
		}
	}
	opMap := outputProperties.AsMap()
	triggeredBuilds, ok := opMap["triggered_build_ids"]
	if !ok {
		return mcp.NewToolResultText("The chrome-release build did not trigger any builds"), nil
	}
	releaseStatus := make(ReleaseStatus)
	for _, relBuild := range triggeredBuilds.([]interface{}) {
		for builder, bbID := range relBuild.(map[string]interface{}) {
			builderID, _ := genBuildID(builder)
			if builderID != nil {
				triggeredBuild, err := bbc.GetBuildByBBID(ctx, bbID.(string), []string{})
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				releaseStatus[builder] = BuildStatus{
					Status: triggeredBuild.Status.String(),
					URL:    fmt.Sprintf("https://cr-buildbucket.appspot.com/build/%s", bbID),
				}
			}
			sklog.Debugf("Triggered build: %s - go/bbid/%s", builder, bbID)
		}
	}

	results, err := json.Marshal(releaseStatus)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(`The latest version on M%s is %s.
List of triggered builds by builder name:\n%s`, milestone, version, string(results))), nil
}

func (bbc BuildbucketClient) GetVersionStatusHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	version, err := request.RequireString(argVersion)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	versionRegex := regexp.MustCompile(chromeVersionPattern)
	if !versionRegex.MatchString(version) {
		return mcp.NewToolResultError("Wrong format for the Chrome version."), nil
	}

	now := time.Now()
	req := &bbpb.SearchBuildsRequest{
		Predicate: &bbpb.BuildPredicate{
			CreateTime: &bbpb.TimeRange{
				StartTime: &tspb.Timestamp{
					// Search for builds in the past 30 days.
					Seconds: now.Add(-30 * 24 * time.Hour).Unix(),
				},
			},
		},
		PageSize: 1000,
		// Specify which fields you want back for each build.
		Mask: &bbpb.BuildMask{
			Fields: &fmpb.FieldMask{
				Paths: []string{"id", "builder", "number", "status", "summary_markdown", "create_time", "input"},
			},
		},
	}

	// Iterate through all release builders and find the latest build for
	// the specified Chrome version.
	tagRegex := regexp.MustCompile(ReleaseTagPattern)
	releaseStatus := make(ReleaseStatus)
	for builder, bucket := range builderBuckets {
		if bucket != "official" {
			continue
		}

		builderID, _ := genBuildID(builder)
		req.Predicate.Builder = builderID
		builds, err := bbc.SearchBuilds(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to builds for %s: %v", builder, err)), nil
		}
		if len(builds) == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("No builds found for %s", builder)), nil
		}

		for _, build := range builds {
			vMatches := tagRegex.FindStringSubmatch(build.Input.GitilesCommit.Ref)
			if len(vMatches) > 1 {
				// If len(matches) is > 1, it means the regex matched, and
				// the first capture group (matches[1]) contains the desired chunk.
				if vMatches[1] == version {
					// If the built version is the same as we are looking for, record the status.
					releaseStatus[builder] = BuildStatus{
						Status: build.Status.String(),
						URL:    fmt.Sprintf("https://cr-buildbucket.appspot.com/build/%d", build.Id),
					}
					break
				}
			}
		}
	}

	results, err := json.Marshal(releaseStatus)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(`List of builds for version %s by builder name:\n%s`, version, string(results))), nil
}

func (bbc BuildbucketClient) GetBuildStepsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	bbID, err := request.RequireString(argBBID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	integerRegex := regexp.MustCompile(`\d+`)
	if !integerRegex.MatchString(bbID) {
		return mcp.NewToolResultError("Buildbucket ID has to be an integer"), nil
	}
	build, err := bbc.GetBuildByBBID(ctx, bbID, []string{"steps"})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	steps, _ := json.Marshal(build.Steps)
	return mcp.NewToolResultText(fmt.Sprintf(`The steps in build %s are:\n%s`, bbID, steps)), nil
}

func (bbc BuildbucketClient) GetBuildByBBID(ctx context.Context, bbID string, fields []string) (*bbpb.Build, error) {
	id, err := strconv.ParseInt(bbID, 10, 64)
	if err != nil {
		return nil, skerr.Fmt("bbID can not be converted to int64: %s", bbID)
	}
	defaultFields := []string{"id", "builder", "number", "status", "summary_markdown"}
	req := &bbpb.GetBuildRequest{
		Id: id,
		// Specify which fields you want back for each build.
		Mask: &bbpb.BuildMask{
			Fields: &fmpb.FieldMask{
				Paths: append(defaultFields, fields...),
			},
		},
	}
	sklog.Info("Sending GetBuild request...")
	build, err := bbc.BuildsClient.GetBuild(ctx, req)
	if err != nil {
		return nil, err
	}
	return build, nil
}

func (bbc BuildbucketClient) SearchBuilds(ctx context.Context, r *bbpb.SearchBuildsRequest) ([]*bbpb.Build, error) {
	sklog.Info("Sending SearchBuilds request...")
	resp, err := bbc.BuildsClient.SearchBuilds(ctx, r)
	if err != nil {
		return nil, skerr.Wrapf(err, "SearchBuilds request failed")
	}
	return resp.Builds, nil
}

func (bbc BuildbucketClient) GetBuildsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	builder, err := request.RequireString(argBuilder)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	rangeStartSecs := request.GetString(argStartTime, "")
	rangeEndSecs := request.GetString(argEndTime, "")
	sklog.Debugf("Arg %s is: %s", argBuilder, builder)
	sklog.Debugf("Arg %s is: %s", argStartTime, rangeStartSecs)
	sklog.Debugf("Arg %s is: %s", argEndTime, rangeEndSecs)

	builderID, err := genBuildID(builder)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// --- Define the Search Predicate ---
	// The predicate defines what you are searching for.
	buildPredicate := &bbpb.BuildPredicate{
		Builder: builderID,
	}
	// TODO(keybo): implement page fetch with smaller page size. 1000 is the upper limit.
	pageSize := 10

	// Set the optional filter on Create_Time.
	if rangeStartSecs != "" || rangeEndSecs != "" {
		buildPredicate.CreateTime = &bbpb.TimeRange{}
		if rangeStartSecs != "" {
			start, err := strconv.ParseInt(rangeStartSecs, 10, 64)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("%s not an INT64: %s", argStartTime, rangeStartSecs)), nil
			}
			buildPredicate.CreateTime.StartTime = &tspb.Timestamp{
				Seconds: start,
			}
		}
		if rangeEndSecs != "" {
			end, err := strconv.ParseInt(rangeEndSecs, 10, 64)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("%s not an INT64: %s", argEndTime, rangeEndSecs)), nil
			}
			buildPredicate.CreateTime.EndTime = &tspb.Timestamp{
				Seconds: end,
			}
		}
		pageSize = 1000
	}

	// --- Create and Send the SearchBuilds Request ---
	req := &bbpb.SearchBuildsRequest{
		Predicate: buildPredicate,
		PageSize:  int32(pageSize),
		// Specify which fields you want back for each build.
		Mask: &bbpb.BuildMask{
			Fields: &fmpb.FieldMask{
				Paths: []string{"id", "builder", "number", "status", "summary_markdown", "create_time", "input", "output"},
			},
		},
	}

	builds, err := bbc.SearchBuilds(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get best revision builds: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found builds: %v", builds)), nil
}
