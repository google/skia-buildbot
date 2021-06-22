// Package buildbucket provides tools for interacting with the buildbucket API.
package buildbucket

import (
	"context"
	"net/http"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	structpb "google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/luci/grpc/prpc"
	"go.skia.org/infra/go/buildbucket/common"
	"go.skia.org/infra/go/skerr"
)

const (
	BUILD_URL_TMPL = "https://%s/build/%d"
	DEFAULT_HOST   = "cr-buildbucket.appspot.com"
)

var (
	DEFAULT_SCOPES = []string{
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	}
)

type BuildBucketInterface interface {
	// GetBuild retrieves the build with the given ID.
	GetBuild(ctx context.Context, buildId int64) (*buildbucketpb.Build, error)
	// Search retrieves Builds which match the given criteria.
	Search(ctx context.Context, pred *buildbucketpb.BuildPredicate) ([]*buildbucketpb.Build, error)
	// GetTrybotsForCL retrieves trybot results for the given CL.
	GetTrybotsForCL(ctx context.Context, issue, patchset int64, gerritUrl string) ([]*buildbucketpb.Build, error)
	// CancelBuilds cancels the specified buildIDs with the specified summaryMarkdown.
	CancelBuilds(ctx context.Context, buildIDs []int64, summaryMarkdown string) ([]*buildbucketpb.Build, error)
	// ScheduleBuilds schedules the specified builds on the given CL.
	ScheduleBuilds(ctx context.Context, builds []string, buildsToTags map[string]map[string]string, issue, patchset int64, gerritUrl, repo, bbProject, bbBucket string) ([]*buildbucketpb.Build, error)
	// CancelBuilds(ctx context.Context, builds)
}

// Client is used for interacting with the BuildBucket API.
type Client struct {
	bc   buildbucketpb.BuildsClient
	host string
}

// NewClient returns an authenticated Client instance.
func NewClient(c *http.Client) *Client {
	host := DEFAULT_HOST
	return &Client{
		bc: buildbucketpb.NewBuildsPRPCClient(&prpc.Client{
			C:    c,
			Host: host,
		}),
		host: host,
	}
}

// NewTestingClient lets the MockClient inject a mock BuildsClient and host.
func NewTestingClient(bc buildbucketpb.BuildsClient, host string) *Client {
	return &Client{
		bc:   bc,
		host: host,
	}
}

// CancelBuilds implements the BuildBucketInterface.
func (c *Client) CancelBuilds(ctx context.Context, buildIDs []int64, summaryMarkdown string) ([]*buildbucketpb.Build, error) {
	requests := []*buildbucketpb.BatchRequest_Request{}
	for _, bID := range buildIDs {
		request := &buildbucketpb.BatchRequest_Request{
			Request: &buildbucketpb.BatchRequest_Request_CancelBuild{
				CancelBuild: &buildbucketpb.CancelBuildRequest{
					Id:              bID,
					SummaryMarkdown: summaryMarkdown,
				},
			},
		}
		requests = append(requests, request)
	}

	resp, err := c.bc.Batch(ctx, &buildbucketpb.BatchRequest{
		Requests: requests,
	})
	if err != nil {
		return nil, skerr.Fmt("Could not cancel builds on buildbucket: %s", err)
	}
	if len(resp.Responses) != len(buildIDs) {
		return nil, skerr.Fmt("Buildbucket gave %d responses for %d builders", len(resp.Responses), len(buildIDs))
	}

	respBuilds := []*buildbucketpb.Build{}
	for _, r := range resp.Responses {
		respBuilds = append(respBuilds, r.GetCancelBuild())
	}
	return respBuilds, nil
}

// ScheduleBuilds implements the BuildBucketInterface.
// change should be cl num and patchset num instead to abstract out those details.
// Needs tests.
func (c *Client) ScheduleBuilds(ctx context.Context, builds []string, buildsToTags map[string]map[string]string, issue, patchset int64, gerritURL, repo, bbProject, bbBucket string) ([]*buildbucketpb.Build, error) {
	requests := []*buildbucketpb.BatchRequest_Request{}
	for _, b := range builds {
		tagStringPairs := []*buildbucketpb.StringPair{}
		tags, ok := buildsToTags[b]
		if ok {
			for n, v := range tags {
				stringPair := &buildbucketpb.StringPair{
					Key:   n,
					Value: v,
				}
				tagStringPairs = append(tagStringPairs, stringPair)
			}
		}
		request := &buildbucketpb.BatchRequest_Request{
			Request: &buildbucketpb.BatchRequest_Request_ScheduleBuild{
				ScheduleBuild: &buildbucketpb.ScheduleBuildRequest{
					Builder: &buildbucketpb.BuilderID{
						Project: bbProject,
						Bucket:  bbBucket,
						Builder: b,
					},
					GerritChanges: []*buildbucketpb.GerritChange{
						{
							Host:     gerritURL,
							Project:  repo,
							Change:   issue,
							Patchset: patchset,
						},
					},
					Properties: &structpb.Struct{},
					Tags:       tagStringPairs,
					Fields:     common.GetBuildFields,
				},
			},
		}
		requests = append(requests, request)
	}

	resp, err := c.bc.Batch(ctx, &buildbucketpb.BatchRequest{
		Requests: requests,
	})
	if err != nil {
		return nil, skerr.Fmt("Could not schedule builds on buildbucket: %s", err)
	}
	if len(resp.Responses) != len(builds) {
		return nil, skerr.Fmt("Buildbucket gave %d responses for %d builders", len(resp.Responses), len(builds))
	}

	respBuilds := []*buildbucketpb.Build{}
	for _, r := range resp.Responses {
		respBuilds = append(respBuilds, r.GetScheduleBuild())
	}
	return respBuilds, nil

	// Actual request looking at console in https://skia-review.googlesource.com/c/buildbot/+/414458 :
	/**
	 {"builds":[
		  {
				"id":"8845613382497379376",  // FIELDS
				"builder":{"project":"skia","bucket":"skia.primary","builder":"Infra-PerCommit-ValidateAutorollConfigs"},  // FIELDS
				"createdBy":"project:skiabuildbot",  // FIELDS
				"createTime":"2021-06-01T15:52:06.460234Z",  // FIELDS
				"startTime":"2021-06-01T15:52:20.654184Z",
				"endTime":"2021-06-01T15:58:34.314161Z",
				"updateTime":"2021-06-01T15:58:34.314603Z",
				"status":"SUCCESS",  // FIELDS
				"input":
					{
						// DONT CARE? UNLESS SkCQ needs to set it's own properties.
						"properties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"runMode":"DRY_RUN","topLevel":true}},
						// CARE
						"gerritChanges":[{"host":"skia-review.googlesource.com","project":"buildbot","change":"414458","patchset":"1"}]
					},
					"output":{"properties":{}},
					"infra":  // ETC ETC ETC
						{
							"buildbucket":
								{
									"requestedProperties":
										{
											"$recipe_engine/cq":
												{
													"active":true,"dryRun":true,"runMode":"DRY_RUN","topLevel":true
												}
										}
									},
							"swarming":{"caches":[{"name":"git","path":"git","waitForWarmCache":"0s"},{"name":"vpython","path":"vpython","waitForWarmCache":"0s","envVar":"VPYTHON_VIRTUALENV_ROOT"},{"name":"goma_v2","path":"goma","waitForWarmCache":"0s"},{"name":"builder_3ba33660561724c80fd00634c8f34f7885d4e073db709c83351769e53f138c46_v2","path":"builder","waitForWarmCache":"240s"}]},"logdog":{"hostname":"logs.chromium.org","project":"skia","prefix":"buildbucket/cr-buildbucket.appspot.com/8845613382497379376"},"resultdb":{"hostname":"results.api.cr.dev"}}

							"tags":
								[
									{"key":"buildset","value":"patch/gerrit/skia-review.googlesource.com/414458/1"},
									{"key":"cq_attempt_key","value":"ba48e0d920013fba"},
									{"key":"cq_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},
									{"key":"cq_cl_owner","value":"borenet@google.com"},
									{"key":"cq_equivalent_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},
									{"key":"cq_experimental","value":"false"},
									{"key":"cq_triggerer","value":"borenet@google.com"},
									{"key":"user_agent","value":"cq"}],"exe":{}},
	**/
	// {"id":"8845613382497379392","builder":{"project":"skia","bucket":"skia.primary","builder":"Infra-PerCommit-Test-Bazel-RBE"},"createdBy":"project:skiabuildbot","createTime":"2021-06-01T15:52:06.460234Z","startTime":"2021-06-01T15:52:20.770674Z","endTime":"2021-06-01T16:06:59.551624Z","updateTime":"2021-06-01T16:06:59.552903Z","status":"SUCCESS","input":{"properties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"experimental":true,"runMode":"DRY_RUN","topLevel":true}},"gerritChanges":[{"host":"skia-review.googlesource.com","project":"buildbot","change":"414458","patchset":"1"}]},"output":{"properties":{}},"infra":{"buildbucket":{"requestedProperties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"experimental":true,"runMode":"DRY_RUN","topLevel":true}}},"swarming":{"caches":[{"name":"git","path":"git","waitForWarmCache":"0s"},{"name":"vpython","path":"vpython","waitForWarmCache":"0s","envVar":"VPYTHON_VIRTUALENV_ROOT"},{"name":"goma_v2","path":"goma","waitForWarmCache":"0s"},{"name":"builder_9f0e128774d739fefdf93b12fc13ecbb4c9dc5175f250ee6d2f5d9c2296a2e7f_v2","path":"builder","waitForWarmCache":"240s"}]},"logdog":{"hostname":"logs.chromium.org","project":"skia","prefix":"buildbucket/cr-buildbucket.appspot.com/8845613382497379392"},"resultdb":{"hostname":"results.api.cr.dev"}},"tags":[{"key":"buildset","value":"patch/gerrit/skia-review.googlesource.com/414458/1"},{"key":"cq_attempt_key","value":"ba48e0d920013fba"},{"key":"cq_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},{"key":"cq_cl_owner","value":"borenet@google.com"},{"key":"cq_equivalent_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},{"key":"cq_experimental","value":"true"},{"key":"cq_triggerer","value":"borenet@google.com"},{"key":"user_agent","value":"cq"}],"exe":{}},{"id":"8845613382497379408","builder":{"project":"skia","bucket":"skia.primary","builder":"Infra-PerCommit-Small"},"createdBy":"project:skiabuildbot","createTime":"2021-06-01T15:52:06.460234Z","startTime":"2021-06-01T15:52:22.878452Z","endTime":"2021-06-01T15:58:33.461670Z","updateTime":"2021-06-01T15:58:33.462502Z","status":"SUCCESS","input":{"properties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"runMode":"DRY_RUN","topLevel":true}},"gerritChanges":[{"host":"skia-review.googlesource.com","project":"buildbot","change":"414458","patchset":"1"}]},"output":{"properties":{}},"infra":{"buildbucket":{"requestedProperties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"runMode":"DRY_RUN","topLevel":true}}},"swarming":{"caches":[{"name":"git","path":"git","waitForWarmCache":"0s"},{"name":"vpython","path":"vpython","waitForWarmCache":"0s","envVar":"VPYTHON_VIRTUALENV_ROOT"},{"name":"goma_v2","path":"goma","waitForWarmCache":"0s"},{"name":"builder_cd3811a7bf039944a6d1d1d1d6906852d4ca2d6d9b8ce66d3edd5d6568f03185_v2","path":"builder","waitForWarmCache":"240s"}]},"logdog":{"hostname":"logs.chromium.org","project":"skia","prefix":"buildbucket/cr-buildbucket.appspot.com/8845613382497379408"},"resultdb":{"hostname":"results.api.cr.dev"}},"tags":[{"key":"buildset","value":"patch/gerrit/skia-review.googlesource.com/414458/1"},{"key":"cq_attempt_key","value":"ba48e0d920013fba"},{"key":"cq_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},{"key":"cq_cl_owner","value":"borenet@google.com"},{"key":"cq_equivalent_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},{"key":"cq_experimental","value":"false"},{"key":"cq_triggerer","value":"borenet@google.com"},{"key":"user_agent","value":"cq"}],"exe":{}},{"id":"8845613382497379424","builder":{"project":"skia","bucket":"skia.primary","builder":"Infra-PerCommit-Puppeteer"},"createdBy":"project:skiabuildbot","createTime":"2021-06-01T15:52:06.460234Z","startTime":"2021-06-01T15:52:20.645292Z","endTime":"2021-06-01T16:08:30.107945Z","updateTime":"2021-06-01T16:08:30.108428Z","status":"SUCCESS","input":{"properties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"runMode":"DRY_RUN","topLevel":true}},"gerritChanges":[{"host":"skia-review.googlesource.com","project":"buildbot","change":"414458","patchset":"1"}]},"output":{"properties":{}},"infra":{"buildbucket":{"requestedProperties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"runMode":"DRY_RUN","topLevel":true}}},"swarming":{"caches":[{"name":"git","path":"git","waitForWarmCache":"0s"},{"name":"vpython","path":"vpython","waitForWarmCache":"0s","envVar":"VPYTHON_VIRTUALENV_ROOT"},{"name":"goma_v2","path":"goma","waitForWarmCache":"0s"},{"name":"builder_d3e17358a9566eb0e87c2db580710ad155a0efd0fa07e4bdc5edca58401e59c6_v2","path":"builder","waitForWarmCache":"240s"}]},"logdog":{"hostname":"logs.chromium.org","project":"skia","prefix":"buildbucket/cr-buildbucket.appspot.com/8845613382497379424"},"resultdb":{"hostname":"results.api.cr.dev"}},"tags":[{"key":"buildset","value":"patch/gerrit/skia-review.googlesource.com/414458/1"},{"key":"cq_attempt_key","value":"ba48e0d920013fba"},{"key":"cq_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},{"key":"cq_cl_owner","value":"borenet@google.com"},{"key":"cq_equivalent_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},{"key":"cq_experimental","value":"false"},{"key":"cq_triggerer","value":"borenet@google.com"},{"key":"user_agent","value":"cq"}],"exe":{}},{"id":"8845613382497379440","builder":{"project":"skia","bucket":"skia.primary","builder":"Infra-PerCommit-Medium"},"createdBy":"project:skiabuildbot","createTime":"2021-06-01T15:52:06.460234Z","startTime":"2021-06-01T15:52:20.690101Z","endTime":"2021-06-01T16:02:00.396490Z","updateTime":"2021-06-01T16:02:00.396941Z","status":"SUCCESS","input":{"properties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"runMode":"DRY_RUN","topLevel":true}},"gerritChanges":[{"host":"skia-review.googlesource.com","project":"buildbot","change":"414458","patchset":"1"}]},"output":{"properties":{}},"infra":{"buildbucket":{"requestedProperties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"runMode":"DRY_RUN","topLevel":true}}},"swarming":{"caches":[{"name":"git","path":"git","waitForWarmCache":"0s"},{"name":"vpython","path":"vpython","waitForWarmCache":"0s","envVar":"VPYTHON_VIRTUALENV_ROOT"},{"name":"goma_v2","path":"goma","waitForWarmCache":"0s"},{"name":"builder_2581f4dad84cb857d51f66a057faae0d659cd8bc22960243cc1a4fb9e8022c5c_v2","path":"builder","waitForWarmCache":"240s"}]},"logdog":{"hostname":"logs.chromium.org","project":"skia","prefix":"buildbucket/cr-buildbucket.appspot.com/8845613382497379440"},"resultdb":{"hostname":"results.api.cr.dev"}},"tags":[{"key":"buildset","value":"patch/gerrit/skia-review.googlesource.com/414458/1"},{"key":"cq_attempt_key","value":"ba48e0d920013fba"},{"key":"cq_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},{"key":"cq_cl_owner","value":"borenet@google.com"},{"key":"cq_equivalent_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},{"key":"cq_experimental","value":"false"},{"key":"cq_triggerer","value":"borenet@google.com"},{"key":"user_agent","value":"cq"}],"exe":{}},{"id":"8845613382497379456","builder":{"project":"skia","bucket":"skia.primary","builder":"Infra-PerCommit-Large"},"createdBy":"project:skiabuildbot","createTime":"2021-06-01T15:52:06.460234Z","startTime":"2021-06-01T15:52:20.761911Z","endTime":"2021-06-01T16:08:29.655239Z","updateTime":"2021-06-01T16:08:29.655616Z","status":"SUCCESS","input":{"properties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"runMode":"DRY_RUN","topLevel":true}},"gerritChanges":[{"host":"skia-review.googlesource.com","project":"buildbot","change":"414458","patchset":"1"}]},"output":{"properties":{}},"infra":{"buildbucket":{"requestedProperties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"runMode":"DRY_RUN","topLevel":true}}},"swarming":{"caches":[{"name":"git","path":"git","waitForWarmCache":"0s"},{"name":"vpython","path":"vpython","waitForWarmCache":"0s","envVar":"VPYTHON_VIRTUALENV_ROOT"},{"name":"goma_v2","path":"goma","waitForWarmCache":"0s"},{"name":"builder_04101e97f437388723fc7d9004ebcdec707aed58e202640c90049996c2cd4c20_v2","path":"builder","waitForWarmCache":"240s"}]},"logdog":{"hostname":"logs.chromium.org","project":"skia","prefix":"buildbucket/cr-buildbucket.appspot.com/8845613382497379456"},"resultdb":{"hostname":"results.api.cr.dev"}},"tags":[{"key":"buildset","value":"patch/gerrit/skia-review.googlesource.com/414458/1"},{"key":"cq_attempt_key","value":"ba48e0d920013fba"},{"key":"cq_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},{"key":"cq_cl_owner","value":"borenet@google.com"},{"key":"cq_equivalent_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},{"key":"cq_experimental","value":"false"},{"key":"cq_triggerer","value":"borenet@google.com"},{"key":"user_agent","value":"cq"}],"exe":{}},{"id":"8845613382497379472","builder":{"project":"skia","bucket":"skia.primary","builder":"Infra-PerCommit-Build-Bazel-RBE"},"createdBy":"project:skiabuildbot","createTime":"2021-06-01T15:52:06.460234Z","startTime":"2021-06-01T15:52:20.608247Z","endTime":"2021-06-01T16:08:01.467023Z","updateTime":"2021-06-01T16:08:01.467563Z","status":"SUCCESS","input":{"properties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"experimental":true,"runMode":"DRY_RUN","topLevel":true}},"gerritChanges":[{"host":"skia-review.googlesource.com","project":"buildbot","change":"414458","patchset":"1"}]},"output":{"properties":{}},"infra":{"buildbucket":{"requestedProperties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"experimental":true,"runMode":"DRY_RUN","topLevel":true}}},"swarming":{"caches":[{"name":"git","path":"git","waitForWarmCache":"0s"},{"name":"vpython","path":"vpython","waitForWarmCache":"0s","envVar":"VPYTHON_VIRTUALENV_ROOT"},{"name":"goma_v2","path":"goma","waitForWarmCache":"0s"},{"name":"builder_f06f3b869af39016191e9e6c7eab3053f4a9a7397d2c1d59d00f95d62842cf3e_v2","path":"builder","waitForWarmCache":"240s"}]},"logdog":{"hostname":"logs.chromium.org","project":"skia","prefix":"buildbucket/cr-buildbucket.appspot.com/8845613382497379472"},"resultdb":{"hostname":"results.api.cr.dev"}},"tags":[{"key":"buildset","value":"patch/gerrit/skia-review.googlesource.com/414458/1"},{"key":"cq_attempt_key","value":"ba48e0d920013fba"},{"key":"cq_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},{"key":"cq_cl_owner","value":"borenet@google.com"},{"key":"cq_equivalent_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},{"key":"cq_experimental","value":"true"},{"key":"cq_triggerer","value":"borenet@google.com"},{"key":"user_agent","value":"cq"}],"exe":{}},{"id":"8845613382497379488","builder":{"project":"skia","bucket":"skia.primary","builder":"Housekeeper-OnDemand-Presubmit"},"createdBy":"project:skiabuildbot","createTime":"2021-06-01T15:52:06.460234Z","startTime":"2021-06-01T15:52:20.741133Z","endTime":"2021-06-01T15:54:29.747222Z","updateTime":"2021-06-01T15:54:29.747581Z","status":"SUCCESS","input":{"properties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"runMode":"DRY_RUN","topLevel":true}},"gerritChanges":[{"host":"skia-review.googlesource.com","project":"buildbot","change":"414458","patchset":"1"}]},"output":{"properties":{}},"infra":{"buildbucket":{"requestedProperties":{"$recipe_engine/cq":{"active":true,"dryRun":true,"runMode":"DRY_RUN","topLevel":true}}},"swarming":{"caches":[{"name":"git","path":"git","waitForWarmCache":"0s"},{"name":"vpython","path":"vpython","waitForWarmCache":"0s","envVar":"VPYTHON_VIRTUALENV_ROOT"},{"name":"goma_v2","path":"goma","waitForWarmCache":"0s"},{"name":"builder_9ad783fbb9338ac9fcbe93284e783c2830677c7a68d644228f4a87831cea160d_v2","path":"builder","waitForWarmCache":"240s"}]},"logdog":{"hostname":"logs.chromium.org","project":"skia","prefix":"buildbucket/cr-buildbucket.appspot.com/8845613382497379488"},"resultdb":{"hostname":"results.api.cr.dev"}},"tags":[{"key":"buildset","value":"patch/gerrit/skia-review.googlesource.com/414458/1"},{"key":"cq_attempt_key","value":"ba48e0d920013fba"},{"key":"cq_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},{"key":"cq_cl_owner","value":"borenet@google.com"},{"key":"cq_equivalent_cl_group_key","value":"16643b62dcb8aab284880075d486f8cb185dd6c45c8917b94bdf0001"},{"key":"cq_experimental","value":"false"},{"key":"cq_triggerer","value":"borenet@google.com"},{"key":"user_agent","value":"cq"}],"exe":{}}]}

	// c.bc.Batch

	// Batch(ctx context.Context, in *BatchRequest, opts ...grpc.CallOption) (*BatchResponse, error)

	/**
		    assert attempt.mode != attempt_mode.AttemptMode.DISAGREEMENT
	    assert isinstance(required_builds, list), type(required_builds)
	    fields = field_mask_pb2.FieldMask(
	        paths=[
	            'id',
	            # It's not strictly necessary to fetch "builder.*" from Buildbucket,
	            # since we know exactly which builder we asked to trigger and amend
	            # the response before returning but this microoptimization isn't
	            # worth the maintenance cost.
	            'builder.*',
	            'create_time',
	            'created_by',
	            'status',
	            'tags',
	        ]
	    )
	    gerrit_changes = _extract_gerrit_changes(attempt)
	    by_exp = collections.Counter(b.experimental for b in required_builds)
	    # Properties and tags depend on experimental status, so cache computation
	    # once for all experimental and non-experimental builds.
	    properties = {e: _make_properties(attempt, e) for e in by_exp}
	    tags = {e: _make_tags(attempt, e) for e in by_exp}

	    req = builds_service_pb2.BatchRequest(
	        requests=[
	            builds_service_pb2.BatchRequest.Request(
	                schedule_build=builds_service_pb2.ScheduleBuildRequest(
	                    # TODO(tandrii): set deterministic request_id to avoid
	                    # retries triggering new builds.
	                    builder=builder_pb2.BuilderID(
	                        project=b.v2_project,
	                        bucket=b.v2_bucket,
	                        builder=b.builder_name
	                    ),
	                    gerrit_changes=gerrit_changes,
	                    properties=properties[b.experimental],
	                    tags=tags[b.experimental],
	                    fields=fields,
	                )
	            ) for b in required_builds
	        ],
	    )
	    logging.debug(
	        'triggering %d%s builds for %s', len(required_builds),
	        (' (%d experimental)' % by_exp[True]) if by_exp[True] else '', attempt
	    )
	    with self._lease_v2('schedule-batch') as client:
	      status, resp = client.call(
	          'Batch', req, builds_service_pb2.BatchResponse()
	      )

	    if status == prpc.STATUS_OK:
	      for one_resp, rb in itertools.izip(resp.responses, required_builds):
	        if one_resp.HasField('schedule_build'):
	          if not rb.experimental:
	            self._cancelator.on_known_build(one_resp.schedule_build, attempt)

	    return status, resp
		**/
}

// GetBuild implements the BuildBucketInterface.
func (c *Client) GetBuild(ctx context.Context, buildId int64) (*buildbucketpb.Build, error) {
	b, err := c.bc.GetBuild(ctx, &buildbucketpb.GetBuildRequest{
		Id:     buildId,
		Fields: common.GetBuildFields,
	})
	return b, err
}

// GetBuild implements the BuildBucketInterface.
func (c *Client) Search(ctx context.Context, pred *buildbucketpb.BuildPredicate) ([]*buildbucketpb.Build, error) {
	rv := []*buildbucketpb.Build{}
	cursor := ""
	for {
		req := &buildbucketpb.SearchBuildsRequest{
			Fields:    common.SearchBuildsFields,
			PageToken: cursor,
			Predicate: pred,
		}
		resp, err := c.bc.SearchBuilds(ctx, req)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			break
		}
		rv = append(rv, resp.Builds...)
		cursor = resp.NextPageToken
		if cursor == "" {
			break
		}
	}
	return rv, nil
}

// GetTrybotsForCL implements the BuildBucketInterface.
func (c *Client) GetTrybotsForCL(ctx context.Context, issue, patchset int64, gerritUrl string) ([]*buildbucketpb.Build, error) {
	pred, err := common.GetTrybotsForCLPredicate(issue, patchset, gerritUrl, map[string]string{})
	if err != nil {
		return nil, err
	}
	return c.Search(ctx, pred)
}

// Make sure Client fulfills the BuildBucketInterface interface.
var _ BuildBucketInterface = (*Client)(nil)
